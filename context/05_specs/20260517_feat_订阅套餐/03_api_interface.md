# HTTP 接口设计文档 — 订阅套餐

## 基础信息

**模块前缀：** `sub`

**接口协议：** HTTP (GET/PUT/POST/PATCH/DELETE)

**Base URL：** `/api`

**认证方式：** Bearer Token（JWT）

**统一响应格式：**

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

- `success` 为 `true` 时表示请求成功，`data` 为业务数据
- `success` 为 `false` 时表示请求失败，`message` 为错误描述
- HTTP 状态码始终为 200，业务状态通过 `success` 字段判断

---

## 接口列表

### 1. 用户端接口

用户端接口统一使用 `middleware.UserAuth()` 中间件，要求用户已登录。

#### 1.1 获取可用套餐列表

**接口路径：** `GET /api/subscription/plans`

**需求追溯：** [需求：4.5 页面 5 - 浏览套餐]

**认证要求：** UserAuth（已登录用户）

**查询参数：** 无

**请求示例：**
```
GET /api/subscription/plans
Authorization: Bearer <token>
```

**响应示例：**
```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "plan": {
        "id": 1,
        "title": "基础套餐",
        "subtitle": "适合个人用户",
        "price_amount": 9.99,
        "currency": "USD",
        "duration_unit": "month",
        "duration_value": 1,
        "custom_seconds": 0,
        "enabled": true,
        "sort_order": 10,
        "stripe_price_id": "price_xxx",
        "creem_product_id": "prod_xxx",
        "max_purchase_per_user": 0,
        "upgrade_group": "vip",
        "total_amount": 5000000,
        "quota_reset_period": "monthly",
        "quota_reset_custom_seconds": 0,
        "created_at": 1716240000,
        "updated_at": 1716240000
      }
    }
  ]
}
```

**业务规则：**
- 仅返回 `enabled = true` 的套餐
- 未确认支付合规时返回空数组
- 按 `sort_order desc, id desc` 排序

---

#### 1.2 获取用户订阅信息

**接口路径：** `GET /api/subscription/self`

**需求追溯：** [需求：4.5 页面 5 - 当前订阅状态展示]

**认证要求：** UserAuth（已登录用户）

**查询参数：** 无

**请求示例：**
```
GET /api/subscription/self
Authorization: Bearer <token>
```

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
          "user_id": 100,
          "plan_id": 1,
          "amount_total": 5000000,
          "amount_used": 1200000,
          "start_time": 1716240000,
          "end_time": 1718832000,
          "status": "active",
          "source": "order",
          "last_reset_time": 0,
          "next_reset_time": 1718832000,
          "upgrade_group": "vip",
          "prev_user_group": "default",
          "created_at": 1716240000,
          "updated_at": 1716240000
        }
      }
    ],
    "all_subscriptions": [
      {
        "subscription": { "..." : "..." }
      }
    ]
  }
}
```

**业务规则：**
- `subscriptions`：当前所有活跃订阅（status = active 且 end_time > now）
- `all_subscriptions`：全部订阅记录（含 expired、cancelled）
- `billing_preference`：用户计费偏好，值为 `subscription_first` / `wallet_first` / `subscription_only` / `wallet_only`

---

#### 1.3 更新计费偏好

**接口路径：** `PUT /api/subscription/self/preference`

**需求追溯：** [需求：4.5 页面 5 - 切换计费偏好]

**认证要求：** UserAuth（已登录用户）

**请求参数：**
```json
{
  "billing_preference": "subscription_first"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| billing_preference | string | 是 | 计费偏好，可选值：`subscription_first`、`wallet_first`、`subscription_only`、`wallet_only` [字典：sub_billing_preference] |

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

**业务规则：**
- 无效值会被归一化为 `subscription_first`
- 即时生效，下次 API 请求按新偏好扣费

---

#### 1.4 Stripe 支付

**接口路径：** `POST /api/subscription/stripe/pay`

**需求追溯：** [需求：4.6 页面 6 - 确认支付 / 5.4 支付回调处理]

**认证要求：** UserAuth（已登录用户）

**限流：** CriticalRateLimit

**请求参数：**
```json
{
  "plan_id": 1
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| plan_id | int | 是 | 要购买的套餐 ID |

**响应示例（成功）：**
```json
{
  "message": "success",
  "data": {
    "pay_link": "https://checkout.stripe.com/c/pay/cs_xxx"
  }
}
```

**响应示例（失败）：**
```json
{
  "message": "error",
  "data": "拉起支付失败"
}
```

**业务规则：**
- 需完成支付合规确认
- 套餐需启用且配置了 StripePriceId
- 系统需配置 Stripe API Secret 和 Webhook Secret
- 购买次数实时校验，超限返回错误
- 创建 pending 状态的 SubscriptionOrder，返回 Stripe Checkout 链接

---

#### 1.5 Creem 支付

**接口路径：** `POST /api/subscription/creem/pay`

**需求追溯：** [需求：4.6 页面 6 - 确认支付 / 5.4 支付回调处理]

**认证要求：** UserAuth（已登录用户）

**限流：** CriticalRateLimit

**请求参数：**
```json
{
  "plan_id": 1
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| plan_id | int | 是 | 要购买的套餐 ID |

**响应示例（成功）：**
```json
{
  "message": "success",
  "data": {
    "checkout_url": "https://api.creem.io/checkout/xxx",
    "order_id": "sub_ref_xxxxxxxx"
  }
}
```

**响应示例（失败）：**
```json
{
  "message": "error",
  "data": "拉起支付失败"
}
```

**业务规则：**
- 需完成支付合规确认
- 套餐需启用且配置了 CreemProductId
- 系统需配置 Creem Webhook Secret（测试模式可跳过）
- 购买次数实时校验

---

#### 1.6 易支付

**接口路径：** `POST /api/subscription/epay/pay`

**需求追溯：** [需求：4.6 页面 6 - 确认支付 / 5.4 支付回调处理]

**认证要求：** UserAuth（已登录用户）

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
| plan_id | int | 是 | 要购买的套餐 ID |
| payment_method | string | 是 | 支付方式（如 alipay、wxpay 等） [字典：sub_payment_method] |

**响应示例（成功）：**
```json
{
  "message": "success",
  "data": {
    "pid": "1001",
    "type": "alipay",
    "out_trade_no": "SUBUSR100NOxxxxxx",
    "notify_url": "https://example.com/api/subscription/epay/notify",
    "return_url": "https://example.com/api/subscription/epay/return",
    "name": "SUB:基础套餐",
    "money": "9.99",
    "sign": "xxxxxxxx",
    "sign_type": "MD5"
  },
  "url": "https://pay.example.com/submit.php"
}
```

**业务规则：**
- 需完成支付合规确认
- 套餐需启用且金额 >= 0.01
- payment_method 需在系统配置的可用支付方式列表中
- 管理员需配置易支付商户信息

---

#### 1.7 Stripe Webhook 回调

**接口路径：** `POST /api/subscription/stripe/webhook`

**需求追溯：** [需求：5.4 支付回调处理]

**认证要求：** 无（公开接口，通过 Stripe 签名验证安全性）

**请求头：**
| 参数 | 说明 |
|------|------|
| Stripe-Signature | Stripe 签名，包含时间戳和 HMAC-SHA256 签名值 |

**请求参数：** Stripe 标准 Event JSON（`checkout.session.completed` 事件）

**响应：**
- 成功：HTTP 200，返回空 JSON `{}`
- 签名验证失败：HTTP 400

**业务规则：**
- 验证 Stripe-Signature 签名有效性（使用 Webhook Secret）
- 仅处理 `checkout.session.completed` 事件
- 从 Event Data 中提取订单号，调用 `CompleteSubscriptionOrder` 完成订单（幂等）
- 同一订单号加锁防止并发处理

---

#### 1.8 Creem Webhook 回调

**接口路径：** `POST /api/subscription/creem/webhook`

**需求追溯：** [需求：5.4 支付回调处理]

**认证要求：** 无（公开接口，通过 Creem 签名验证安全性）

**请求头：**
| 参数 | 说明 |
|------|------|
| X-Creem-Signature | Creem 签名值 |

**请求参数：** Creem 标准 Webhook JSON（`order.created` 事件，状态为 paid）

**响应：**
- 成功：HTTP 200，返回空 JSON `{}`
- 签名验证失败：HTTP 400

**业务规则：**
- 验证 X-Creem-Signature 签名有效性（使用 Webhook Secret）
- 仅处理订单状态为 `paid` 的事件
- 从 Event Data 中提取订单号，调用 `CompleteSubscriptionOrder` 完成订单（幂等）
- 同一订单号加锁防止并发处理

---

#### 1.9 易支付回调通知

**接口路径：** `POST/GET /api/subscription/epay/notify`

**需求追溯：** [需求：5.4 支付回调处理]

**认证要求：** 无（公开接口，通过签名验证安全性）

**请求参数：** 易支付标准回调参数（通过 URL Query 或 POST Form 传递）

**响应：**
- 成功：返回纯文本 `success`
- 失败：返回纯文本 `fail`

**业务规则：**
- 验证签名有效性
- 仅处理交易成功状态（`TRADE_SUCCESS`）
- 调用 `CompleteSubscriptionOrder` 完成订单（幂等）
- 同一订单号加锁防止并发处理

---

#### 1.10 易支付同步返回

**接口路径：** `GET/POST /api/subscription/epay/return`

**需求追溯：** [需求：5.4 支付回调处理]

**认证要求：** 无

**请求参数：** 易支付标准返回参数

**响应：** 重定向到前端页面
- 成功：`/console/topup?pay=success`
- 失败：`/console/topup?pay=fail`
- 待处理：`/console/topup?pay=pending`

---

### 2. 管理员接口

管理员接口统一使用 `middleware.AdminAuth()` 中间件，要求管理员已登录。

#### 2.1 获取所有套餐列表

**接口路径：** `GET /api/subscription/admin/plans`

**需求追溯：** [需求：4.1 页面 1 - 管理员订阅管理页]

**认证要求：** AdminAuth（管理员）

**查询参数：** 无

**请求示例：**
```
GET /api/subscription/admin/plans
Authorization: Bearer <admin_token>
```

**响应示例：**
```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "plan": {
        "id": 1,
        "title": "基础套餐",
        "subtitle": "适合个人用户",
        "price_amount": 9.99,
        "currency": "USD",
        "duration_unit": "month",
        "duration_value": 1,
        "custom_seconds": 0,
        "enabled": true,
        "sort_order": 10,
        "stripe_price_id": "price_xxx",
        "creem_product_id": "",
        "max_purchase_per_user": 0,
        "upgrade_group": "vip",
        "total_amount": 5000000,
        "quota_reset_period": "monthly",
        "quota_reset_custom_seconds": 0,
        "created_at": 1716240000,
        "updated_at": 1716240000
      }
    }
  ]
}
```

**业务规则：**
- 返回所有套餐（含已禁用的）
- 按 `sort_order desc, id desc` 排序

---

#### 2.2 创建套餐

**接口路径：** `POST /api/subscription/admin/plans`

**需求追溯：** [需求：4.2 页面 2 - 套餐创建抽屉]

**认证要求：** AdminAuth（管理员） + 支付合规确认

**请求参数：**
```json
{
  "plan": {
    "title": "专业套餐",
    "subtitle": "适合团队使用",
    "price_amount": 29.99,
    "currency": "USD",
    "duration_unit": "month",
    "duration_value": 1,
    "custom_seconds": 0,
    "enabled": true,
    "sort_order": 5,
    "stripe_price_id": "price_yyy",
    "creem_product_id": "prod_yyy",
    "max_purchase_per_user": 3,
    "upgrade_group": "pro",
    "total_amount": 20000000,
    "quota_reset_period": "monthly",
    "quota_reset_custom_seconds": 0
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| plan.title | string | 是 | 套餐标题，最大 128 字符，不能为空 |
| plan.subtitle | string | 否 | 副标题，最大 255 字符 |
| plan.price_amount | float | 是 | 价格，>=0，<=9999，精确到小数点后 6 位 |
| plan.currency | string | 否 | 货币类型，默认 `USD`（当前强制 USD） |
| plan.duration_unit | string | 否 | 时长单位：`year`/`month`/`day`/`hour`/`custom`，默认 `month` [字典：sub_duration_unit] |
| plan.duration_value | int | 否 | 时长数值，>=1（custom 时忽略），默认 1 |
| plan.custom_seconds | int64 | 条件必填 | 自定义秒数，duration_unit 为 custom 时需 >0 |
| plan.enabled | bool | 否 | 是否启用，默认 true |
| plan.sort_order | int | 否 | 排序权重，默认 0 |
| plan.stripe_price_id | string | 否 | Stripe 定价 ID，最大 128 字符 |
| plan.creem_product_id | string | 否 | Creem 产品 ID，最大 128 字符 |
| plan.max_purchase_per_user | int | 否 | 每用户最大购买次数，>=0，0 表示无限制，默认 0 |
| plan.upgrade_group | string | 否 | 升级组名称，需为系统已存在的有效分组，最大 64 字符 |
| plan.total_amount | int64 | 否 | 总额度，>=0，0 表示无限制，默认 0 |
| plan.quota_reset_period | string | 否 | 重置周期：`never`/`daily`/`weekly`/`monthly`/`custom`，默认 `never` [字典：sub_reset_period] |
| plan.quota_reset_custom_seconds | int64 | 条件必填 | 自定义重置秒数，quota_reset_period 为 custom 时需 >0 |

**响应示例（成功）：**
```json
{
  "success": true,
  "message": "",
  "data": {
    "id": 2,
    "title": "专业套餐",
    "subtitle": "适合团队使用",
    "price_amount": 29.99,
    "..."
  }
}
```

**响应示例（失败）：**
```json
{
  "success": false,
  "message": "套餐标题不能为空"
}
```

**错误码：**

| 错误消息 | 说明 |
|---------|------|
| 参数错误 | 请求体解析失败 |
| 套餐标题不能为空 | title 为空 |
| 价格不能为负数 | price_amount < 0 |
| 价格不能超过9999 | price_amount > 9999 |
| 购买上限不能为负数 | max_purchase_per_user < 0 |
| 总额度不能为负数 | total_amount < 0 |
| 升级分组不存在 | upgrade_group 不是有效分组名 |
| 自定义重置周期需大于0秒 | quota_reset_period=custom 时 quota_reset_custom_seconds <= 0 |

---

#### 2.3 编辑套餐

**接口路径：** `PUT /api/subscription/admin/plans/:id`

**需求追溯：** [需求：4.2 页面 2 - 套餐编辑抽屉]

**认证要求：** AdminAuth（管理员） + 支付合规确认

**路径参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| id | int | 套餐 ID |

**请求参数：** 与创建套餐相同（完整 plan 对象），所有字段都会被更新。

**响应示例（成功）：**
```json
{
  "success": true,
  "message": "",
  "data": null
}
```

**业务规则：**
- 字段校验规则与创建相同
- 编辑使用事务更新，通过 map 显式指定所有可更新字段（允许零值更新）
- 更新后清除套餐缓存

---

#### 2.4 启用/禁用套餐

**接口路径：** `PATCH /api/subscription/admin/plans/:id`

**需求追溯：** [需求：4.3 页面 3 - 启用/禁用确认弹窗]

**认证要求：** AdminAuth（管理员） + 支付合规确认

**路径参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| id | int | 套餐 ID |

**请求参数：**
```json
{
  "enabled": false
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| enabled | bool | 是 | true=启用，false=禁用 |

**响应示例（成功）：**
```json
{
  "success": true,
  "message": "",
  "data": null
}
```

---

#### 2.5 管理员绑定订阅（旧接口）

**接口路径：** `POST /api/subscription/admin/bind`

**需求追溯：** [需求：4.4 页面 4 - 添加订阅]

**认证要求：** AdminAuth（管理员） + 支付合规确认

**请求参数：**
```json
{
  "user_id": 100,
  "plan_id": 1
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user_id | int | 是 | 目标用户 ID，>0 |
| plan_id | int | 是 | 要绑定的套餐 ID，>0 |

**响应示例（成功，无分组升级）：**
```json
{
  "success": true,
  "message": "",
  "data": null
}
```

**响应示例（成功，有分组升级）：**
```json
{
  "success": true,
  "message": "",
  "data": {
    "message": "用户分组将升级到 vip"
  }
}
```

**业务规则：**
- 直接创建 UserSubscription，无需支付
- 订阅来源标记为 `admin`
- 如果套餐有 upgrade_group，自动升级用户分组
- 校验购买次数上限

---

#### 2.6 查看用户订阅列表

**接口路径：** `GET /api/subscription/admin/users/:id/subscriptions`

**需求追溯：** [需求：4.4 页面 4 - 用户订阅管理弹窗]

**认证要求：** AdminAuth（管理员）

**路径参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| id | int | 用户 ID |

**响应示例：**
```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "subscription": {
        "id": 1,
        "user_id": 100,
        "plan_id": 1,
        "amount_total": 5000000,
        "amount_used": 1200000,
        "start_time": 1716240000,
        "end_time": 1718832000,
        "status": "active",
        "source": "order",
        "last_reset_time": 0,
        "next_reset_time": 1718832000,
        "upgrade_group": "vip",
        "prev_user_group": "default",
        "created_at": 1716240000,
        "updated_at": 1716240000
      }
    }
  ]
}
```

**业务规则：**
- 返回目标用户的所有订阅（含 active、expired、cancelled）
- 按 `end_time desc, id desc` 排序

---

#### 2.7 管理员为用户创建订阅

**接口路径：** `POST /api/subscription/admin/users/:id/subscriptions`

**需求追溯：** [需求：4.4 页面 4 - 添加订阅]

**认证要求：** AdminAuth（管理员） + 支付合规确认

**路径参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| id | int | 目标用户 ID |

**请求参数：**
```json
{
  "plan_id": 1
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| plan_id | int | 是 | 要绑定的套餐 ID，>0 |

**响应示例：** 与 2.5 管理员绑定订阅相同。

**业务规则：**
- 功能与 2.5 相同，只是用户 ID 通过路径参数传递
- 订阅来源标记为 `admin`

---

#### 2.8 作废用户订阅

**接口路径：** `POST /api/subscription/admin/user_subscriptions/:id/invalidate`

**需求追溯：** [需求：4.4 页面 4 - 作废订阅]

**认证要求：** AdminAuth（管理员）

**路径参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| id | int | 用户订阅实例 ID |

**请求参数：** 无

**响应示例（成功，无分组降级）：**
```json
{
  "success": true,
  "message": "",
  "data": null
}
```

**响应示例（成功，有分组降级）：**
```json
{
  "success": true,
  "message": "",
  "data": {
    "message": "用户分组将回退到 default"
  }
}
```

**业务规则：**
- 仅 active 状态的订阅才能作废
- 将订阅状态改为 `cancelled`，end_time 设为当前时间
- 如果订阅有 upgrade_group 且用户当前分组匹配，自动回退到 prev_user_group
- 使用事务 + 行锁保证并发安全

---

#### 2.9 删除用户订阅

**接口路径：** `DELETE /api/subscription/admin/user_subscriptions/:id`

**需求追溯：** [需求：4.4 页面 4 - 删除订阅]

**认证要求：** AdminAuth（管理员）

**路径参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| id | int | 用户订阅实例 ID |

**请求参数：** 无

**响应示例：** 与作废相同（可能包含分组降级信息）。

**业务规则：**
- 硬删除订阅记录
- 删除前同样检查并执行分组降级逻辑
- 使用事务 + 行锁保证并发安全

---

### 3. OpenAI 兼容接口

#### 3.1 获取订阅信息

**接口路径：** `GET /api/dashboard/billing/subscription`

**需求追溯：** [需求：5.6 OpenAI 兼容接口]

**认证要求：** UserAuth 或 TokenAuth

**响应示例：**
```json
{
  "object": "billing_subscription",
  "has_payment_method": true,
  "soft_limit_usd": 100.0,
  "hard_limit_usd": 100.0,
  "system_hard_limit_usd": 100.0,
  "access_until": 1718832000
}
```

**业务规则：**
- 无限额度显示为固定大数值 `100000000`
- 额度显示类型根据系统配置转换（USD/CNY/TOKENS）
- access_until 为 0 表示无过期

---

#### 3.2 获取使用量

**接口路径：** `GET /api/dashboard/billing/usage`

**需求追溯：** [需求：5.6 OpenAI 兼容接口]

**认证要求：** UserAuth 或 TokenAuth

**响应格式：** 标准 OpenAI billing usage 格式。

---

## 定时任务接口（非 HTTP）

以下功能通过定时任务自动执行，不对外暴露 HTTP 接口。

### 订阅过期处理

**触发方式：** 定时任务，每分钟执行

**处理逻辑：** [需求：5.1 订阅过期处理]
1. 查询 `status = active AND end_time <= now` 的 UserSubscription
2. 批量更新状态为 `expired`
3. 检查用户是否有其他活跃的升级订阅，没有则回退分组

### 订阅配额重置

**触发方式：** 定时任务，每分钟执行

**处理逻辑：** [需求：5.2 订阅配额重置]
1. 查询 `next_reset_time > 0 AND next_reset_time <= now AND status = active` 的 UserSubscription
2. 重置 `amount_used` 为 0
3. 计算并更新新的 `next_reset_time`

### 订单过期处理

**触发方式：** 定时任务，在订阅过期处理任务中一并执行

**处理逻辑：** [需求：5.7 订单过期处理]
1. 查询 `status = pending AND create_time < 阈值` 的 SubscriptionOrder
2. 批量更新状态为 `expired`

### 预消费记录清理

**触发方式：** 定时任务，每 30 分钟执行

**处理逻辑：** [需求：5.3 预消费记录清理]
1. 清理创建时间超过 7 天的预消费记录

---

## 安全说明

### 认证与授权

| 接口分类 | 认证方式 | 说明 |
|---------|---------|------|
| 用户端接口 | Bearer Token (UserAuth) | 需登录，数据范围限定为当前用户 |
| 管理员接口 | Bearer Token (AdminAuth) | 需管理员身份，部分接口需支付合规确认 |
| 支付回调 | 无认证（签名验证） | 通过支付网关签名验证安全性 |
| 易支付同步返回 | 无认证 | 浏览器重定向，处理完再跳转前端 |

### 限流

- Stripe/Creem/易支付支付接口使用 `CriticalRateLimit` 中间件限流

### 并发安全

- 订单完成使用 `FOR UPDATE` 行锁
- 订单号加分布式锁（`LockOrder/UnlockOrder`）
- 预消费记录通过 `request_id` 唯一索引保证幂等

---

**文档版本：** v1.0

**最后更新：** 2026-05-17

**作者：** lixuetao
