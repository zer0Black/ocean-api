package service

import (
	"fmt"
	"time"
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
