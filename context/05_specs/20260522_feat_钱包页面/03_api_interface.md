# HTTP 接口设计文档（代码逆推）

本文档从现有代码库逆推生成，所有接口路径、请求参数、响应结构均与实际代码保持一致。

---

## 基础信息

**模块前缀：** wallet（钱包充值相关）

**接口协议：** HTTP（简化 GET/POST 模式）

**Base URL：** `/api`

**统一响应结构：**

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

部分支付接口使用历史格式（无 `success` 字段）：

```json
{
  "message": "success",
  "data": {}
}
```

**分页参数：**

| 参数 | 别名 | 说明 | 默认值 | 最大值 |
|------|------|------|--------|--------|
| `p` | `page` | 页码（从 1 开始） | 1 | - |
| `page_size` | `ps`, `size` | 每页条数 | 10 | 100 |

**分页响应：**

```json
{
  "page": 1,
  "page_size": 10,
  "total": 100,
  "items": []
}
```

**认证方式：**

| 中间件 | 说明 |
|--------|------|
| `middleware.UserAuth()` | 普通用户，Session 或 Token 认证 |
| `middleware.AdminAuth()` | 管理员（role >= 10） |
| 无 | Webhook 回调，通过签名验证 |

---

## 接口列表

### 1. 充值配置查询

#### 1.1 GET /api/user/topup/info

**需求追溯：** 页面1-钱包页面 / 页面初始化加载

**认证：** UserAuth

**请求参数：** 无

**响应示例：**

```json
{
  "enable_online_topup": true,
  "enable_stripe_topup": true,
  "enable_creem_topup": true,
  "enable_waffo_topup": true,
  "enable_waffo_pancake_topup": true,
  "enable_redemption": true,
  "payment_compliance_confirmed": true,
  "payment_compliance_terms_version": "1.0",
  "waffo_pay_methods": [
    {
      "name": "Credit Card",
      "type": "credit_card"
    }
  ],
  "creem_products": "[{\"id\":\"prod_xxx\",\"name\":\"Basic\",\"price\":10.00,\"currency\":\"USD\",\"quota\":5000000}]",
  "pay_methods": [
    {
      "name": "Alipay",
      "type": "alipay",
      "color": "#1677ff",
      "min_topup": "1"
    }
  ],
  "min_topup": 1,
  "stripe_min_topup": 1,
  "waffo_min_topup": 1,
  "waffo_pancake_min_topup": 1,
  "amount_options": [1, 5, 10, 20, 50, 100],
  "discount": {
    "10": 0.95,
    "50": 0.9,
    "100": 0.85
  },
  "topup_link": "https://example.com/buy-code"
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| enable_online_topup | bool | 是否启用易支付在线充值 |
| enable_stripe_topup | bool | 是否启用 Stripe 信用卡充值 |
| enable_creem_topup | bool | 是否启用 Creem 加密货币充值 |
| enable_waffo_topup | bool | 是否启用 Waffo 充值 |
| enable_waffo_pancake_topup | bool | 是否启用 Waffo Pancake 充值（注意：当前路由已禁用，此开关生效但支付接口不可用） |
| enable_redemption | bool | 是否启用兑换码功能 |
| payment_compliance_confirmed | bool | 是否已完成支付合规确认 |
| payment_compliance_terms_version | string | 合规条款版本 |
| waffo_pay_methods | array/null | Waffo 渠道的子支付方式列表 |
| creem_products | string | Creem 产品配置 JSON 字符串 |
| pay_methods | array | 标准支付方式列表，每项含 name/type/color/min_topup |
| min_topup | int | 易支付最小充值金额 |
| stripe_min_topup | int | Stripe 最小充值金额 |
| waffo_min_topup | int | Waffo 最小充值金额 |
| waffo_pancake_min_topup | int | Waffo Pancake 最小充值金额 |
| amount_options | array[int] | 预设金额选项列表 |
| discount | map[int]float64 | 折扣配置，key=充值金额，value=折扣率（0~1） |
| topup_link | string | 兑换码购买链接 |

---

### 2. 兑换码充值

#### 2.1 POST /api/user/topup

**需求追溯：** 页面1-钱包页面 / 兑换码充值

**认证：** UserAuth + CriticalRateLimit

**限流：** CriticalRateLimit

**合规检查：** 需要 payment_compliance_confirmed = true

**请求参数：**

```json
{
  "key": "ABCD1234EFGH5678"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | 兑换码 |

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": 5000000
}
```

`data` 为兑换成功后增加的配额数量（int）。

---

### 3. 易支付充值

#### 3.1 POST /api/user/amount

**需求追溯：** 页面1-钱包页面 / 规则4-支付方式选择与金额计算联动

**认证：** UserAuth

**请求参数：**

```json
{
  "amount": 10
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| amount | int64 | 是 | 充值金额（美元） |

**响应示例：**

```json
{
  "message": "success",
  "data": "¥72.00"
}
```

`data` 为格式化后的实付金额字符串。

#### 3.2 POST /api/user/pay

**需求追溯：** 页面2-支付确认弹窗 / 确认支付

**认证：** UserAuth + CriticalRateLimit

**限流：** CriticalRateLimit

**请求参数：**

```json
{
  "amount": 10,
  "payment_method": "alipay"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| amount | int64 | 是 | 充值金额（美元） |
| payment_method | string | 是 | 支付方式（如 alipay、wxpay） |

**响应示例：**

```json
{
  "message": "success",
  "data": {
    "pid": "1001",
    "type": "alipay",
    "out_trade_no": "T20260522001",
    "notify_url": "https://example.com/api/user/epay/notify",
    "return_url": "https://example.com/wallet?show_history=true",
    "name": "TopUp",
    "money": "72.00",
    "sign": "abc123",
    "sign_type": "MD5"
  },
  "url": "https://pay.example.com/submit"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| data | object | 易支付表单参数，需以 form 表单提交到 url |
| url | string | 易支付支付页面 URL |

---

### 4. Stripe 充值

#### 4.1 POST /api/user/stripe/amount

**需求追溯：** 页面1-钱包页面 / 规则4-支付方式选择与金额计算联动

**认证：** UserAuth

**请求参数：**

```json
{
  "amount": 10,
  "payment_method": "stripe",
  "success_url": "https://example.com/wallet?show_history=true",
  "cancel_url": "https://example.com/wallet"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| amount | int64 | 是 | 充值金额（美元） |
| payment_method | string | 是 | 支付方式标识 |
| success_url | string | 否 | 支付成功回调 URL |
| cancel_url | string | 否 | 支付取消回调 URL |

**响应示例：**

```json
{
  "message": "success",
  "data": "$10.00"
}
```

#### 4.2 POST /api/user/stripe/pay

**需求追溯：** 页面2-支付确认弹窗 / 确认支付

**认证：** UserAuth + CriticalRateLimit

**限流：** CriticalRateLimit

**请求参数：** 同 4.1 StripeAmount

**响应示例：**

```json
{
  "message": "success",
  "data": {
    "pay_link": "https://checkout.stripe.com/c/pay/cs_xxx"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| data.pay_link | string | Stripe Checkout 页面 URL |

---

### 5. Creem 充值

#### 5.1 POST /api/user/creem/pay

**需求追溯：** 页面3-Creem产品确认弹窗 / 确认支付

**认证：** UserAuth + CriticalRateLimit

**限流：** CriticalRateLimit

**请求参数：**

```json
{
  "product_id": "prod_basic_10",
  "payment_method": "creem"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| product_id | string | 是 | Creem 产品 ID |
| payment_method | string | 是 | 支付方式标识 |

**响应示例：**

```json
{
  "message": "success",
  "data": {
    "checkout_url": "https://payment.creem.io/checkout/xxx",
    "order_id": "creem_order_abc123"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| data.checkout_url | string | Creem 结账页面 URL |
| data.order_id | string | 订单参考号 |

---

### 6. Waffo 充值

#### 6.1 POST /api/user/waffo/amount

**需求追溯：** 页面1-钱包页面 / 规则4-支付方式选择与金额计算联动

**认证：** UserAuth

**请求参数：**

```json
{
  "amount": 10,
  "pay_method_index": 0,
  "pay_method_type": "credit_card",
  "pay_method_name": "Credit Card"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| amount | int64 | 是 | 充值金额（美元） |
| pay_method_index | *int | 否 | Waffo 子支付方式索引（推荐） |
| pay_method_type | string | 否 | Waffo 子支付方式类型（已废弃） |
| pay_method_name | string | 否 | Waffo 子支付方式名称（已废弃） |

**响应示例：**

```json
{
  "message": "success",
  "data": "$10.00"
}
```

#### 6.2 POST /api/user/waffo/pay

**需求追溯：** 页面1-钱包页面 / 选择Waffo支付方式并发起支付

**认证：** UserAuth + CriticalRateLimit

**限流：** CriticalRateLimit

**请求参数：** 同 6.1 WaffoAmount

**响应示例：**

```json
{
  "message": "success",
  "data": {
    "payment_url": "https://pay.waffo.io/checkout/xxx",
    "order_id": "waffo_order_abc123"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| data.payment_url | string | Waffo 支付页面 URL |
| data.order_id | string | 订单参考号 |

---

### 7. 推荐奖励

#### 7.1 GET /api/user/aff

**需求追溯：** 页面1-钱包页面 / 推荐奖励区域

**认证：** UserAuth

**请求参数：** 无

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": "abc123def"
}
```

`data` 为用户的推荐码（aff_code）字符串。前端根据 aff_code 拼接推荐链接。

#### 7.2 POST /api/user/aff_transfer

**需求追溯：** 页面5-额度转赠弹窗 / 确认转赠

**认证：** UserAuth + 合规确认检查

**请求参数：**

```json
{
  "quota": 500000
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| quota | int | 是 | 转赠配额数量，最小为 500000（QUOTA_PER_DOLLAR），步长为 500000 |

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": null
}
```

---

### 8. 账单历史

#### 8.1 GET /api/user/topup/self（普通用户）

**需求追溯：** 页面4-账单历史弹窗

**认证：** UserAuth

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| p | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页条数，默认 10，最大 100 |
| keyword | string | 否 | 按订单号模糊搜索 |

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": {
    "page": 1,
    "page_size": 10,
    "total": 50,
    "items": [
      {
        "id": 1,
        "user_id": 123,
        "amount": 5000000,
        "money": 10.00,
        "trade_no": "T20260522123456",
        "payment_method": "alipay",
        "payment_provider": "epay",
        "create_time": 1719360000,
        "complete_time": 1719360060,
        "status": "success"
      }
    ]
  }
}
```

**TopUp 字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | 订单 ID |
| user_id | int | 用户 ID |
| amount | int64 | 充值配额数量（500000 = $1） |
| money | float64 | 实付金额 |
| trade_no | string | 交易号 |
| payment_method | string | 支付方式名称 |
| payment_provider | string | 支付提供商标识（epay/stripe/creem/waffo/waffo_pancake） |
| create_time | int64 | 创建时间（Unix 时间戳） |
| complete_time | int64 | 完成时间（Unix 时间戳） |
| status | string | 订单状态：pending/success/failed/expired |

---

### 9. 管理员接口

#### 9.1 GET /api/user/topup（管理员）

**需求追溯：** 页面4-账单历史弹窗 / 规则2-数据范围权限控制

**认证：** AdminAuth

**查询参数：** 同 8.1（含 keyword 搜索）

**响应结构：** 同 8.1，但返回所有用户的记录。

#### 9.2 POST /api/user/topup/complete（管理员补单）

**需求追溯：** 页面4-账单历史弹窗 / 管理员补单

**认证：** AdminAuth

**请求参数：**

```json
{
  "trade_no": "T20260522123456"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| trade_no | string | 是 | 待补单的交易号 |

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": null
}
```

补单操作幂等：已成功的订单直接返回成功，不重复增加配额。

---

### 10. 订阅计划（钱包页面展示）

#### 10.1 GET /api/subscription/plans

**需求追溯：** 页面1-钱包页面 / 订阅计划区域 / 可用计划列表

**认证：** UserAuth

**请求参数：** 无

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "plan": {
        "id": 1,
        "title": "Pro Plan",
        "subtitle": "Monthly subscription",
        "price_amount": 29.99,
        "currency": "USD",
        "duration_unit": "month",
        "duration_value": 1,
        "custom_seconds": 0,
        "enabled": true,
        "sort_order": 100,
        "stripe_price_id": "price_xxx",
        "creem_product_id": "prod_xxx",
        "max_purchase_per_user": 0,
        "upgrade_group": "",
        "total_amount": 50000000,
        "quota_reset_period": "monthly",
        "quota_reset_custom_seconds": 0,
        "plan_type": "api",
        "rate_limit_tokens_per_window": 0,
        "rate_limit_weekly_multiplier": 0,
        "created_at": 1719360000,
        "updated_at": 1719360000
      }
    }
  ]
}
```

#### 10.2 GET /api/subscription/self

**需求追溯：** 页面1-钱包页面 / 订阅计划区域 / 当前订阅状态

**认证：** UserAuth

**请求参数：** 无

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": {
    "billing_preference": "subscription_first",
    "subscriptions": [
      {
        "subscription": {
          "id": 1,
          "user_id": 123,
          "plan_id": 1,
          "amount_total": 50000000,
          "amount_used": 5000000,
          "start_time": 1719360000,
          "end_time": 1722038400,
          "status": "active",
          "source": "order",
          "last_reset_time": 1719360000,
          "next_reset_time": 1722038400,
          "upgrade_group": "",
          "prev_user_group": "default",
          "plan_type": "api",
          "rate_limit_tokens_per_window": 0,
          "rate_limit_weekly_multiplier": 0,
          "created_at": 1719360000,
          "updated_at": 1719360000
        }
      }
    ],
    "all_subscriptions": []
  }
}
```

`billing_preference` 可选值：`subscription_first`、`wallet_first`、`subscription_only`、`wallet_only`。

#### 10.3 PUT /api/subscription/self/preference

**需求追溯：** 页面1-钱包页面 / 切换计费偏好

**认证：** UserAuth

**请求参数：**

```json
{
  "billing_preference": "subscription_first"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| billing_preference | string | 是 | 计费偏好：subscription_first/wallet_first/subscription_only/wallet_only |

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": {
    "billing_preference": "subscription_first"
  }
}
```

#### 10.4 GET /api/subscription/rate-limits

**需求追溯：** 页面1-钱包页面 / 订阅计划区域 / Coding Plan 速率限制

**认证：** UserAuth

**请求参数：** 无

**响应示例：**

```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "subscription_id": 1,
      "plan_title": "Coding Pro",
      "plan_type": "coding_plan",
      "window_5h": {
        "limit": 100000,
        "used": 5000,
        "remaining": 95000,
        "reset_at": 1719378000
      },
      "window_week": {
        "limit": 500000,
        "used": 25000,
        "remaining": 475000,
        "reset_at": 1719960000
      }
    }
  ]
}
```

仅 `coding_plan` 类型的订阅返回速率限制数据。

#### 10.5 POST /api/subscription/stripe/pay

**需求追溯：** 页面1-钱包页面 / 购买订阅计划

**认证：** UserAuth + CriticalRateLimit

**限流：** CriticalRateLimit

**请求参数：**

```json
{
  "plan_id": 1
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| plan_id | int | 是 | 订阅计划 ID |

**响应示例：**

```json
{
  "message": "success",
  "data": {
    "pay_link": "https://checkout.stripe.com/c/pay/cs_xxx"
  }
}
```

#### 10.6 POST /api/subscription/creem/pay

**需求追溯：** 页面1-钱包页面 / 购买订阅计划

**认证：** UserAuth + CriticalRateLimit

**限流：** CriticalRateLimit

**请求参数：**

```json
{
  "plan_id": 1
}
```

**响应示例：**

```json
{
  "message": "success",
  "data": {
    "checkout_url": "https://payment.creem.io/checkout/xxx",
    "order_id": "sub_ref_abc123"
  }
}
```

#### 10.7 POST /api/subscription/epay/pay

**需求追溯：** 页面1-钱包页面 / 购买订阅计划

**认证：** UserAuth + CriticalRateLimit

**限流：** CriticalRateLimit

**请求参数：**

```json
{
  "plan_id": 1,
  "payment_method": "alipay"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| plan_id | int | 是 | 订阅计划 ID |
| payment_method | string | 是 | 支付方式（如 alipay、wxpay） |

**响应示例：**

```json
{
  "message": "success",
  "data": {
    "pid": "1001",
    "type": "alipay",
    "out_trade_no": "S20260522001",
    "notify_url": "https://example.com/api/subscription/epay/notify",
    "return_url": "https://example.com/wallet?show_history=true",
    "name": "Subscription",
    "money": "29.99",
    "sign": "abc123",
    "sign_type": "MD5"
  },
  "url": "https://pay.example.com/submit"
}
```

响应结构与易支付充值接口相同，`data` 为易支付表单参数，`url` 为支付页面 URL。

---

### 11. Webhook 回调接口（无认证）

#### 11.1 POST /api/stripe/webhook

**需求追溯：** 5.1 支付回调 Webhook 处理

**认证：** 无（通过 Stripe-Signature 请求头验证）

**请求：** Stripe Event Payload（JSON）

**响应：** HTTP 200

处理逻辑：验证签名 -> 查找订单 -> 更新状态 -> 增加配额。

#### 11.2 POST /api/creem/webhook

**需求追溯：** 5.1 支付回调 Webhook 处理

**认证：** 无（通过 creem-signature 请求头验证）

**请求：** Creem Event Payload（JSON）

**响应：** HTTP 200

#### 11.3 POST /api/waffo/webhook

**需求追溯：** 5.1 支付回调 Webhook 处理

**认证：** 无（通过 X-SIGNATURE 请求头验证）

**请求：** Waffo Event Payload（JSON）

**响应：** 签名后的 JSON 响应

#### 11.4 POST/GET /api/user/epay/notify

**需求追溯：** 5.1 支付回调 Webhook 处理

**认证：** 无（通过签名参数验证）

**请求：** 表单数据或 Query String

**响应：** 纯文本 "success" 或 "fail"

#### 11.5 POST/GET /api/subscription/epay/notify

**需求追溯：** 5.1 支付回调 Webhook 处理（订阅订单）

**认证：** 无

**响应：** 纯文本 "success" 或 "fail"

#### 11.6 GET/POST /api/subscription/epay/return

**需求追溯：** 用户从 Epay 支付页面返回

**认证：** 无

**响应：** 重定向到钱包页面

---

## 错误码规范

本项目的 API 接口通过统一的响应格式返回错误：

```json
{
  "success": false,
  "message": "具体错误描述（通过 i18n 返回）"
}
```

### 钱包页面相关错误场景

| 场景 | 错误消息（i18n key） | 说明 |
|------|---------------------|------|
| 兑换码无效或已使用 | Redemption failed | 兑换码状态不正确或已过期 |
| 合规未确认 | Redemption codes are disabled until... | payment_compliance_confirmed 为 false |
| 支付请求失败 | Payment request failed | 第三方支付接口调用失败 |
| 管理员补单失败 | Failed to complete order | 订单状态不允许补单 |
| 金额低于最小值 | Amount is too small | 低于对应支付渠道最小充值金额 |
| 推荐奖励转赠失败 | - | 配额不足或合规未确认 |

---

## 安全说明

### 认证要求

所有 `/api/user/self/*` 和 `/api/user/topup/*`（用户级）接口需要 UserAuth 认证。
所有 `/api/user/topup`（管理员级）和补单接口需要 AdminAuth 认证。
Webhook 回调接口无 HTTP 认证，通过各支付平台的签名机制验证请求合法性。

### 限流策略

| 接口 | 限流级别 |
|------|---------|
| POST /api/user/topup（兑换码） | CriticalRateLimit |
| POST /api/user/pay（易支付） | CriticalRateLimit |
| POST /api/user/stripe/pay | CriticalRateLimit |
| POST /api/user/creem/pay | CriticalRateLimit |
| POST /api/user/waffo/pay | CriticalRateLimit |
| POST /api/subscription/*/pay | CriticalRateLimit |

### 合规控制

- POST /api/user/topup（兑换码）和 POST /api/user/aff_transfer（转赠）均受 `payment_compliance_confirmed` 开关控制
- 开关为 false 时，兑换码充值和奖励转赠操作被拒绝
