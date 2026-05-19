package service

import (
	"testing"
	"time"
)

func TestCalc5hWindowKey(t *testing.T) {
	tests := []struct {
		name     string
		utcHour  int
		expected int
	}{
		{"0:00-4:59 -> window 0", 0, 0},
		{"5:00-9:59 -> window 1", 5, 1},
		{"10:00-14:59 -> window 2", 10, 2},
		{"15:00-19:59 -> window 3", 15, 3},
		{"20:00-23:59 -> window 4", 20, 4},
		{"4:59 -> window 0", 4, 0},
		{"23:00 -> window 4", 23, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Calc5hWindowKey(tt.utcHour)
			if got != tt.expected {
				t.Errorf("Calc5hWindowKey(%d) = %d, want %d", tt.utcHour, got, tt.expected)
			}
		})
	}
}

func TestCalc5hWindowResetAt(t *testing.T) {
	// UTC 2026-05-18 03:30:00 is in window 0 (0:00-4:59), reset at 05:00
	ts := time.Date(2026, 5, 18, 3, 30, 0, 0, time.UTC)
	resetAt := Calc5hWindowResetAt(ts)
	expected := time.Date(2026, 5, 18, 5, 0, 0, 0, time.UTC).Unix()
	if resetAt != expected {
		t.Errorf("Calc5hWindowResetAt(03:30) = %d, want %d", resetAt, expected)
	}

	// UTC 2026-05-18 21:00:00 is in window 4 (20:00-23:59), reset at next day 00:00
	ts2 := time.Date(2026, 5, 18, 21, 0, 0, 0, time.UTC)
	resetAt2 := Calc5hWindowResetAt(ts2)
	expected2 := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC).Unix()
	if resetAt2 != expected2 {
		t.Errorf("Calc5hWindowResetAt(21:00) = %d, want %d", resetAt2, expected2)
	}
}

func TestCalcWeekWindowKey(t *testing.T) {
	// 2026-05-18 is a Monday, ISO week 21
	ts := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	got := CalcWeekWindowKey(ts)
	if got != "2026-W21" {
		t.Errorf("CalcWeekWindowKey(Monday) = %s, want 2026-W21", got)
	}

	// 2026-05-17 is a Sunday, ISO week 20
	ts2 := time.Date(2026, 5, 17, 23, 59, 59, 0, time.UTC)
	got2 := CalcWeekWindowKey(ts2)
	if got2 != "2026-W20" {
		t.Errorf("CalcWeekWindowKey(Sunday) = %s, want 2026-W20", got2)
	}
}

func TestCalcWeekResetAt(t *testing.T) {
	// Any time during week 20 resets at Monday 00:00 UTC of week 21
	ts := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	resetAt := CalcWeekResetAt(ts)
	expected := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC).Unix()
	if resetAt != expected {
		t.Errorf("CalcWeekResetAt() = %d, want %d", resetAt, expected)
	}
}

func TestRateLimitRedisKeys(t *testing.T) {
	subId := 123
	windowKey := 2
	weekKey := "2026-W20"

	k5h := RateLimitRedisKey5h(subId, windowKey)
	kWeek := RateLimitRedisKeyWeek(subId, weekKey)

	if k5h != "rate_limit:123:5h:2" {
		t.Errorf("5h key = %s, want rate_limit:123:5h:2", k5h)
	}
	if kWeek != "rate_limit:123:week:2026-W20" {
		t.Errorf("week key = %s, want rate_limit:123:week:2026-W20", kWeek)
	}
}

// --- 补充测试 ---

func TestCalc5hWindowResetAt_MiddleWindows(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			"window 1 resets at 10:00",
			time.Date(2026, 5, 18, 7, 15, 0, 0, time.UTC),
			time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
		},
		{
			"window 2 resets at 15:00",
			time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 18, 15, 0, 0, 0, time.UTC),
		},
		{
			"window 3 resets at 20:00",
			time.Date(2026, 5, 18, 18, 45, 0, 0, time.UTC),
			time.Date(2026, 5, 18, 20, 0, 0, 0, time.UTC),
		},
		{
			"exact boundary 05:00 is window 1, resets at 10:00",
			time.Date(2026, 5, 18, 5, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
		},
		{
			"exact boundary 10:00 is window 2, resets at 15:00",
			time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 18, 15, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Calc5hWindowResetAt(tt.input)
			want := tt.expected.Unix()
			if got != want {
				t.Errorf("Calc5hWindowResetAt(%v) = %d, want %d", tt.input, got, want)
			}
		})
	}
}

func TestCalcWeekResetAt_DifferentDays(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			"Monday resets to next Monday",
			time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			"Sunday resets to next day Monday",
			time.Date(2026, 5, 24, 23, 59, 59, 0, time.UTC),
			time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			"Saturday resets to Monday 2 days later",
			time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			"Friday resets to Monday 3 days later",
			time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcWeekResetAt(tt.input)
			want := tt.expected.Unix()
			if got != want {
				t.Errorf("CalcWeekResetAt(%v) = %d, want %d", tt.input, got, want)
			}
		})
	}
}

func TestCalcWeekWindowKey_YearBoundary(t *testing.T) {
	// 2026-01-01 is a Thursday, ISO week 1 of 2026
	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	got := CalcWeekWindowKey(ts)
	if got != "2026-W01" {
		t.Errorf("CalcWeekWindowKey(2026-01-01) = %s, want 2026-W01", got)
	}

	// 2025-12-31 is a Wednesday, ISO week 1 of 2026
	ts2 := time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)
	got2 := CalcWeekWindowKey(ts2)
	if got2 != "2026-W01" {
		t.Errorf("CalcWeekWindowKey(2025-12-31) = %s, want 2026-W01", got2)
	}
}

func TestRateLimitRedisKeys_BoundaryValues(t *testing.T) {
	// subscriptionId = 1, windowKey = 0
	k5h := RateLimitRedisKey5h(1, 0)
	if k5h != "rate_limit:1:5h:0" {
		t.Errorf("5h key = %s, want rate_limit:1:5h:0", k5h)
	}

	// subscriptionId = 0 with week key
	kWeek := RateLimitRedisKeyWeek(0, "2026-W01")
	if kWeek != "rate_limit:0:week:2026-W01" {
		t.Errorf("week key = %s, want rate_limit:0:week:2026-W01", kWeek)
	}

	// windowKey = 4 (max valid)
	k5h2 := RateLimitRedisKey5h(999, 4)
	if k5h2 != "rate_limit:999:5h:4" {
		t.Errorf("5h key = %s, want rate_limit:999:5h:4", k5h2)
	}
}
