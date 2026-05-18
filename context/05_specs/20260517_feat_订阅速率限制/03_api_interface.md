# 订阅速率限制 - HTTP 接口设计

## 基础信息

**模块前缀：** sub_rate_limit（扩展现有 subscription 模块）

**接口协议：** HTTP

**Base URL：** `/api`

**核心设计原则：**

本功能是现有订阅系统的扩展，接口设计遵循项目已有的路由和响应规范：

- `/api/*` 管理接口使用统一响应结构 `{"success": true/false, "message": "", "data": {}}`
- `/v1/*` relay 接口遵循 OpenAI 兼容的错误响应格式

**接口方法约定：**

| 方法 | 用途 | 说明 |
|------|------|------|
| GET | 查询数据 | 现有接口，无变更 |
| POST | 创建资源 | 现有管理员创建套餐接口 |
| PUT | 更新资源 | 现有管理员更新套餐接口 |

---

## 一、新增接口

### 1.1 查询限速状态

**接口路径：** `GET /api/subscription/rate-limits`

**需求追溯：** [需求：5.5 限速状态查询 API]、[需求：4.2 用户钱包页（限速状态展示）]

**认证方式：** `middleware.UserAuth()`（普通用户，role >= 1）

**功能说明：** 返回当前用户所有活跃 CodingPlan 订阅的限速使用情况，供钱包页展示和客户端工具使用。

**请求参数：** 无

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "subscription_id": 123,
      "plan_title": "Coding Plan Pro",
      "plan_type": "coding_plan",
      "window_5h": {
        "limit": 500000,
        "used": 125000,
        "remaining": 375000,
        "reset_at": 1747461600
      },
      "window_week": {
        "limit": 10000000,
        "used": 2500000,
        "remaining": 7500000,
        "reset_at": 1747593600
      }
    }
  ]
}
```

**响应字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| subscription_id | int | 用户订阅 ID |
| plan_title | string | 套餐名称（来自关联的 SubscriptionPlan） |
| plan_type | string | 套餐类型快照，固定为 "coding_plan" |
| window_5h | object | 5 小时窗口限速状态 |
| window_5h.limit | int | 5 小时窗口 token 上限 |
| window_5h.used | int | 当前 5 小时窗口已使用 token 数 |
| window_5h.remaining | int | 当前 5 小时窗口剩余 token 数 |
| window_5h.reset_at | int64 | 当前 5 小时窗口重置时间（Unix 时间戳） |
| window_week | object | 周窗口限速状态 |
| window_week.limit | int | 周窗口 token 上限（5h limit × 周倍数） |
| window_week.used | int | 当前周窗口已使用 token 数 |
| window_week.remaining | int | 当前周窗口剩余 token 数 |
| window_week.reset_at | int64 | 当前周窗口重置时间（Unix 时间戳，下周一 UTC 00:00） |

**异常场景：**

| 场景 | 响应 | 说明 |
|------|------|------|
| Redis 不可用 | `data: []` | 返回空数组，记录告警日志 |
| 用户无 CodingPlan 订阅 | `data: []` | 返回空数组 |
| 用户未登录 | HTTP 401 | 认证中间件拦截 |

---

## 二、修改的现有接口

### 2.1 获取套餐列表（用户侧）

**接口路径：** `GET /api/subscription/plans`

**需求追溯：** [需求：4.3 CodingPlan 套餐卡片（限速参数展示）]、[需求：4.4 购买确认弹窗（限速参数展示）]

**认证方式：** `middleware.UserAuth()`（现有，无变更）

**变更内容：** 响应中每个套餐对象新增以下字段：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| plan_type | string | 套餐类型：`"api"` / `"coding_plan"`，默认 `"api"` |
| rate_limit_tokens_per_window | int | 5 小时 token 上限。CodingPlan 时 ≥ 1，API 套餐时为 0 |
| rate_limit_weekly_multiplier | int | 周倍数。CodingPlan 时 ≥ 1，API 套餐时为 0 |

**响应示例（变更部分）：**

```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "id": 1,
      "title": "Coding Plan Pro",
      "plan_type": "coding_plan",
      "rate_limit_tokens_per_window": 500000,
      "rate_limit_weekly_multiplier": 20,
      "price_amount": 20.0,
      "currency": "USD",
      "duration_unit": "month",
      "duration_value": 1,
      "total_amount": 50000000,
      "enabled": true
    },
    {
      "id": 2,
      "title": "API Standard",
      "plan_type": "api",
      "rate_limit_tokens_per_window": 0,
      "rate_limit_weekly_multiplier": 0,
      "price_amount": 10.0,
      "currency": "USD",
      "duration_unit": "month",
      "duration_value": 1,
      "total_amount": 10000000,
      "enabled": true
    }
  ]
}
```

**说明：** 前端根据 `plan_type` 判断是否在卡片和购买弹窗上展示限速参数区域。

---

### 2.2 创建套餐（管理员）

**接口路径：** `POST /api/subscription/admin/plans`

**需求追溯：** [需求：4.1 套餐编辑抽屉（限速配置区域）]

**认证方式：** `middleware.AdminAuth()`（现有，无变更）

**变更内容：** 请求体新增以下字段：

| 字段名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| plan_type | string | 否 | `"api"` | 套餐类型 [字典：sub_plan_type] |
| rate_limit_tokens_per_window | int | 条件必填 | 0 | plan_type 为 coding_plan 时必填，≥ 1 [长度来源：需求规格说明书] |
| rate_limit_weekly_multiplier | int | 条件必填 | 0 | plan_type 为 coding_plan 时必填，≥ 1 [长度来源：需求规格说明书] |

**请求示例（变更部分）：**

```json
{
  "title": "Coding Plan Pro",
  "plan_type": "coding_plan",
  "rate_limit_tokens_per_window": 500000,
  "rate_limit_weekly_multiplier": 20,
  "price_amount": 20.0,
  "currency": "USD",
  "duration_unit": "month",
  "duration_value": 1,
  "total_amount": 50000000,
  "enabled": true
}
```

**校验规则：**

- `plan_type` 为 `"coding_plan"` 时，`rate_limit_tokens_per_window` 和 `rate_limit_weekly_multiplier` 必填且 ≥ 1
- `plan_type` 为 `"api"` 时，忽略 `rate_limit_tokens_per_window` 和 `rate_limit_weekly_multiplier`，存储为 0
- 校验失败返回 `{"success": false, "message": "..."}`

**错误响应示例：**

```json
{
  "success": false,
  "message": "rate_limit_tokens_per_window is required for coding_plan and must be >= 1"
}
```

---

### 2.3 更新套餐（管理员）

**接口路径：** `PUT /api/subscription/admin/plans/:id`

**需求追溯：** [需求：4.1 套餐编辑抽屉（限速配置区域）]

**认证方式：** `middleware.AdminAuth()`（现有，无变更）

**变更内容：** 与创建套餐相同，请求体新增 `plan_type`、`rate_limit_tokens_per_window`、`rate_limit_weekly_multiplier` 字段。

**路径参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| id | int | 套餐 ID |

**业务说明：** 修改套餐限速参数仅影响后续新购买的订阅。已有订阅保留购买时的限速参数快照（存储在 UserSubscription 中）。

---

### 2.4 获取用户订阅（用户侧/管理员）

**涉及接口：**
- `GET /api/subscription/self`（用户侧）
- `GET /api/subscription/admin/users/:id/subscriptions`（管理员侧）

**需求追溯：** [需求：4.2 用户钱包页（限速状态展示）]

**变更内容：** 响应中每个订阅对象新增以下字段：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| plan_type | string | 套餐类型快照：`"api"` / `"coding_plan"` |
| rate_limit_tokens_per_window | int | 5 小时 token 上限快照 |
| rate_limit_weekly_multiplier | int | 周倍数快照 |

**说明：** 这三个字段是订阅创建时从套餐快照的值。前端根据 `plan_type` 判断是否展示限速状态区域，并调用 `GET /api/subscription/rate-limits` 获取实时用量数据。

---

## 三、Relay 层接口行为（非独立 API，内部链路逻辑）

### 3.1 限速检查

**触发时机：** 用户通过 `/v1/*` 发起 AI API 请求时，在预消费（PreConsume）之前执行

**需求追溯：** [需求：5.1 限速检查]

**执行顺序：**

```
TokenAuth（认证）→ RPM 限速（现有 ModelRequestRateLimit）→ TPM 限速（本功能）→ 预消费 → 上游请求
```

**处理逻辑：**

1. 识别请求计费方式是否为订阅计费
2. 查询用户活跃订阅中 PlanType 为 `"coding_plan"` 的订阅
3. 对每个 CodingPlan 订阅，查询 Redis 中 5 小时窗口和周窗口的已用 token
4. 判断是否超限（5h 已用 ≥ 5h 上限，或周已用 ≥ 周上限）
5. 任一窗口超限即返回 429

**429 响应格式（OpenAI 兼容）：**

```json
{
  "error": {
    "type": "rate_limit_exceeded",
    "message": "Rate limit exceeded for the current window",
    "rate_limit": {
      "limit_type": "5h",
      "limit": 500000,
      "remaining": 0,
      "reset_at": 1747461600
    }
  }
}
```

**429 响应头：**

```
HTTP/1.1 429 Too Many Requests
Retry-After: 3600
```

**降级策略：**

| 异常场景 | 处理方式 |
|---------|---------|
| Redis 不可用 | 降级放行请求，记录告警日志 |
| 限速配置异常（上限 ≤ 0） | 降级放行请求，记录错误日志 |
| 非 CodingPlan 订阅 | 跳过限速检查 |
| 非订阅计费模式 | 跳过限速检查 |

---

### 3.2 限速响应头注入

**触发时机：** 每次正常 API 响应返回时（包括流式和非流式）

**需求追溯：** [需求：5.3 限速响应头注入]

**响应头字段：**

| 响应头 | 类型 | 说明 |
|--------|------|------|
| X-RateLimit-Limit-5h | int | 5 小时窗口 token 上限 |
| X-RateLimit-Remaining-5h | int | 5 小时窗口剩余 token |
| X-RateLimit-Reset-5h | int64 | 5 小时窗口重置时间（Unix 时间戳） |
| X-RateLimit-Limit-Week | int | 周窗口 token 上限 |
| X-RateLimit-Remaining-Week | int | 周窗口剩余 token |
| X-RateLimit-Reset-Week | int64 | 周窗口重置时间（Unix 时间戳） |

**说明：**
- 仅 CodingPlan 套餐的活跃订阅才注入这些响应头
- Remaining 值不反映当前进行中的请求消耗量，仅反映已完成请求的累计消耗
- Redis 不可用时跳过响应头注入，不影响正常请求响应

---

### 3.3 限速用量记录

**触发时机：** API 请求完成后（流式响应结束或非流式响应返回后）

**需求追溯：** [需求：5.2 限速用量记录]

**处理逻辑：**

1. 获取实际 token 消耗量（输入 + 输出 token）
2. 计算当前 5 小时窗口 key 和周窗口 key
3. 向 Redis Sorted Set 中添加记录
4. 设置 TTL（5h 窗口 = 6 小时，周窗口 = 8 天）

**说明：**
- 仅记录 CodingPlan 订阅的用量
- 流式请求在全部输出完成后一次性记录
- Redis 写入失败不影响已完成请求，记录错误日志

---

### 3.4 限速数据清理

**触发时机：** 订阅状态变更为 expired 或 cancelled 时

**需求追溯：** [需求：5.4 限速数据清理]

**处理逻辑：**

1. 订阅过期/取消时，查询 Redis 中该订阅所有限速相关的 Sorted Set key
2. 批量删除这些 key
3. Redis 清理失败不阻塞过期流程，依赖 TTL 兜底（6 小时 / 8 天后自动过期）

---

## 四、错误码规范

### Relay 限速错误

| 错误类型 | HTTP 状态码 | 说明 |
|---------|-----------|------|
| rate_limit_exceeded | 429 | Token 速率限制超限，响应体包含限速详情 |

### 管理接口错误

| 场景 | 错误信息 |
|------|---------|
| CodingPlan 缺少限速参数 | plan_type 为 coding_plan 时必须填写限速参数 |
| 限速参数值无效 | rate_limit_tokens_per_window 和 rate_limit_weekly_multiplier 必须 ≥ 1 |

---

## 五、安全说明

### 认证与授权

| 接口 | 认证方式 | 授权 |
|------|---------|------|
| GET /api/subscription/rate-limits | UserAuth() | 用户仅能查询自己的限速状态 |
| POST /api/subscription/admin/plans | AdminAuth() | 管理员（role >= 10） |
| PUT /api/subscription/admin/plans/:id | AdminAuth() | 管理员（role >= 10） |
| /v1/* relay 限速检查 | TokenAuth() | Token 认证，限速检查基于用户身份 |

### 数据隔离

- 用户限速状态查询通过 `user_id` 过滤，确保行级数据隔离
- Redis key 以 `subscription_id` 为维度，不同订阅的计数器互不干扰

### 输入验证

- `plan_type` 仅允许 `"api"` 和 `"coding_plan"` 两个值
- `rate_limit_tokens_per_window` 和 `rate_limit_weekly_multiplier` 必须为正整数
- 所有管理接口输入通过后端校验，不依赖前端校验

---

**文档版本：** v1.0

**最后更新：** 2026-05-18
