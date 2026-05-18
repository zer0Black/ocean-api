# 订阅速率限制 - 数据库模型设计

## 基础信息

**模块前缀：** sub_rate_limit（扩展现有 subscription 模块）

**数据库类型：** 关系型（SQLite / MySQL >= 5.7.8 / PostgreSQL >= 9.6，三库兼容）+ Redis

**说明：** 本功能不创建新表，仅在现有 `subscription_plans` 和 `user_subscriptions` 表上新增字段。限速计数器数据存储在 Redis Sorted Set 中。

**主键策略：** 沿用项目现有约定，`int` 类型主键，GORM 自动递增。

**公共字段：** 沿用现有模型的 `CreatedAt`（int64 Unix 时间戳）和 `UpdatedAt`（int64 Unix 时间戳）。

**字段命名：** Go 结构体 PascalCase，GORM 自动转换为 snake_case 数据库列名。JSON key 使用 snake_case，与数据库列名一致。

---

## ER 图

```
subscription_plans (1) ────── (N) user_subscriptions
    │                                │
    │ 新增字段:                       │ 新增字段（快照）:
    │ - plan_type                    │ - plan_type
    │ - rate_limit_tokens_per_window │ - rate_limit_tokens_per_window
    │ - rate_limit_weekly_multiplier │ - rate_limit_weekly_multiplier
    │                                │
    │                                └── (关联) Redis Sorted Set
    │                                      rate_limit:{sub_id}:5h:{window_key}
    │                                      rate_limit:{sub_id}:week:{week_key}

user_subscriptions.status:
  - active  → 执行限速检查和用量记录
  - expired → 停止限速检查，触发数据清理
  - cancelled → 停止限速检查，触发数据清理
```

**实体关系说明：**
- `subscription_plans` 定义套餐的限速参数配置
- `user_subscriptions` 在创建时从套餐快照限速参数，后续限速检查使用订阅上的快照值
- Redis Sorted Set 存储每个订阅的实际 token 消耗记录，与 `user_subscriptions.id` 关联

---

## 表结构变更定义

### 1. subscription_plans（套餐表）新增字段

**表名：** `subscription_plans`（GORM 默认约定，SubscriptionPlan 复数化）

**用途：** 在套餐定义中增加限速配置参数，管理员创建/编辑 CodingPlan 套餐时设置。

---

**GORM 模型变更（Go 结构体新增字段）：**

```go
type SubscriptionPlan struct {
    Id int `json:"id"`

    // ...现有字段保持不变（Title, Subtitle, PriceAmount, Currency 等）...

    // === 以下为新增字段 ===

    // 套餐类型：api（API 套餐）、coding_plan（CodingPlan 套餐）
    PlanType string `json:"plan_type" gorm:"type:varchar(32);not null;default:'api'"`

    // 5 小时窗口 token 上限（仅 CodingPlan 时有效，API 套餐为 0）
    RateLimitTokensPerWindow int `json:"rate_limit_tokens_per_window" gorm:"type:int;not null;default:0"`

    // 周倍数（周上限 = 5 小时上限 × 周倍数，仅 CodingPlan 时有效，API 套餐为 0）
    RateLimitWeeklyMultiplier int `json:"rate_limit_weekly_multiplier" gorm:"type:int;not null;default:0"`

    // ...现有字段保持不变（CreatedAt, UpdatedAt 等）...
}
```

---

**新增字段说明：**

| 字段名 | Go 类型 | GORM 类型 | 必填 | 默认值 | 说明 |
|--------|---------|-----------|------|--------|------|
| PlanType | string | varchar(32) | 是 | "api" | 套餐类型 [字典：sub_plan_type] |
| RateLimitTokensPerWindow | int | int | 是 | 0 | 5 小时窗口 token 上限。CodingPlan 时 ≥ 1，API 套餐为 0 [长度来源：需求规格说明书] |
| RateLimitWeeklyMultiplier | int | int | 是 | 0 | 周倍数。CodingPlan 时 ≥ 1，API 套餐为 0 [长度来源：需求规格说明书] |

**新增索引说明：**

无需新增索引。限速配置通过 PlanId 关联查询，不直接按限速参数字段查询。`plan_type` 字段基数低（仅 2 个值），单独索引收益有限。

**业务规则：**

- `plan_type` 为 `"api"` 时，`rate_limit_tokens_per_window` 和 `rate_limit_weekly_multiplier` 应为 0
- `plan_type` 为 `"coding_plan"` 时，`rate_limit_tokens_per_window` ≥ 1 且 `rate_limit_weekly_multiplier` ≥ 1
- 修改套餐限速参数不影响已有订阅（订阅创建时快照限速参数到 UserSubscription）
- 周上限由系统自动计算：`rate_limit_tokens_per_window × rate_limit_weekly_multiplier`

---

### 2. user_subscriptions（用户订阅表）新增字段

**表名：** `user_subscriptions`（GORM 默认约定，UserSubscription 复数化）

**用途：** 在用户订阅中保存购买时的限速参数快照。限速检查使用订阅上的快照值，而非套餐上的最新值，确保管理员修改套餐配置不影响已有订阅。

---

**GORM 模型变更（Go 结构体新增字段）：**

```go
type UserSubscription struct {
    Id     int `json:"id"`
    UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
    PlanId int `json:"plan_id" gorm:"index"`

    // ...现有字段保持不变（AmountTotal, AmountUsed, StartTime, EndTime, Status 等）...

    // === 以下为新增字段（购买时从 SubscriptionPlan 快照） ===

    // 套餐类型快照：api / coding_plan
    PlanType string `json:"plan_type" gorm:"type:varchar(32);not null;default:'api'"`

    // 5 小时窗口 token 上限快照
    RateLimitTokensPerWindow int `json:"rate_limit_tokens_per_window" gorm:"type:int;not null;default:0"`

    // 周倍数快照
    RateLimitWeeklyMultiplier int `json:"rate_limit_weekly_multiplier" gorm:"type:int;not null;default:0"`

    // ...现有字段保持不变（UpgradeGroup, PrevUserGroup, CreatedAt, UpdatedAt 等）...
}
```

---

**新增字段说明：**

| 字段名 | Go 类型 | GORM 类型 | 必填 | 默认值 | 说明 |
|--------|---------|-----------|------|--------|------|
| PlanType | string | varchar(32) | 是 | "api" | 套餐类型快照 [字典：sub_plan_type]。限速检查时根据此字段判断是否执行 |
| RateLimitTokensPerWindow | int | int | 是 | 0 | 5 小时窗口 token 上限快照 [长度来源：需求规格说明书]。限速检查使用此值 |
| RateLimitWeeklyMultiplier | int | int | 是 | 0 | 周倍数快照 [长度来源：需求规格说明书]。周上限 = 此值 × rate_limit_tokens_per_window |

**新增索引说明：**

无需新增索引。限速检查时查询用户活跃订阅的路径为 `WHERE user_id = ? AND status = 'active'`，已有复合索引 `idx_user_sub_active`（user_id, status, end_time）覆盖。`plan_type` 在应用层过滤即可。

**业务规则：**

- 这三个字段在订阅创建时（管理员绑定或用户支付成功后）从 SubscriptionPlan 快照写入
- 快照值在订阅生命周期内不变更，即使管理员修改了套餐的限速参数
- `plan_type` 为 `"coding_plan"` 的订阅才参与限速检查，`"api"` 类型的订阅跳过
- 订阅状态变为 expired 或 cancelled 时，触发 Redis 限速数据清理（参见 Redis 设计章节）

**现有完整字段参考（含已有字段）：**

| 字段名 | Go 类型 | GORM 类型 | 必填 | 默认值 | 说明 |
|--------|---------|-----------|------|--------|------|
| Id | int | - | 是 | 自增 | 主键 |
| UserId | int | index | 是 | - | 用户 ID |
| PlanId | int | index | 是 | - | 套餐 ID |
| AmountTotal | int64 | bigint | 是 | 0 | 总额度 |
| AmountUsed | int64 | bigint | 是 | 0 | 已使用额度 |
| StartTime | int64 | bigint | 是 | - | 订阅开始时间（Unix 时间戳） |
| EndTime | int64 | bigint, index | 是 | - | 订阅结束时间（Unix 时间戳） |
| Status | string | varchar(32), index | 是 | - | 状态：active / expired / cancelled |
| Source | string | varchar(32) | 是 | "order" | 来源：order / admin |
| LastResetTime | int64 | bigint | 是 | 0 | 上次配额重置时间 |
| NextResetTime | int64 | bigint, index | 是 | 0 | 下次配额重置时间 |
| UpgradeGroup | string | varchar(64) | 是 | "" | 购买后升级的用户分组 |
| PrevUserGroup | string | varchar(64) | 是 | "" | 购买前的用户分组 |
| **PlanType** | **string** | **varchar(32)** | **是** | **"api"** | **套餐类型快照 [字典：sub_plan_type]** |
| **RateLimitTokensPerWindow** | **int** | **int** | **是** | **0** | **5 小时 token 上限快照** |
| **RateLimitWeeklyMultiplier** | **int** | **int** | **是** | **0** | **周倍数快照** |
| CreatedAt | int64 | bigint | 是 | - | 创建时间（Unix 时间戳） |
| UpdatedAt | int64 | bigint | 是 | - | 更新时间（Unix 时间戳） |

**现有索引参考：**

| 索引名 | 类型 | 字段 | 用途 |
|--------|------|------|------|
| idx_user_sub_active | INDEX | user_id, status, end_time | 查询用户活跃订阅 |
| user_id | INDEX | user_id | 按用户查询订阅 |
| plan_id | INDEX | plan_id | 按套餐查询订阅 |
| end_time | INDEX | end_time | 按结束时间查询（过期扫描） |
| status | INDEX | status | 按状态过滤 |
| next_reset_time | INDEX | next_reset_time | 配额重置定时任务 |

---

## Redis 数据设计

### 3. 限速计数器（Redis Sorted Set）

**用途：** 记录 CodingPlan 订阅每次 API 请求的 token 消耗量，作为限速检查的数据来源。

**Key 格式：**

| Key 模式 | 说明 | TTL |
|----------|------|-----|
| `rate_limit:{subscription_id}:5h:{window_key}` | 5 小时窗口内的 token 消耗记录 | 6 小时（21600 秒） |
| `rate_limit:{subscription_id}:week:{week_key}` | 自然周内的 token 消耗记录 | 8 天（691200 秒） |

**数据结构：** Redis Sorted Set
- **Score**: 请求完成时的 Unix 时间戳（秒）
- **Member**: `{request_id}:{token_count}`（例如 `req_abc123:5000`）

**window_key 计算规则：**

| 窗口类型 | key 计算方式 | 示例值 | 说明 |
|---------|-------------|--------|------|
| 5 小时窗口 | `floor(hour(UTC) / 5)` | 0, 1, 2, 3, 4 | UTC 0:00 起每 5 小时一个编号 |
| 周窗口 | `{year}-W{ISO_week_number}` | 2026-W20 | ISO 8601 周编号 |

**完整 Key 示例：**

```
rate_limit:123:5h:2          → 订阅 123 的第 2 个 5h 窗口（10:00-15:00 UTC）
rate_limit:123:week:2026-W20 → 订阅 123 的 2026 年第 20 周数据
```

**操作说明：**

| 操作 | Redis 命令 | 说明 |
|------|-----------|------|
| 记录消耗 | `ZADD key <timestamp> <request_id>:<token_count>` | 添加一条消耗记录 |
| 设置 TTL | `EXPIRE key <seconds>` | 首次写入时设置 |
| 查询窗口内用量 | `ZRANGEBYSCORE key <window_start> <window_end>` | 获取窗口内所有记录 |
| 计算总量 | 遍历 ZRANGEBYSCORE 结果，累加 member 中的 token_count | 应用层计算 |
| 清理订阅数据 | `SCAN MATCH rate_limit:{subscription_id}:*` + `DEL` | 订阅过期/取消时 |

**设计说明：**

- 选择 Sorted Set 而非复用现有 `common/limiter/` 的原因参见需求规格说明书 7.1 节
- TTL 留有余量（5h 窗口设 6h TTL，周窗口设 8 天 TTL），确保窗口未过期时 key 不会提前消失
- 并发超额的容忍策略：采用事后统计模型，接受并发流式请求导致的窗口内超额，下次限速检查时基于实际累计用量判断

---

## 字典数据

### sub_plan_type（套餐类型）

项目不使用数据库字典表，枚举值通过 Go 常量定义。

**Go 常量定义：**

```go
const (
    PlanTypeAPI        = "api"         // API 套餐，按额度计费，无速率限制
    PlanTypeCodingPlan = "coding_plan" // CodingPlan 套餐，按 token 速率限制
)
```

**字典值说明：**

| 值 | 标签 | 说明 |
|----|------|------|
| api | API 套餐 | 传统 API 调用套餐，按额度消耗计费，不限制 token 速率 |
| coding_plan | CodingPlan 套餐 | Coding Plan 类型套餐，配置 5 小时窗口和周窗口的 token 速率限制 |

---

## 数据迁移方案

### 新增字段迁移

使用 GORM AutoMigrate，在 `model/subscription.go` 的 SubscriptionPlan 和 UserSubscription 结构体中添加新字段后，已有的 `migrateDB()` 中的 `DB.AutoMigrate(&SubscriptionPlan{}, &UserSubscription{})` 在应用启动时自动执行 `ADD COLUMN`。

无需编写自定义迁移函数，原因如下：

1. 所有新增字段都有 NOT NULL 约束和 DEFAULT 值，已存在的记录自动获得默认值
2. 不涉及列类型变更，纯新增字段
3. 三种数据库（SQLite / MySQL / PostgreSQL）对 `ADD COLUMN` 的语法一致

**迁移代码变更位置：**
- `model/subscription.go`：在 SubscriptionPlan 和 UserSubscription 结构体中添加新字段
- 无需修改 `model/main.go` 的 `migrateDB()` 函数，因为这两个模型已在 AutoMigrate 列表中

---

## 性能优化

### 数据库查询性能

限速检查在每次 API 请求时执行，查询路径为：
1. 根据用户 ID 查询活跃订阅：使用已有索引 `idx_user_sub_active`（user_id, status, end_time）
2. 应用层过滤 `plan_type = "coding_plan"` 的订阅
3. 使用快照值查询 Redis 限速计数器

该路径无慢查询风险。`plan_type` 基数低（仅 2 个值），不适合建索引，应用层过滤即可。

### Redis 性能考量

- 5 小时窗口内每个订阅的请求记录量：估算约 100-1000 条（取决于用户使用频率）
- 周窗口内每个订阅约 1000-10000 条
- Sorted Set 的 ZRANGEBYSCORE 时间复杂度 O(log N + M)，在此规模下无性能瓶颈
- TTL 机制确保过期数据自动清理，不无限累积
- 每次 API 请求产生 2 次 Redis 写入（5h + week），加上 1 次读取（限速检查），总计 3 次 Redis 操作，延迟可控

### 缓存策略

- 限速检查结果不缓存，每次请求实时查询 Redis，确保限速精度
- SubscriptionPlan 的限速参数可随现有套餐缓存机制缓存
- UserSubscription 的限速快照值可随现有订阅缓存机制缓存

---

## 安全措施

### 数据隔离

- Redis key 以 `subscription_id` 为维度，不同订阅的计数器互不干扰
- 用户限速状态查询 API 通过 `user_id` 过滤，确保行级数据隔离

### 降级安全

- Redis 不可用时降级放行请求，记录告警日志，确保服务可用性优先于限速精度
- 限速配置异常时（上限 ≤ 0）降级放行，避免配置错误导致用户完全无法使用

---

**文档版本：** v1.0

**最后更新：** 2026-05-18
