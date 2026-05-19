package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// I18nError wraps an i18n message key with optional template arguments.
// It implements the error interface so it can be used where error is expected,
// while also carrying structured i18n data for the controller layer.
type I18nError struct {
	Key  string
	Args map[string]any
}

func (e *I18nError) Error() string {
	return e.Key
}

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

// GetUserRateLimitStatus returns rate limit status for all active CodingPlan subscriptions of a user.
func GetUserRateLimitStatus(userId int) ([]RateLimitStatus, error) {
	if userId <= 0 {
		return nil, nil
	}
	subs, err := model.GetActiveCodingPlanSubscriptions(userId)
	if err != nil {
		return nil, err
	}
	if len(subs) == 0 {
		return []RateLimitStatus{}, nil
	}

	result := make([]RateLimitStatus, 0, len(subs))
	for _, sub := range subs {
		status, _ := CheckSubscriptionRateLimit(sub.Id, sub.RateLimitTokensPerWindow, sub.RateLimitWeeklyMultiplier)
		plan, _ := model.GetSubscriptionPlanById(sub.PlanId)
		planTitle := ""
		if plan != nil {
			planTitle = plan.Title
		}
		status.PlanTitle = planTitle
		status.PlanType = sub.PlanType
		result = append(result, *status)
	}
	return result, nil
}

// ValidateRateLimitParams validates rate limit parameters based on plan type.
// Returns an *I18nError on validation failure, which carries an i18n key for
// the controller to pass to common.ApiErrorI18n.
func ValidateRateLimitParams(planType string, tokensPerWindow int, weeklyMultiplier int) error {
	switch planType {
	case model.PlanTypeAPI:
		return nil
	case model.PlanTypeCodingPlan:
		if tokensPerWindow < 1 {
			return &I18nError{Key: i18n.MsgRateLimitTokensRequired}
		}
		if weeklyMultiplier < 1 {
			return &I18nError{Key: i18n.MsgRateLimitMultiplierReq}
		}
		return nil
	default:
		return &I18nError{Key: i18n.MsgRateLimitInvalidPlanType, Args: map[string]any{"plan_type": planType}}
	}
}

// CheckUserCodingPlanRateLimit checks if the user has exceeded rate limits
// on any of their active CodingPlan subscriptions.
func CheckUserCodingPlanRateLimit(userId int) (bool, *RateLimitStatus) {
	if userId <= 0 {
		return false, nil
	}
	subs, err := model.GetActiveCodingPlanSubscriptions(userId)
	if err != nil || len(subs) == 0 {
		return false, nil
	}
	for _, sub := range subs {
		status, exceeded := CheckSubscriptionRateLimit(sub.Id, sub.RateLimitTokensPerWindow, sub.RateLimitWeeklyMultiplier)
		if exceeded {
			return true, status
		}
	}
	return false, nil
}

// InjectRateLimitHeaders injects X-RateLimit-* response headers for a subscription.
func InjectRateLimitHeaders(c *gin.Context, subscriptionId int) {
	if common.RDB == nil || c == nil {
		return
	}
	sub, err := model.GetUserSubscriptionById(subscriptionId)
	if err != nil || sub == nil || sub.PlanType != model.PlanTypeCodingPlan {
		return
	}
	status, _ := CheckSubscriptionRateLimit(sub.Id, sub.RateLimitTokensPerWindow, sub.RateLimitWeeklyMultiplier)
	if status == nil {
		return
	}
	c.Header("X-RateLimit-Limit-5h", strconv.Itoa(status.Window5h.Limit))
	c.Header("X-RateLimit-Remaining-5h", strconv.Itoa(status.Window5h.Remaining))
	c.Header("X-RateLimit-Reset-5h", strconv.FormatInt(status.Window5h.ResetAt, 10))
	c.Header("X-RateLimit-Limit-Week", strconv.Itoa(status.WindowWeek.Limit))
	c.Header("X-RateLimit-Remaining-Week", strconv.Itoa(status.WindowWeek.Remaining))
	c.Header("X-RateLimit-Reset-Week", strconv.FormatInt(status.WindowWeek.ResetAt, 10))
}
