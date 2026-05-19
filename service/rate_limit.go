package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/go-redis/redis/v8"
)

// Calc5hWindowKey returns the 5-hour window index (0-4) for a given UTC hour.
func Calc5hWindowKey(utcHour int) int {
	return utcHour / 5
}

// Calc5hWindowResetAt returns the Unix timestamp when the current 5h window resets.
func Calc5hWindowResetAt(now time.Time) int64 {
	windowIdx := Calc5hWindowKey(now.Hour())
	resetHour := (windowIdx + 1) * 5
	if resetHour >= 24 {
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC).Unix()
	}
	return time.Date(now.Year(), now.Month(), now.Day(), resetHour, 0, 0, 0, time.UTC).Unix()
}

// CalcWeekWindowKey returns the ISO week key (e.g. "2026-W20") for a given time.
func CalcWeekWindowKey(now time.Time) string {
	year, week := now.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// CalcWeekResetAt returns the Unix timestamp when the current week resets (next Monday 00:00 UTC).
func CalcWeekResetAt(now time.Time) int64 {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	nextMonday := now.AddDate(0, 0, 8-weekday)
	return time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 0, 0, 0, 0, time.UTC).Unix()
}

// RateLimitRedisKey5h builds the Redis key for a 5h window.
func RateLimitRedisKey5h(subscriptionId int, windowKey int) string {
	return fmt.Sprintf("rate_limit:%d:5h:%d", subscriptionId, windowKey)
}

// RateLimitRedisKeyWeek builds the Redis key for a week window.
func RateLimitRedisKeyWeek(subscriptionId int, weekKey string) string {
	return fmt.Sprintf("rate_limit:%d:week:%s", subscriptionId, weekKey)
}

// WindowStatus represents the rate limit status for a single window.
type WindowStatus struct {
	Limit     int   `json:"limit"`
	Used      int   `json:"used"`
	Remaining int   `json:"remaining"`
	ResetAt   int64 `json:"reset_at"`
}

// RateLimitStatus represents the full rate limit status for a subscription.
type RateLimitStatus struct {
	SubscriptionId int          `json:"subscription_id"`
	PlanTitle      string       `json:"plan_title"`
	PlanType       string       `json:"plan_type"`
	Window5h       WindowStatus `json:"window_5h"`
	WindowWeek     WindowStatus `json:"window_week"`
}

// CheckSubscriptionRateLimit checks if the subscription has exceeded its rate limits.
// Returns the current status and whether the rate limit is exceeded.
func CheckSubscriptionRateLimit(subscriptionId int, limit5h int, weeklyMultiplier int) (*RateLimitStatus, bool) {
	now := time.Now().UTC()
	status := &RateLimitStatus{
		SubscriptionId: subscriptionId,
	}

	if limit5h <= 0 || weeklyMultiplier <= 0 {
		return status, false
	}

	limitWeek := limit5h * weeklyMultiplier

	// Query 5h window
	windowKey5h := Calc5hWindowKey(now.Hour())
	redisKey5h := RateLimitRedisKey5h(subscriptionId, windowKey5h)
	used5h := queryWindowUsage(redisKey5h)
	remaining5h := limit5h - used5h
	if remaining5h < 0 {
		remaining5h = 0
	}
	status.Window5h = WindowStatus{
		Limit:     limit5h,
		Used:      used5h,
		Remaining: remaining5h,
		ResetAt:   Calc5hWindowResetAt(now),
	}

	// Query week window
	weekKey := CalcWeekWindowKey(now)
	redisKeyWeek := RateLimitRedisKeyWeek(subscriptionId, weekKey)
	usedWeek := queryWindowUsage(redisKeyWeek)
	remainingWeek := limitWeek - usedWeek
	if remainingWeek < 0 {
		remainingWeek = 0
	}
	status.WindowWeek = WindowStatus{
		Limit:     limitWeek,
		Used:      usedWeek,
		Remaining: remainingWeek,
		ResetAt:   CalcWeekResetAt(now),
	}

	exceeded := used5h >= limit5h || usedWeek >= limitWeek
	return status, exceeded
}

// queryWindowUsage queries the total token usage stored in a Redis sorted set.
// Members are stored as "requestId:tokenCount" with score as timestamp.
func queryWindowUsage(redisKey string) int {
	if common.RDB == nil {
		return 0
	}
	ctx := context.Background()
	members, err := common.RDB.ZRangeByScore(ctx, redisKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: "+inf",
	}).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			logger.LogError(nil, fmt.Sprintf("rate limit query failed for key %s: %v", redisKey, err))
		}
		return 0
	}
	total := 0
	for _, m := range members {
		parts := strings.SplitN(m, ":", 2)
		if len(parts) == 2 {
			if count, err := strconv.Atoi(parts[1]); err == nil {
				total += count
			}
		}
	}
	return total
}

// RecordRateLimitUsage records token usage for a subscription in both 5h and week windows.
func RecordRateLimitUsage(subscriptionId int, requestId string, tokenCount int) error {
	if common.RDB == nil {
		return nil
	}
	if subscriptionId <= 0 || tokenCount <= 0 {
		return nil
	}

	now := time.Now().UTC()
	timestamp := now.Unix()
	member := fmt.Sprintf("%s:%d", requestId, tokenCount)

	ctx := context.Background()
	pipe := common.RDB.Pipeline()

	// Record in 5h window
	windowKey5h := Calc5hWindowKey(now.Hour())
	redisKey5h := RateLimitRedisKey5h(subscriptionId, windowKey5h)
	pipe.ZAdd(ctx, redisKey5h, &redis.Z{Score: float64(timestamp), Member: member})
	pipe.Expire(ctx, redisKey5h, 6*time.Hour)

	// Record in week window
	weekKey := CalcWeekWindowKey(now)
	redisKeyWeek := RateLimitRedisKeyWeek(subscriptionId, weekKey)
	pipe.ZAdd(ctx, redisKeyWeek, &redis.Z{Score: float64(timestamp), Member: member})
	pipe.Expire(ctx, redisKeyWeek, 8*24*time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.LogError(nil, fmt.Sprintf("failed to record rate limit usage for subscription %d: %v", subscriptionId, err))
		return err
	}
	return nil
}

// CleanupRateLimitData removes all rate limit keys for a given subscription.
func CleanupRateLimitData(subscriptionId int) error {
	if common.RDB == nil {
		return nil
	}
	ctx := context.Background()
	pattern := fmt.Sprintf("rate_limit:%d:*", subscriptionId)

	var cursor uint64
	for {
		keys, nextCursor, err := common.RDB.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			logger.LogError(nil, fmt.Sprintf("rate limit key scan failed for subscription %d: %v", subscriptionId, err))
			return err
		}
		if len(keys) > 0 {
			_, err := common.RDB.Del(ctx, keys...).Result()
			if err != nil {
				logger.LogError(nil, fmt.Sprintf("rate limit key delete failed for subscription %d: %v", subscriptionId, err))
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}
