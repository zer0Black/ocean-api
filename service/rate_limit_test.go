package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
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

// --- T3: 限速服务业务操作测试 ---

// setupMiniredis creates a miniredis server and sets common.RDB to point to it.
// Returns the miniredis instance and a cleanup function to restore the original RDB.
func setupMiniredis(t *testing.T) (*miniredis.Miniredis, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	origRDB := common.RDB
	common.RDB = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return mr, func() {
		common.RDB = origRDB
		mr.Close()
	}
}

func TestCheckRateLimit_NotExceeded(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()

	subId := 1
	limit5h := 500000
	weeklyMultiplier := 20

	now := time.Now().UTC()
	windowKey5h := Calc5hWindowKey(now.Hour())
	weekKey := CalcWeekWindowKey(now)

	// Pre-populate Redis with some usage data (100 tokens used in each window)
	redisKey5h := RateLimitRedisKey5h(subId, windowKey5h)
	redisKeyWeek := RateLimitRedisKeyWeek(subId, weekKey)
	mr.ZAdd(redisKey5h, 100.0, "req_1:100")
	mr.ZAdd(redisKeyWeek, 100.0, "req_1:100")

	status, exceeded := CheckSubscriptionRateLimit(subId, limit5h, weeklyMultiplier)
	if exceeded {
		t.Error("should not be exceeded with low usage")
	}
	if status.Window5h.Used != 100 {
		t.Errorf("5h used = %d, want 100", status.Window5h.Used)
	}
	if status.Window5h.Remaining != limit5h-100 {
		t.Errorf("5h remaining = %d, want %d", status.Window5h.Remaining, limit5h-100)
	}
	if status.Window5h.Limit != limit5h {
		t.Errorf("5h limit = %d, want %d", status.Window5h.Limit, limit5h)
	}
	weekLimit := limit5h * weeklyMultiplier
	if status.WindowWeek.Used != 100 {
		t.Errorf("week used = %d, want 100", status.WindowWeek.Used)
	}
	if status.WindowWeek.Limit != weekLimit {
		t.Errorf("week limit = %d, want %d", status.WindowWeek.Limit, weekLimit)
	}
}

func TestCheckRateLimit_Exceeded5h(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()

	subId := 1
	limit5h := 100
	weeklyMultiplier := 20

	now := time.Now().UTC()
	windowKey5h := Calc5hWindowKey(now.Hour())
	redisKey5h := RateLimitRedisKey5h(subId, windowKey5h)

	// Exceed 5h limit
	mr.ZAdd(redisKey5h, 100.0, "req_1:150")

	status, exceeded := CheckSubscriptionRateLimit(subId, limit5h, weeklyMultiplier)
	if !exceeded {
		t.Error("should be exceeded when 5h usage >= limit")
	}
	if status.Window5h.Remaining != 0 {
		t.Errorf("5h remaining should be 0 when exceeded, got %d", status.Window5h.Remaining)
	}
}

func TestCheckRateLimit_ExceededWeek(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()

	subId := 1
	limit5h := 500
	weeklyMultiplier := 2 // week limit = 1000

	now := time.Now().UTC()
	weekKey := CalcWeekWindowKey(now)
	redisKeyWeek := RateLimitRedisKeyWeek(subId, weekKey)

	// Exceed week limit (week limit = 1000, set usage to 1200)
	mr.ZAdd(redisKeyWeek, 100.0, "req_1:1200")

	status, exceeded := CheckSubscriptionRateLimit(subId, limit5h, weeklyMultiplier)
	if !exceeded {
		t.Error("should be exceeded when weekly usage >= limit")
	}
	if status.WindowWeek.Remaining != 0 {
		t.Errorf("week remaining should be 0 when exceeded, got %d", status.WindowWeek.Remaining)
	}
}

func TestRecordRateLimitUsage(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()

	subId := 1
	requestId := "req_test_001"
	tokenCount := 5000

	err := RecordRateLimitUsage(subId, requestId, tokenCount)
	if err != nil {
		t.Errorf("RecordRateLimitUsage failed: %v", err)
	}

	now := time.Now().UTC()
	windowKey5h := Calc5hWindowKey(now.Hour())
	weekKey := CalcWeekWindowKey(now)

	// Verify data was written to both windows
	key5h := RateLimitRedisKey5h(subId, windowKey5h)
	keyWeek := RateLimitRedisKeyWeek(subId, weekKey)

	if !mr.Exists(key5h) {
		t.Error("expected 5h key to exist after recording")
	}
	if !mr.Exists(keyWeek) {
		t.Error("expected week key to exist after recording")
	}

	// Verify the member exists in the sorted set
	members, _ := mr.ZMembers(key5h)
	found := false
	for _, m := range members {
		if m == requestId+":5000" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected member %s:5000 in 5h sorted set, members: %v", requestId, members)
	}
}

func TestRecordRateLimitUsage_MultipleRecords(t *testing.T) {
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	subId := 1

	err := RecordRateLimitUsage(subId, "req_1", 100)
	if err != nil {
		t.Fatalf("first record failed: %v", err)
	}
	err = RecordRateLimitUsage(subId, "req_2", 200)
	if err != nil {
		t.Fatalf("second record failed: %v", err)
	}
	err = RecordRateLimitUsage(subId, "req_3", 300)
	if err != nil {
		t.Fatalf("third record failed: %v", err)
	}

	now := time.Now().UTC()
	windowKey5h := Calc5hWindowKey(now.Hour())
	key5h := RateLimitRedisKey5h(subId, windowKey5h)

	// queryWindowUsage should sum all token counts
	used := queryWindowUsage(key5h)
	if used != 600 {
		t.Errorf("total used = %d, want 600", used)
	}
}

func TestCleanupRateLimitData(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()

	subId := 1

	// Create some rate limit keys
	now := time.Now().UTC()
	key5h := RateLimitRedisKey5h(subId, Calc5hWindowKey(now.Hour()))
	keyWeek := RateLimitRedisKeyWeek(subId, CalcWeekWindowKey(now))

	mr.ZAdd(key5h, 100.0, "req_1:100")
	mr.ZAdd(keyWeek, 100.0, "req_1:100")

	if !mr.Exists(key5h) {
		t.Fatal("5h key should exist before cleanup")
	}
	if !mr.Exists(keyWeek) {
		t.Fatal("week key should exist before cleanup")
	}

	err := CleanupRateLimitData(subId)
	if err != nil {
		t.Errorf("CleanupRateLimitData failed: %v", err)
	}

	if mr.Exists(key5h) {
		t.Error("5h key should be deleted after cleanup")
	}
	if mr.Exists(keyWeek) {
		t.Error("week key should be deleted after cleanup")
	}
}

func TestCleanupRateLimitData_DoesNotAffectOtherSubscriptions(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()

	subId1 := 1
	subId2 := 2

	now := time.Now().UTC()
	key1 := RateLimitRedisKey5h(subId1, Calc5hWindowKey(now.Hour()))
	key2 := RateLimitRedisKey5h(subId2, Calc5hWindowKey(now.Hour()))

	mr.ZAdd(key1, 100.0, "req_1:100")
	mr.ZAdd(key2, 100.0, "req_2:100")

	err := CleanupRateLimitData(subId1)
	if err != nil {
		t.Fatalf("CleanupRateLimitData failed: %v", err)
	}

	if mr.Exists(key1) {
		t.Error("subId1 key should be deleted")
	}
	if !mr.Exists(key2) {
		t.Error("subId2 key should NOT be deleted")
	}
}

// --- T3: 补充边界与异常路径测试 ---

func TestCheckRateLimit_ZeroLimits(t *testing.T) {
	// limit5h <= 0 should return not exceeded with zeroed status
	status, exceeded := CheckSubscriptionRateLimit(1, 0, 20)
	if exceeded {
		t.Error("should not be exceeded with zero 5h limit")
	}
	if status.Window5h.Used != 0 || status.Window5h.Limit != 0 {
		t.Errorf("expected zeroed 5h window status, got %+v", status.Window5h)
	}
	if status.WindowWeek.Used != 0 || status.WindowWeek.Limit != 0 {
		t.Errorf("expected zeroed week window status, got %+v", status.WindowWeek)
	}

	// weeklyMultiplier <= 0 should also return not exceeded
	_, exceeded2 := CheckSubscriptionRateLimit(1, 100, 0)
	if exceeded2 {
		t.Error("should not be exceeded with zero weekly multiplier")
	}

	// Both zero
	_, exceeded3 := CheckSubscriptionRateLimit(1, 0, 0)
	if exceeded3 {
		t.Error("should not be exceeded with all zeros")
	}

	// Negative values
	_, exceeded4 := CheckSubscriptionRateLimit(1, -100, -5)
	if exceeded4 {
		t.Error("should not be exceeded with negative limits")
	}
}

func TestCheckRateLimit_NoData(t *testing.T) {
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	// Redis is empty, usage should be 0
	status, exceeded := CheckSubscriptionRateLimit(1, 1000, 10)
	if exceeded {
		t.Error("should not be exceeded with no data")
	}
	if status.Window5h.Used != 0 {
		t.Errorf("5h used = %d, want 0 with no data", status.Window5h.Used)
	}
	if status.WindowWeek.Used != 0 {
		t.Errorf("week used = %d, want 0 with no data", status.WindowWeek.Used)
	}
}

func TestCheckRateLimit_NilRDB(t *testing.T) {
	origRDB := common.RDB
	common.RDB = nil
	defer func() { common.RDB = origRDB }()

	status, exceeded := CheckSubscriptionRateLimit(1, 100, 10)
	if exceeded {
		t.Error("should not be exceeded with nil RDB")
	}
	if status.Window5h.Used != 0 {
		t.Errorf("5h used = %d, want 0 with nil RDB", status.Window5h.Used)
	}
}

func TestRecordRateLimitUsage_InvalidInput(t *testing.T) {
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	// subscriptionId <= 0 should be no-op
	err := RecordRateLimitUsage(0, "req_1", 100)
	if err != nil {
		t.Errorf("should not error with zero subscriptionId: %v", err)
	}
	err = RecordRateLimitUsage(-1, "req_1", 100)
	if err != nil {
		t.Errorf("should not error with negative subscriptionId: %v", err)
	}

	// tokenCount <= 0 should be no-op
	err = RecordRateLimitUsage(1, "req_1", 0)
	if err != nil {
		t.Errorf("should not error with zero tokenCount: %v", err)
	}
	err = RecordRateLimitUsage(1, "req_1", -50)
	if err != nil {
		t.Errorf("should not error with negative tokenCount: %v", err)
	}
}

func TestRecordRateLimitUsage_NilRDB(t *testing.T) {
	origRDB := common.RDB
	common.RDB = nil
	defer func() { common.RDB = origRDB }()

	err := RecordRateLimitUsage(1, "req_1", 100)
	if err != nil {
		t.Errorf("should not error with nil RDB: %v", err)
	}
}

func TestCleanupRateLimitData_NoKeys(t *testing.T) {
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	// No keys exist for this subscription, should succeed silently
	err := CleanupRateLimitData(999)
	if err != nil {
		t.Errorf("CleanupRateLimitData should succeed with no keys: %v", err)
	}
}

func TestCleanupRateLimitData_NilRDB(t *testing.T) {
	origRDB := common.RDB
	common.RDB = nil
	defer func() { common.RDB = origRDB }()

	err := CleanupRateLimitData(1)
	if err != nil {
		t.Errorf("CleanupRateLimitData should not error with nil RDB: %v", err)
	}
}

func TestQueryWindowUsage_NilRDB(t *testing.T) {
	origRDB := common.RDB
	common.RDB = nil
	defer func() { common.RDB = origRDB }()

	used := queryWindowUsage("rate_limit:1:5h:0")
	if used != 0 {
		t.Errorf("queryWindowUsage with nil RDB = %d, want 0", used)
	}
}

func TestCheckRateLimit_ExactAtLimit(t *testing.T) {
	mr, cleanup := setupMiniredis(t)
	defer cleanup()

	subId := 1
	limit5h := 100
	weeklyMultiplier := 10

	now := time.Now().UTC()
	redisKey5h := RateLimitRedisKey5h(subId, Calc5hWindowKey(now.Hour()))

	// Exactly at 5h limit
	mr.ZAdd(redisKey5h, 100.0, "req_1:100")

	status, exceeded := CheckSubscriptionRateLimit(subId, limit5h, weeklyMultiplier)
	if !exceeded {
		t.Error("should be exceeded when usage equals limit exactly")
	}
	if status.Window5h.Remaining != 0 {
		t.Errorf("remaining = %d, want 0 at exact limit", status.Window5h.Remaining)
	}
}

// --- T4: 限速状态查询测试 ---

// setupTestDB initializes an in-memory SQLite database for service-level tests
// that need to query model data.
func setupTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	model.DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false

	if err := db.AutoMigrate(
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Purge caches to avoid stale plan data from previous test runs
	model.PurgeAllSubscriptionPlanCaches()

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
}

func TestGetUserRateLimitStatus_InvalidUser(t *testing.T) {
	// userId <= 0 should return nil without querying DB
	statuses, err := GetUserRateLimitStatus(0)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if statuses != nil {
		t.Errorf("expected nil for userId=0, got %v", statuses)
	}

	statuses2, err := GetUserRateLimitStatus(-1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if statuses2 != nil {
		t.Errorf("expected nil for userId=-1, got %v", statuses2)
	}
}

func TestGetUserRateLimitStatus_NoSubscriptions(t *testing.T) {
	setupTestDB(t)
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	// User 99999 has no subscriptions
	statuses, err := GetUserRateLimitStatus(99999)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected empty list, got %d items", len(statuses))
	}
}

func TestGetUserRateLimitStatus_WithCodingPlan(t *testing.T) {
	setupTestDB(t)
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	// Create a coding_plan
	plan := &model.SubscriptionPlan{
		Title:                     "Test Coding Plan",
		PriceAmount:               10.0,
		Currency:                  "USD",
		DurationUnit:              model.SubscriptionDurationMonth,
		DurationValue:             1,
		Enabled:                   true,
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  500000,
		RateLimitWeeklyMultiplier: 20,
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	now := time.Now().Unix()
	sub := &model.UserSubscription{
		UserId:                    1,
		PlanId:                    plan.Id,
		AmountTotal:               0,
		AmountUsed:                0,
		StartTime:                 now - 100,
		EndTime:                   now + 86400*30,
		Status:                    "active",
		Source:                    "admin",
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  500000,
		RateLimitWeeklyMultiplier: 20,
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	statuses, err := GetUserRateLimitStatus(1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	s := statuses[0]
	if s.PlanType != model.PlanTypeCodingPlan {
		t.Errorf("expected plan_type %s, got %s", model.PlanTypeCodingPlan, s.PlanType)
	}
	if s.PlanTitle != "Test Coding Plan" {
		t.Errorf("expected plan_title 'Test Coding Plan', got '%s'", s.PlanTitle)
	}
	if s.SubscriptionId != sub.Id {
		t.Errorf("expected subscription_id %d, got %d", sub.Id, s.SubscriptionId)
	}
	if s.Window5h.Limit != 500000 {
		t.Errorf("expected 5h limit 500000, got %d", s.Window5h.Limit)
	}
	if s.Window5h.Used != 0 {
		t.Errorf("expected 5h used 0, got %d", s.Window5h.Used)
	}
	weekLimit := 500000 * 20
	if s.WindowWeek.Limit != weekLimit {
		t.Errorf("expected week limit %d, got %d", weekLimit, s.WindowWeek.Limit)
	}
}

// --- T4: 补充边界与异常路径测试 ---

func TestGetUserRateLimitStatus_IgnoresAPIPlanSubscriptions(t *testing.T) {
	setupTestDB(t)
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	// Create an api plan (not coding_plan)
	plan := &model.SubscriptionPlan{
		Title:         "API Plan",
		PriceAmount:   5.0,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		PlanType:      model.PlanTypeAPI,
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	now := time.Now().Unix()
	sub := &model.UserSubscription{
		UserId:    1,
		PlanId:    plan.Id,
		StartTime: now - 100,
		EndTime:   now + 86400*30,
		Status:    "active",
		Source:    "admin",
		PlanType:  model.PlanTypeAPI,
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	// API subscriptions should not appear in rate limit status
	statuses, err := GetUserRateLimitStatus(1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected empty list for api-only subscriptions, got %d", len(statuses))
	}
}

func TestGetUserRateLimitStatus_IgnoresExpiredCodingPlans(t *testing.T) {
	setupTestDB(t)
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	plan := &model.SubscriptionPlan{
		Title:                     "Expired Coding Plan",
		PriceAmount:               10.0,
		DurationUnit:              model.SubscriptionDurationMonth,
		DurationValue:             1,
		Enabled:                   true,
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  100000,
		RateLimitWeeklyMultiplier: 10,
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	now := time.Now().Unix()
	sub := &model.UserSubscription{
		UserId:                    1,
		PlanId:                    plan.Id,
		StartTime:                 now - 86400*60,
		EndTime:                   now - 100, // expired
		Status:                    "active",
		Source:                    "admin",
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  100000,
		RateLimitWeeklyMultiplier: 10,
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	// Expired subscriptions should not appear
	statuses, err := GetUserRateLimitStatus(1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected empty list for expired subscription, got %d", len(statuses))
	}
}

func TestGetUserRateLimitStatus_MultipleCodingPlans(t *testing.T) {
	setupTestDB(t)
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	plan1 := &model.SubscriptionPlan{
		Title:                     "Coding Plan A",
		PriceAmount:               10.0,
		DurationUnit:              model.SubscriptionDurationMonth,
		DurationValue:             1,
		Enabled:                   true,
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  500000,
		RateLimitWeeklyMultiplier: 20,
	}
	if err := model.DB.Create(plan1).Error; err != nil {
		t.Fatalf("failed to create plan1: %v", err)
	}

	plan2 := &model.SubscriptionPlan{
		Title:                     "Coding Plan B",
		PriceAmount:               20.0,
		DurationUnit:              model.SubscriptionDurationMonth,
		DurationValue:             1,
		Enabled:                   true,
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  1000000,
		RateLimitWeeklyMultiplier: 15,
	}
	if err := model.DB.Create(plan2).Error; err != nil {
		t.Fatalf("failed to create plan2: %v", err)
	}

	now := time.Now().Unix()
	for i, p := range []*model.SubscriptionPlan{plan1, plan2} {
		sub := &model.UserSubscription{
			UserId:                     1,
			PlanId:                     p.Id,
			StartTime:                  now - 100,
			EndTime:                    now + 86400*30 + int64(i)*100,
			Status:                     "active",
			Source:                     "admin",
			PlanType:                   model.PlanTypeCodingPlan,
			RateLimitTokensPerWindow:   p.RateLimitTokensPerWindow,
			RateLimitWeeklyMultiplier:  p.RateLimitWeeklyMultiplier,
		}
		if err := model.DB.Create(sub).Error; err != nil {
			t.Fatalf("failed to create subscription %d: %v", i, err)
		}
	}

	statuses, err := GetUserRateLimitStatus(1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	// Verify each status has correct plan info
	titles := map[string]bool{}
	for _, s := range statuses {
		titles[s.PlanTitle] = true
		if s.PlanType != model.PlanTypeCodingPlan {
			t.Errorf("expected plan_type coding_plan, got %s", s.PlanType)
		}
	}
	if !titles["Coding Plan A"] || !titles["Coding Plan B"] {
		t.Errorf("expected both plan titles, got %v", titles)
	}
}

func TestGetUserRateLimitStatus_ZeroRateLimits(t *testing.T) {
	setupTestDB(t)
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	// CodingPlan with zero rate limits
	plan := &model.SubscriptionPlan{
		Title:                     "Zero Limit Plan",
		DurationUnit:              model.SubscriptionDurationMonth,
		DurationValue:             1,
		Enabled:                   true,
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  0,
		RateLimitWeeklyMultiplier: 0,
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	now := time.Now().Unix()
	sub := &model.UserSubscription{
		UserId:                    1,
		PlanId:                    plan.Id,
		StartTime:                 now - 100,
		EndTime:                   now + 86400*30,
		Status:                    "active",
		Source:                    "admin",
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  0,
		RateLimitWeeklyMultiplier: 0,
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	statuses, err := GetUserRateLimitStatus(1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	// Zero limits should still return a status with zeroed windows
	s := statuses[0]
	if s.Window5h.Limit != 0 {
		t.Errorf("expected 5h limit 0, got %d", s.Window5h.Limit)
	}
	if s.WindowWeek.Limit != 0 {
		t.Errorf("expected week limit 0, got %d", s.WindowWeek.Limit)
	}
}

func TestGetUserRateLimitStatus_DeletedPlan(t *testing.T) {
	setupTestDB(t)
	_, cleanup := setupMiniredis(t)
	defer cleanup()

	// Create a plan then delete it
	plan := &model.SubscriptionPlan{
		Title:                     "To Be Deleted",
		DurationUnit:              model.SubscriptionDurationMonth,
		DurationValue:             1,
		Enabled:                   true,
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  500000,
		RateLimitWeeklyMultiplier: 20,
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	now := time.Now().Unix()
	sub := &model.UserSubscription{
		UserId:                    1,
		PlanId:                    plan.Id,
		StartTime:                 now - 100,
		EndTime:                   now + 86400*30,
		Status:                    "active",
		Source:                    "admin",
		PlanType:                  model.PlanTypeCodingPlan,
		RateLimitTokensPerWindow:  500000,
		RateLimitWeeklyMultiplier: 20,
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	// Delete the plan
	model.DB.Delete(plan)

	// Should still return status with empty plan title
	statuses, err := GetUserRateLimitStatus(1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].PlanTitle != "" {
		t.Errorf("expected empty plan_title for deleted plan, got '%s'", statuses[0].PlanTitle)
	}
}

// --- T5: 套餐 CRUD 校验变更测试 ---

func TestValidateRateLimitParams_APIPlan(t *testing.T) {
	err := ValidateRateLimitParams("api", 0, 0)
	if err != nil {
		t.Errorf("API plan with zero rate limit fields should pass: %v", err)
	}
}

func TestValidateRateLimitParams_CodingPlanValid(t *testing.T) {
	err := ValidateRateLimitParams("coding_plan", 500000, 20)
	if err != nil {
		t.Errorf("valid CodingPlan should pass: %v", err)
	}
}

func TestValidateRateLimitParams_CodingPlanMissingTokens(t *testing.T) {
	err := ValidateRateLimitParams("coding_plan", 0, 20)
	if err == nil {
		t.Error("CodingPlan with zero tokens_per_window should fail")
	}
}

func TestValidateRateLimitParams_CodingPlanMissingMultiplier(t *testing.T) {
	err := ValidateRateLimitParams("coding_plan", 500000, 0)
	if err == nil {
		t.Error("CodingPlan with zero weekly_multiplier should fail")
	}
}

func TestValidateRateLimitParams_CodingPlanNegativeTokens(t *testing.T) {
	err := ValidateRateLimitParams("coding_plan", -1, 20)
	if err == nil {
		t.Error("CodingPlan with negative tokens_per_window should fail")
	}
}

func TestValidateRateLimitParams_InvalidPlanType(t *testing.T) {
	err := ValidateRateLimitParams("invalid", 0, 0)
	if err == nil {
		t.Error("invalid plan_type should fail")
	}
}

// --- T5: 补充边界与异常路径测试 ---

func TestValidateRateLimitParams_CodingPlanNegativeMultiplier(t *testing.T) {
	err := ValidateRateLimitParams("coding_plan", 500000, -1)
	if err == nil {
		t.Error("CodingPlan with negative weekly_multiplier should fail")
	}
}

func TestValidateRateLimitParams_APIPlanWithNonZeroFields(t *testing.T) {
	// API plan should accept even non-zero rate limit fields (they get zeroed by controller)
	err := ValidateRateLimitParams("api", 999, 50)
	if err != nil {
		t.Errorf("API plan should always pass validation: %v", err)
	}
}

func TestValidateRateLimitParams_EmptyPlanType(t *testing.T) {
	err := ValidateRateLimitParams("", 0, 0)
	if err == nil {
		t.Error("empty plan_type should fail")
	}
}

func TestValidateRateLimitParams_I18nErrorType_TokensRequired(t *testing.T) {
	err := ValidateRateLimitParams("coding_plan", 0, 20)
	if err == nil {
		t.Fatal("expected error for missing tokens_per_window")
	}
	i18nErr, ok := err.(*I18nError)
	if !ok {
		t.Fatalf("expected *I18nError, got %T", err)
	}
	if i18nErr.Key != "subscription.rate_limit_tokens_required" {
		t.Errorf("expected key subscription.rate_limit_tokens_required, got %s", i18nErr.Key)
	}
}

func TestValidateRateLimitParams_I18nErrorType_MultiplierRequired(t *testing.T) {
	err := ValidateRateLimitParams("coding_plan", 500000, 0)
	if err == nil {
		t.Fatal("expected error for missing weekly_multiplier")
	}
	i18nErr, ok := err.(*I18nError)
	if !ok {
		t.Fatalf("expected *I18nError, got %T", err)
	}
	if i18nErr.Key != "subscription.rate_limit_multiplier_required" {
		t.Errorf("expected key subscription.rate_limit_multiplier_required, got %s", i18nErr.Key)
	}
}

func TestValidateRateLimitParams_I18nErrorType_InvalidPlanType(t *testing.T) {
	err := ValidateRateLimitParams("foobar", 0, 0)
	if err == nil {
		t.Fatal("expected error for invalid plan_type")
	}
	i18nErr, ok := err.(*I18nError)
	if !ok {
		t.Fatalf("expected *I18nError, got %T", err)
	}
	if i18nErr.Key != "subscription.rate_limit_invalid_plan_type" {
		t.Errorf("expected key subscription.rate_limit_invalid_plan_type, got %s", i18nErr.Key)
	}
	if i18nErr.Args == nil || i18nErr.Args["plan_type"] != "foobar" {
		t.Errorf("expected Args to contain plan_type=foobar, got %v", i18nErr.Args)
	}
}

func TestValidateRateLimitParams_CodingPlanExactOne(t *testing.T) {
	// Minimum valid values (both = 1)
	err := ValidateRateLimitParams("coding_plan", 1, 1)
	if err != nil {
		t.Errorf("CodingPlan with tokens=1 and multiplier=1 should pass: %v", err)
	}
}

// --- T6: 订阅快照 + 过期清理钩子测试 ---

func TestCleanupOnSubscriptionExpiry(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	// Pre-populate rate limit keys for subscription 1
	mr.ZAdd("rate_limit:1:5h:2", 100.0, "req_1:100")
	mr.ZAdd("rate_limit:1:week:2026-W20", 200.0, "req_2:200")

	origClient := common.RDB
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { common.RDB = origClient }()

	err = CleanupRateLimitData(1)
	if err != nil {
		t.Errorf("CleanupRateLimitData failed: %v", err)
	}

	// Verify keys are removed
	if mr.Exists("rate_limit:1:5h:2") {
		t.Error("expected 5h key to be deleted after cleanup")
	}
	if mr.Exists("rate_limit:1:week:2026-W20") {
		t.Error("expected week key to be deleted after cleanup")
	}
}

// --- T6: 补充测试 ---

func TestCleanupOnSubscriptionExpiry_MultipleWindowKeys(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	// Pre-populate rate limit keys across multiple windows for subscription 42
	mr.ZAdd("rate_limit:42:5h:0", 100.0, "req_1:100")
	mr.ZAdd("rate_limit:42:5h:1", 200.0, "req_2:200")
	mr.ZAdd("rate_limit:42:5h:4", 300.0, "req_3:300")
	mr.ZAdd("rate_limit:42:week:2026-W19", 150.0, "req_4:150")
	mr.ZAdd("rate_limit:42:week:2026-W20", 250.0, "req_5:250")

	origClient := common.RDB
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { common.RDB = origClient }()

	err = CleanupRateLimitData(42)
	if err != nil {
		t.Errorf("CleanupRateLimitData failed: %v", err)
	}

	// Verify all 5 keys are removed
	for _, key := range []string{
		"rate_limit:42:5h:0",
		"rate_limit:42:5h:1",
		"rate_limit:42:5h:4",
		"rate_limit:42:week:2026-W19",
		"rate_limit:42:week:2026-W20",
	} {
		if mr.Exists(key) {
			t.Errorf("expected key %s to be deleted after cleanup", key)
		}
	}
}

func TestCleanupOnSubscriptionExpiry_DoesNotAffectOtherSubscriptions(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	// Create keys for both subscription 1 and subscription 2
	mr.ZAdd("rate_limit:1:5h:2", 100.0, "req_1:100")
	mr.ZAdd("rate_limit:2:5h:2", 200.0, "req_2:200")
	mr.ZAdd("rate_limit:1:week:2026-W20", 300.0, "req_3:300")
	mr.ZAdd("rate_limit:2:week:2026-W20", 400.0, "req_4:400")

	origClient := common.RDB
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { common.RDB = origClient }()

	// Cleanup only subscription 1
	err = CleanupRateLimitData(1)
	if err != nil {
		t.Errorf("CleanupRateLimitData failed: %v", err)
	}

	// Subscription 1 keys should be gone
	if mr.Exists("rate_limit:1:5h:2") {
		t.Error("expected subscription 1 5h key to be deleted")
	}
	if mr.Exists("rate_limit:1:week:2026-W20") {
		t.Error("expected subscription 1 week key to be deleted")
	}

	// Subscription 2 keys should remain
	if !mr.Exists("rate_limit:2:5h:2") {
		t.Error("expected subscription 2 5h key to remain")
	}
	if !mr.Exists("rate_limit:2:week:2026-W20") {
		t.Error("expected subscription 2 week key to remain")
	}
}
