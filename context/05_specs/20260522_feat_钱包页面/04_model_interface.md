# 数据库模型设计文档（代码逆推）

本文档从现有代码库逆推生成，所有表结构、字段定义均与 GORM 模型代码保持一致。

---

## 基础信息

**数据库类型：** SQLite / MySQL >= 5.7.8 / PostgreSQL >= 9.6（三者兼容）

**ORM：** GORM v2

**主键策略：** `int` 类型自增主键，GORM 自动管理

**时间字段：** `int64` Unix 时间戳

**命名规则：** GORM 默认约定，结构体名复数化为 snake_case 表名

---

## ER 图

```
users (1) ──────── (N) topups
users (1) ──────── (N) redemptions（创建者）
users (1) ──────── (N) user_subscriptions
subscription_plans (1) ─── (N) user_subscriptions
subscription_plans (1) ─── (N) subscription_orders
users (1) ──────── (N) subscription_orders
users (1) ──────── (N) subscription_pre_consume_records
user_subscriptions (1) ── (N) subscription_pre_consume_records
```

---

## 表结构定义

### 1. 核心业务表

#### 1.1 topups（充值订单表）

**表名：** `topups`

**用途：** 记录所有充值订单（易支付、Stripe、Creem、Waffo 等所有支付渠道），存储订单状态和支付信息。

**GORM 模型：** `model/topup.go:14`

```go
type TopUp struct {
    Id              int     `json:"id"`
    UserId          int     `json:"user_id" gorm:"index"`
    Amount          int64   `json:"amount"`
    Money           float64 `json:"money"`
    TradeNo         string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
    PaymentMethod   string  `json:"payment_method" gorm:"type:varchar(50)"`
    PaymentProvider string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
    CreateTime      int64   `json:"create_time"`
    CompleteTime    int64   `json:"complete_time"`
    Status          string  `json:"status"`
}
```

**字段说明：**

| 字段名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | int | 是 | 自增 | 主键 ID |
| user_id | int | 是 | - | 充值用户 ID |
| amount | int64 | 是 | - | 充值配额数量（500000 = $1） |
| money | float64 | 是 | - | 实付金额 |
| trade_no | string(varchar 255) | 是 | - | 交易号，唯一标识 [字典：-] |
| payment_method | string(varchar 50) | 是 | - | 支付方式名称（如 alipay、wxpay） |
| payment_provider | string(varchar 50) | 是 | '' | 支付提供商标识 [字典：payment_provider] |
| create_time | int64 | 是 | - | 创建时间（Unix 时间戳） |
| complete_time | int64 | 是 | - | 完成时间（Unix 时间戳） |
| status | string | 是 | - | 订单状态 [字典：topup_status] |

**索引说明：**

| 索引名 | 类型 | 字段 | 用途 |
|--------|------|------|------|
| PRIMARY | PRIMARY KEY | id | 主键索引 |
| idx_topups_user_id | INDEX | user_id | 按用户查询订单 |
| idx_topups_trade_no | UNIQUE | trade_no | 交易号唯一约束 |

**payment_provider 字典值：**

| 值 | 说明 |
|----|------|
| epay | 易支付 |
| stripe | Stripe |
| creem | Creem |
| waffo | Waffo |
| waffo_pancake | Waffo Pancake |

**status 字典值（topup_status）：**

| 值 | 说明 |
|----|------|
| pending | 待支付 |
| success | 支付成功（终态） |
| failed | 支付失败（终态） |
| expired | 已过期（终态） |

---

#### 1.2 redemptions（兑换码表）

**表名：** `redemptions`

**用途：** 存储管理员创建的兑换码，支持批量创建和一次性使用。

**GORM 模型：** `model/redemption.go:14`

```go
type Redemption struct {
    Id           int            `json:"id"`
    UserId       int            `json:"user_id"`
    Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
    Status       int            `json:"status" gorm:"default:1"`
    Name         string         `json:"name" gorm:"index"`
    Quota        int            `json:"quota" gorm:"default:100"`
    CreatedTime  int64          `json:"created_time" gorm:"bigint"`
    RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
    Count        int            `json:"count" gorm:"-:all"`
    UsedUserId   int            `json:"used_user_id"`
    DeletedAt    gorm.DeletedAt `gorm:"index"`
    ExpiredTime  int64          `json:"expired_time" gorm:"bigint"`
}
```

**字段说明：**

| 字段名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | int | 是 | 自增 | 主键 ID |
| user_id | int | 是 | - | 创建者（管理员）用户 ID |
| key | string(char 32) | 是 | - | 兑换码，32 位字符，唯一 |
| status | int | 是 | 1 | 状态 [字典：redemption_status] |
| name | string | 是 | - | 兑换码名称/批次名 |
| quota | int | 是 | 100 | 兑换可获得的配额数量 |
| created_time | int64 | 是 | - | 创建时间（Unix 时间戳） |
| redeemed_time | int64 | 是 | - | 兑换时间（Unix 时间戳） |
| count | int | 否 | - | 批量创建数量（仅 API 请求参数，不存储到数据库） |
| used_user_id | int | 是 | - | 使用该兑换码的用户 ID |
| deleted_at | gorm.DeletedAt | 否 | null | 软删除时间 |
| expired_time | int64 | 是 | - | 过期时间（Unix 时间戳），0 表示不过期 |

**索引说明：**

| 索引名 | 类型 | 字段 | 用途 |
|--------|------|------|------|
| PRIMARY | PRIMARY KEY | id | 主键索引 |
| idx_redemptions_key | UNIQUE | key | 兑换码唯一约束 |
| idx_redemptions_name | INDEX | name | 按名称搜索 |
| idx_redemptions_deleted_at | INDEX | deleted_at | 软删除索引 |

**status 字典值（redemption_status）：**

| 值 | 说明 |
|----|------|
| 1 | 未使用 |
| 2 | 已使用（项目约定：1=启用，2=禁用） |

---

#### 1.3 users（用户表，钱包相关字段）

**表名：** `users`

**用途：** 用户主表。此处仅列出钱包页面相关的字段，完整用户模型包含更多字段。

**GORM 模型：** `model/user.go:24`

仅列出钱包页面直接使用的字段：

```go
type User struct {
    // ... 其他字段省略 ...

    // 钱包余额相关
    Quota            int    `json:"quota" gorm:"type:int;default:0"`
    UsedQuota        int    `json:"used_quota" gorm:"type:int;default:0;column:used_quota"`
    RequestCount     int    `json:"request_count" gorm:"type:int;default:0;"`

    // 推荐奖励相关
    AffCode         string `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
    AffCount        int    `json:"aff_count" gorm:"type:int;default:0;column:aff_count"`
    AffQuota        int    `json:"aff_quota" gorm:"type:int;default:0;column:aff_quota"`
    AffHistoryQuota int    `json:"aff_history_quota" gorm:"type:int;default:0;column:aff_history"`
    InviterId       int    `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`

    // Stripe 支付相关
    StripeCustomer  string `json:"stripe_customer" gorm:"type:varchar(64);column:stripe_customer;index"`

    // 用户设置（含 billing_preference）
    Setting         string `json:"setting" gorm:"type:text;column:setting"`

    // ... 其他字段省略 ...
}
```

**钱包相关字段说明：**

| 字段名 | 数据库列名 | 类型 | 必填 | 默认值 | 说明 |
|--------|-----------|------|------|--------|------|
| quota | quota | int | 是 | 0 | 当前余额（配额单位，500000 = $1） |
| used_quota | used_quota | int | 是 | 0 | 累计已使用配额 |
| request_count | request_count | int | 是 | 0 | 累计 API 请求次数 |
| aff_code | aff_code | varchar(32) | 是 | - | 推荐码，唯一 |
| aff_count | aff_count | int | 是 | 0 | 邀请注册人数 |
| aff_quota | aff_quota | int | 是 | 0 | 待入账推荐奖励配额 |
| aff_history_quota | aff_history | int | 是 | 0 | 历史累计推荐奖励配额 |
| inviter_id | inviter_id | int | 是 | - | 邀请人用户 ID（0=无邀请人） |
| stripe_customer | stripe_customer | varchar(64) | 否 | - | Stripe 客户 ID |
| setting | setting | text | 否 | - | 用户设置 JSON（含 billing_preference） |

**索引说明（钱包相关）：**

| 索引名 | 类型 | 字段 | 用途 |
|--------|------|------|------|
| idx_users_aff_code | UNIQUE | aff_code | 推荐码唯一 |
| idx_users_stripe_customer | INDEX | stripe_customer | Stripe 客户查询 |
| idx_users_inviter_id | INDEX | inviter_id | 查询邀请人 |

**setting 字段 JSON 结构（钱包页面相关）：**

```json
{
  "billing_preference": "subscription_first"
}
```

`billing_preference` 可选值：`subscription_first`（默认）、`wallet_first`、`subscription_only`、`wallet_only`。

---

### 2. 订阅相关表

#### 2.1 subscription_plans（订阅计划表）

**表名：** `subscription_plans`

**用途：** 定义可购买的订阅计划（API Plan 和 Coding Plan）。

**GORM 模型：** `model/subscription.go:159`

```go
type SubscriptionPlan struct {
    Id int `json:"id"`

    Title    string `json:"title" gorm:"type:varchar(128);not null"`
    Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`

    PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
    Currency    string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`

    DurationUnit  string `json:"duration_unit" gorm:"type:varchar(16);not null;default:'month'"`
    DurationValue int    `json:"duration_value" gorm:"type:int;not null;default:1"`
    CustomSeconds int64  `json:"custom_seconds" gorm:"type:bigint;not null;default:0"`

    Enabled   bool `json:"enabled" gorm:"default:true"`
    SortOrder int  `json:"sort_order" gorm:"type:int;default:0"`

    StripePriceId  string `json:"stripe_price_id" gorm:"type:varchar(128);default:''"`
    CreemProductId string `json:"creem_product_id" gorm:"type:varchar(128);default:''"`

    MaxPurchasePerUser int    `json:"max_purchase_per_user" gorm:"type:int;default:0"`
    UpgradeGroup      string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`

    TotalAmount              int64  `json:"total_amount" gorm:"type:bigint;not null;default:0"`
    QuotaResetPeriod         string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
    QuotaResetCustomSeconds  int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

    PlanType                  string `json:"plan_type" gorm:"type:varchar(32);not null;default:'api'"`
    RateLimitTokensPerWindow  int    `json:"rate_limit_tokens_per_window" gorm:"type:int;not null;default:0"`
    RateLimitWeeklyMultiplier int    `json:"rate_limit_weekly_multiplier" gorm:"type:int;not null;default:0"`

    CreatedAt int64 `json:"created_at" gorm:"bigint"`
    UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}
```

**字段说明：**

| 字段名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | int | 是 | 自增 | 主键 ID |
| title | varchar(128) | 是 | - | 计划标题 |
| subtitle | varchar(255) | 是 | '' | 副标题 |
| price_amount | decimal(10,6) | 是 | 0 | 显示价格 |
| currency | varchar(8) | 是 | 'USD' | 货币类型 [字典：currency] |
| duration_unit | varchar(16) | 是 | 'month' | 时长单位 [字典：duration_unit] |
| duration_value | int | 是 | 1 | 时长数值 |
| custom_seconds | bigint | 是 | 0 | 自定义时长（秒），优先于 duration_unit/value |
| enabled | bool | 是 | true | 是否启用 |
| sort_order | int | 是 | 0 | 排序权重 |
| stripe_price_id | varchar(128) | 是 | '' | Stripe 价格 ID |
| creem_product_id | varchar(128) | 是 | '' | Creem 产品 ID |
| max_purchase_per_user | int | 是 | 0 | 每用户最大购买次数（0=无限） |
| upgrade_group | varchar(64) | 是 | '' | 购买后升级到的用户组（空=不变） |
| total_amount | bigint | 是 | 0 | 总配额数量（0=无限） |
| quota_reset_period | varchar(16) | 是 | 'never' | 配额重置周期 [字典：quota_reset_period] |
| quota_reset_custom_seconds | bigint | 是 | 0 | 自定义重置周期（秒） |
| plan_type | varchar(32) | 是 | 'api' | 计划类型 [字典：plan_type] |
| rate_limit_tokens_per_window | int | 是 | 0 | 5小时窗口 token 限制（仅 coding_plan） |
| rate_limit_weekly_multiplier | int | 是 | 0 | 周倍率（仅 coding_plan） |
| created_at | bigint | 是 | - | 创建时间（Unix 时间戳） |
| updated_at | bigint | 是 | - | 更新时间（Unix 时间戳） |

**索引说明：**

| 索引名 | 类型 | 字段 | 用途 |
|--------|------|------|------|
| PRIMARY | PRIMARY KEY | id | 主键索引 |

**字典值说明：**

currency：USD、EUR 等 ISO 4217 货币代码

duration_unit：month、year、day、hour、custom

quota_reset_period：never、daily、weekly、monthly、custom

plan_type：

| 值 | 说明 |
|----|------|
| api | API 配额型计划 |
| coding_plan | Token 速率限制型计划 |

---

#### 2.2 subscription_orders（订阅订单表）

**表名：** `subscription_orders`

**用途：** 记录订阅计划的购买订单。

**GORM 模型：** `model/subscription.go:216`

```go
type SubscriptionOrder struct {
    Id     int     `json:"id"`
    UserId int     `json:"user_id" gorm:"index"`
    PlanId int     `json:"plan_id" gorm:"index"`
    Money  float64 `json:"money"`

    TradeNo         string `json:"trade_no" gorm:"unique;type:varchar(255);index"`
    PaymentMethod   string `json:"payment_method" gorm:"type:varchar(50)"`
    PaymentProvider string `json:"payment_provider" gorm:"type:varchar(50);default:''"`
    Status          string `json:"status"`
    CreateTime      int64  `json:"create_time"`
    CompleteTime    int64  `json:"complete_time"`

    ProviderPayload string `json:"provider_payload" gorm:"type:text"`
}
```

**字段说明：**

| 字段名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | int | 是 | 自增 | 主键 ID |
| user_id | int | 是 | - | 购买用户 ID |
| plan_id | int | 是 | - | 订阅计划 ID |
| money | float64 | 是 | - | 支付金额 |
| trade_no | varchar(255) | 是 | - | 交易号，唯一 [字典：-] |
| payment_method | varchar(50) | 是 | - | 支付方式名称 |
| payment_provider | varchar(50) | 是 | '' | 支付提供商标识 [字典：payment_provider] |
| status | string | 是 | - | 订单状态 [字典：topup_status] |
| create_time | bigint | 是 | - | 创建时间（Unix 时间戳） |
| complete_time | bigint | 是 | - | 完成时间（Unix 时间戳） |
| provider_payload | text | 是 | - | 支付提供商返回的原始数据 |

**索引说明：**

| 索引名 | 类型 | 字段 | 用途 |
|--------|------|------|------|
| PRIMARY | PRIMARY KEY | id | 主键索引 |
| idx_subscription_orders_user_id | INDEX | user_id | 按用户查询 |
| idx_subscription_orders_plan_id | INDEX | plan_id | 按计划查询 |
| idx_subscription_orders_trade_no | UNIQUE | trade_no | 交易号唯一 |

---

#### 2.3 user_subscriptions（用户订阅实例表）

**表名：** `user_subscriptions`

**用途：** 记录用户已购买的订阅实例，包含配额用量、有效期、快照信息。

**GORM 模型：** `model/subscription.go:255`

```go
type UserSubscription struct {
    Id     int `json:"id"`
    UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
    PlanId int `json:"plan_id" gorm:"index"`

    AmountTotal int64 `json:"amount_total" gorm:"type:bigint;not null;default:0"`
    AmountUsed  int64 `json:"amount_used" gorm:"type:bigint;not null;default:0"`

    StartTime int64  `json:"start_time" gorm:"bigint"`
    EndTime   int64  `json:"end_time" gorm:"bigint;index;index:idx_user_sub_active,priority:3"`
    Status    string `json:"status" gorm:"type:varchar(32);index;index:idx_user_sub_active,priority:2"`

    Source string `json:"source" gorm:"type:varchar(32);default:'order'"`

    LastResetTime int64 `json:"last_reset_time" gorm:"type:bigint;default:0"`
    NextResetTime int64 `json:"next_reset_time" gorm:"type:bigint;default:0;index"`

    UpgradeGroup  string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
    PrevUserGroup string `json:"prev_user_group" gorm:"type:varchar(64);default:''"`

    PlanType                  string `json:"plan_type" gorm:"type:varchar(32);not null;default:'api'"`
    RateLimitTokensPerWindow  int    `json:"rate_limit_tokens_per_window" gorm:"type:int;not null;default:0"`
    RateLimitWeeklyMultiplier int    `json:"rate_limit_weekly_multiplier" gorm:"type:int;not null;default:0"`

    CreatedAt int64 `json:"created_at" gorm:"bigint"`
    UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}
```

**字段说明：**

| 字段名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | int | 是 | 自增 | 主键 ID |
| user_id | int | 是 | - | 用户 ID |
| plan_id | int | 是 | - | 订阅计划 ID |
| amount_total | bigint | 是 | 0 | 总配额数量 |
| amount_used | bigint | 是 | 0 | 已使用配额数量 |
| start_time | bigint | 是 | - | 生效时间（Unix 时间戳） |
| end_time | bigint | 是 | - | 到期时间（Unix 时间戳） |
| status | varchar(32) | 是 | - | 状态 [字典：subscription_status] |
| source | varchar(32) | 是 | 'order' | 来源 [字典：subscription_source] |
| last_reset_time | bigint | 是 | 0 | 上次配额重置时间 |
| next_reset_time | bigint | 是 | 0 | 下次配额重置时间 |
| upgrade_group | varchar(64) | 是 | '' | 升级到的用户组 |
| prev_user_group | varchar(64) | 是 | '' | 升级前的用户组 |
| plan_type | varchar(32) | 是 | 'api' | 计划类型快照（购买时从 plan 复制） |
| rate_limit_tokens_per_window | int | 是 | 0 | 5小时窗口限制快照 |
| rate_limit_weekly_multiplier | int | 是 | 0 | 周倍率快照 |
| created_at | bigint | 是 | - | 创建时间（Unix 时间戳） |
| updated_at | bigint | 是 | - | 更新时间（Unix 时间戳） |

**索引说明：**

| 索引名 | 类型 | 字段 | 用途 |
|--------|------|------|------|
| PRIMARY | PRIMARY KEY | id | 主键索引 |
| idx_user_sub_active | INDEX | user_id, status, end_time | 查询用户活跃订阅（复合索引） |
| idx_user_subscriptions_user_id | INDEX | user_id | 按用户查询 |
| idx_user_subscriptions_plan_id | INDEX | plan_id | 按计划查询 |
| idx_user_subscriptions_end_time | INDEX | end_time | 按到期时间查询 |
| idx_user_subscriptions_status | INDEX | status | 按状态查询 |
| idx_user_subscriptions_next_reset_time | INDEX | next_reset_time | 配额重置定时任务 |

**status 字典值（subscription_status）：**

| 值 | 说明 |
|----|------|
| active | 活跃 |
| expired | 已过期 |
| cancelled | 已取消 |

**source 字典值（subscription_source）：**

| 值 | 说明 |
|----|------|
| order | 正常订单购买 |
| admin | 管理员手动绑定 |

---

#### 2.4 subscription_pre_consume_records（订阅预消费记录表）

**表名：** `subscription_pre_consume_records`

**用途：** 记录订阅配额的预消费和结算，保证幂等性。

**GORM 模型：** `model/subscription.go:944`

```go
type SubscriptionPreConsumeRecord struct {
    Id                 int    `json:"id"`
    RequestId          string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
    UserId             int    `json:"user_id" gorm:"index"`
    UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
    PreConsumed        int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
    Status             string `json:"status" gorm:"type:varchar(32);index"`
    CreatedAt          int64  `json:"created_at" gorm:"bigint"`
    UpdatedAt          int64  `json:"updated_at" gorm:"bigint;index"`
}
```

**字段说明：**

| 字段名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | int | 是 | 自增 | 主键 ID |
| request_id | varchar(64) | 是 | - | 请求唯一标识，幂等键 |
| user_id | int | 是 | - | 用户 ID |
| user_subscription_id | int | 是 | - | 用户订阅实例 ID |
| pre_consumed | bigint | 是 | 0 | 预消费配额数量 |
| status | varchar(32) | 是 | - | 状态 [字典：pre_consume_status] |
| created_at | bigint | 是 | - | 创建时间（Unix 时间戳） |
| updated_at | bigint | 是 | - | 更新时间（Unix 时间戳） |

**索引说明：**

| 索引名 | 类型 | 字段 | 用途 |
|--------|------|------|------|
| PRIMARY | PRIMARY KEY | id | 主键索引 |
| idx_request_id | UNIQUE | request_id | 请求幂等 |
| idx_user_id | INDEX | user_id | 按用户查询 |
| idx_user_subscription_id | INDEX | user_subscription_id | 按订阅实例查询 |
| idx_status | INDEX | status | 按状态查询 |
| idx_updated_at | INDEX | updated_at | 按更新时间查询 |

**status 字典值（pre_consume_status）：**

| 值 | 说明 |
|----|------|
| consumed | 已消费（结算完成） |
| refunded | 已退还 |

---

## 索引策略总结

### 高频查询场景索引

| 查询场景 | 表 | 索引 |
|---------|-----|------|
| 用户查询自己的充值记录 | topups | idx_topups_user_id |
| 按交易号查找订单 | topups | idx_topups_trade_no (UNIQUE) |
| 按订单号模糊搜索 | topups | 全表扫描（无专用索引，数据量可控） |
| 查询用户活跃订阅 | user_subscriptions | idx_user_sub_active（复合索引：user_id + status + end_time） |
| 配额重置定时任务 | user_subscriptions | idx_user_subscriptions_next_reset_time |
| 兑换码查找 | redemptions | idx_redemptions_key (UNIQUE) |
| 请求幂等检查 | subscription_pre_consume_records | idx_request_id (UNIQUE) |

---

## 性能优化

### 当前设计特点

1. **时间字段使用 int64 Unix 时间戳**：避免数据库日期函数差异，三种数据库兼容
2. **JSON 字段使用 text 类型存储**：setting 等字段统一用 text，保证 SQLite/MySQL/PostgreSQL 兼容
3. **无外键约束**：所有关联关系通过应用层维护，user_id、plan_id 等字段仅有索引，无 FK 约束
4. **软删除**：redemptions 表使用 GORM DeletedAt 实现软删除
5. **复合索引优化活跃订阅查询**：user_subscriptions 的 idx_user_sub_active 复合索引覆盖最频繁的"查询用户活跃订阅"场景

### 缓存策略

- 用户余额（quota、used_quota、request_count）：通过 GET /api/user/self 获取，前端缓存
- 充值配置（TopupInfo）：页面加载时获取，配置变更频率低
- 订阅计划列表：公开数据，通过 GET /api/subscription/plans 获取

---

## 数据迁移说明

本项目的所有表通过 GORM AutoMigrate 管理（`model/main.go` 的 `migrateDB()` 函数），不使用 SQL 迁移文件。新增表和新增字段由 AutoMigrate 在应用启动时自动处理。
