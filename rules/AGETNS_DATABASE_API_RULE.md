# API 接口与数据库设计规约

本文档从代码库中提取的约定，描述 API 接口的固定结构和数据库设计的固定模式。新增功能时必须遵循这些规约。

---

## 1. API 接口规约

### 1.1 路由前缀

| 前缀 | 用途 | 认证要求 |
|------|------|---------|
| `/api/*` | 管理后台和用户接口 | 按路由组分级 |
| `/v1/*` | AI API relay（OpenAI 兼容） | Token 认证 |
| `/v1beta/*` | Google Gemini 原生格式 | Token 认证 |
| `/mj/*` | Midjourney 代理 | Token 认证 |
| `/suno/*` | Suno 代理 | Token 认证 |
| `/pg/*` | Playground | Token 认证 |

### 1.2 统一响应结构

所有 `/api/*` 管理后台接口使用统一的 JSON 响应结构。管理后台接口统一使用 HTTP 200，通过响应体中的 `success` 字段区分成功和失败。relay 转发等非管理接口可能使用其他 HTTP 状态码。

**成功响应**：
```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

**错误响应**：
```json
{
  "success": false,
  "message": "错误描述"
}
```

**对应 Go 调用**：
```go
// 成功
common.ApiSuccess(c, data)
common.ApiSuccessI18n(c, i18n.MsgXxx, data)

// 错误
common.ApiError(c, err)
common.ApiErrorMsg(c, "具体错误描述")
common.ApiErrorI18n(c, i18n.MsgXxx)
```

推荐使用上述工具函数构造响应，避免手动拼接 `gin.H{...}`。历史代码中存在直接使用 `c.JSON()` 的写法，新增代码应统一使用工具函数。

### 1.3 分页查询

**请求参数**（Query String）：

| 参数 | 别名 | 说明 | 默认值 | 最大值 |
|------|------|------|--------|--------|
| `p` | `page` | 页码（从 1 开始） | 1 | - |
| `page_size` | `ps`, `size` | 每页条数 | 10 | 100 |

**响应结构**（在 `data` 中）：
```json
{
  "success": true,
  "message": "",
  "data": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "items": []
  }
}
```

**Controller 模式**：
```go
pageInfo := common.GetPageQuery(c)
// ... 查询数据 ...
pageInfo.SetTotal(int(total))
pageInfo.SetItems(items)
common.ApiSuccess(c, pageInfo)
```

### 1.4 认证中间件分级

| 中间件 | 角色 | 说明 |
|--------|------|------|
| （无中间件） | 公开 | 无需认证 |
| `middleware.UserAuth()` | 普通用户（role >= 1） | Session 或 Token 认证 |
| `middleware.AdminAuth()` | 管理员（role >= 10） | 管理后台操作 |
| `middleware.RootAuth()` | 超级管理员（role >= 100） | 系统设置等敏感操作 |
| `middleware.TokenAuth()` | API Token | relay 转发用 |
| `middleware.TokenOrUserAuth()` | Session 或 Token | 混合认证 |
| `middleware.TokenAuthReadOnly()` | API Token（只读） | 仅允许读取操作 |

**角色常量**（`common/constants.go`）：
- `RoleCommonUser = 1`
- `RoleAdminUser = 10`
- `RoleRootUser = 100`

### 1.5 限流中间件

| 中间件 | 用途 |
|--------|------|
| `middleware.GlobalAPIRateLimit()` | API 全局限流 |
| `middleware.CriticalRateLimit()` | 关键操作限流（登录、注册、支付） |
| `middleware.SearchRateLimit()` | 搜索接口限流 |
| `middleware.EmailVerificationRateLimit()` | 邮箱验证限流 |
| `middleware.TurnstileCheck()` | Turnstile 人机验证 |

### 1.6 请求参数绑定

Controller 层使用 `common.UnmarshalBodyReusable(c, &req)` 绑定请求体，支持 JSON、Form 和 Multipart 三种 Content-Type 自动识别。

### 1.7 国际化

所有面向客户端的错误消息和提示文案必须通过 i18n key 返回：
- 错误消息：`common.ApiErrorI18n(c, i18n.MsgXxx)`
- 成功消息：`common.ApiSuccessI18n(c, i18n.MsgXxx, data)`
- 禁止在 controller 响应中硬编码中文或其他自然语言字符串

### 1.8 路由组织模式

路由按资源分组，使用子路由组管理：
```go
// 公开路由
apiRouter.GET("/status", controller.GetStatus)

// 用户级别路由
selfRoute := userRoute.Group("/")
selfRoute.Use(middleware.UserAuth())

// 管理员级别路由
adminRoute := userRoute.Group("/")
adminRoute.Use(middleware.AdminAuth())
```

---

## 2. 数据库设计规约

### 2.1 三数据库兼容

所有数据库代码必须同时兼容 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6。

兼容性变量（`model/main.go`）：

| 变量 | MySQL/SQLite | PostgreSQL |
|------|-------------|------------|
| `commonGroupCol` | `` `group` `` | `"group"` |
| `commonKeyCol` | `` `key` `` | `"key"` |
| `commonTrueVal` | `1` | `true` |
| `commonFalseVal` | `0` | `false` |

检测标志：`common.UsingPostgreSQL`、`common.UsingSQLite`、`common.UsingMySQL`

### 2.2 模型定义模式

项目不使用统一 BaseModel 嵌入结构体，每个模型独立定义所需字段。

**典型模型结构**：
```go
type Xxx struct {
    Id        int            `json:"id"`
    Name      string         `json:"name" gorm:"index"`
    Status    int            `json:"status" gorm:"default:1"`
    CreatedAt int64          `json:"created_at" gorm:"bigint"`
    UpdatedAt int64          `json:"updated_at" gorm:"bigint"`
    DeletedAt gorm.DeletedAt `gorm:"index"`
}
```

### 2.3 固定字段约定

以下字段在项目中有一致的使用模式：

**主键**：
```go
Id int `json:"id"`
```
使用 `int` 类型，GORM 自动递增。极少数大表使用 `int64`。

**时间字段**：
```go
CreatedAt  int64 `json:"created_at" gorm:"bigint"`           // 创建时间，Unix 时间戳
UpdatedAt  int64 `json:"updated_at" gorm:"bigint"`           // 更新时间，Unix 时间戳
CreatedTime int64 `json:"created_time" gorm:"bigint"`       // 旧模型用的创建时间
```
统一使用 `int64` Unix 时间戳，不用 `time.Time`。项目中存在两种设置方式：

**方式一：GORM Hook**（较新模型，如订阅相关模型）：
```go
func (x *Xxx) BeforeCreate(tx *gorm.DB) error {
    now := common.GetTimestamp()
    x.CreatedAt = now
    x.UpdatedAt = now
    return nil
}

func (x *Xxx) BeforeUpdate(tx *gorm.DB) error {
    x.UpdatedAt = common.GetTimestamp()
    return nil
}
```

**方式二：GORM Tag**（旧模型，如 User、Token、Channel 等）：
```go
CreatedAt int64 `json:"created_at" gorm:"bigint;autoCreateTime"`
UpdatedAt int64 `json:"updated_at" gorm:"bigint;autoUpdateTime"`
```

两种方式在项目中并存，新增模型建议统一使用方式一（Hook），更灵活且不依赖 GORM tag 隐式行为。

**状态字段**：
```go
Status int `json:"status" gorm:"default:1"`
```
状态值约定：
- `1` = 启用（`UserStatusEnabled`、`TokenStatusEnabled`、`ChannelStatusEnabled`）
- `2` = 禁用（`UserStatusDisabled`、`TokenStatusDisabled`）
- 不使用 `0`，因为 `0` 是 Go 的零值默认值

**软删除**：
```go
DeletedAt gorm.DeletedAt `gorm:"index"`
```

**用户关联**：
```go
UserId int `json:"user_id" gorm:"index"`
```

**分组字段**：
```go
Group string `json:"group" gorm:"type:varchar(64);default:'default'"`
```

**额度字段**：
```go
Quota     int `json:"quota" gorm:"type:int;default:0"`
UsedQuota int `json:"used_quota" gorm:"type:int;default:0"`
```

**备注字段**：
```go
Remark string `json:"remark,omitempty" gorm:"type:varchar(255)" validate:"max=255"`
```

### 2.4 GORM Tag 使用规范

```go
// 索引
`gorm:"index"`                                          // 普通索引
`gorm:"uniqueIndex"`                                    // 唯一索引
`gorm:"index:idx_name,priority:1"`                      // 命名复合索引

// 字段类型
`gorm:"type:varchar(128)"`                              // 变长字符串
`gorm:"type:text"`                                      // 长文本、JSON 字符串
`gorm:"type:bigint"`                                    // 大整数（时间戳、金额）
`gorm:"type:decimal(10,6)"`                             // 精确小数（价格）
`gorm:"type:int"`                                       // 普通整数

// 约束
`gorm:"not null"`                                       // 非空
`gorm:"default:0"`                                      // 默认值
`gorm:"unique"`                                         // 唯一约束

// 特殊
`gorm:"column:custom_name"`                             // 自定义列名
`gorm:"->"`                                             // 只读（计算字段）
`gorm:"-"`                                              // 不映射到数据库
`gorm:"-:all"`                                          // 完全忽略
```

### 2.5 表名规则

遵循 GORM 默认约定：结构体名复数化为 snake_case 表名（如 `User` -> `users`，`SubscriptionPlan` -> `subscription_plans`）。需要自定义时实现 `TableName()` 方法。

### 2.6 JSON 字段处理

三种方式，按复杂度选择：

**方式一：字符串存储**（简单场景，推荐）
```go
Setting string `json:"setting" gorm:"type:text"`
// 业务层自行序列化/反序列化
```

**方式二：json.RawMessage**（透传场景）
```go
Data json.RawMessage `json:"data" gorm:"type:text"`
```

**方式三：实现 Scanner/Valuer 接口**（结构化场景）
```go
type ChannelInfo struct {
    IsMultiKey bool `json:"is_multi_key"`
}

func (c ChannelInfo) Value() (driver.Value, error) {
    return common.Marshal(&c)
}

func (c *ChannelInfo) Scan(value interface{}) error {
    bytesValue, _ := value.([]byte)
    return common.Unmarshal(bytesValue, c)
}
```

注意：JSON 数据统一使用 `type:text` 存储，不使用 `type:json` 或 `type:jsonb`，确保三种数据库兼容。历史代码中存在少量 `type:json` 的用法，新增字段应统一使用 `type:text`。

### 2.7 模型 CRUD 方法模式

```go
// 创建
func (x *Xxx) Insert() error {
    return DB.Create(x).Error
}

// 更新
func (x *Xxx) Update() error {
    return DB.Save(x).Error
}

// 按ID查询
func GetXxxById(id int) (*Xxx, error) {
    var x Xxx
    err := DB.Where("id = ?", id).First(&x).Error
    return &x, err
}

// 分页列表
func GetAllXxx(pageInfo *common.PageInfo) (items []*Xxx, total int64, err error) {
    err = DB.Model(&Xxx{}).Count(&total).Error
    if err != nil {
        return
    }
    err = DB.Order("id desc").Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).Find(&items).Error
    return
}

// 软删除
func DeleteXxxById(id int) error {
    return DB.Where("id = ?", id).Delete(&Xxx{}).Error
}
```

**常见变体方法**：

```go
// 可更新零值字段（使用 Select 显式指定列）
func (x *Xxx) SelectUpdate() error {
    return DB.Select("status", "updated_at").Updates(x).Error
}

// 硬删除（绕过软删除）
func (x *Xxx) HardDelete() error {
    return DB.Unscoped().Delete(x).Error
}

// 批量插入
func BatchInsertXxx(items []Xxx) error {
    return DB.Create(&items).Error
}

// 带权限检查的删除
func DeleteXxxById(id int, userId int) error {
    return DB.Where("id = ? AND user_id = ?", id, userId).Delete(&Xxx{}).Error
}
```

### 2.8 数据库迁移

使用 GORM AutoMigrate，禁止创建 `.sql` 迁移文件。

新表：在 `model/` 下定义结构体，加入 `model/main.go` 的 `migrateDB()` 中 `DB.AutoMigrate(...)` 列表。

新列：在结构体中添加字段，AutoMigrate 启动时自动 `ADD COLUMN`。

修改列类型：编写幂等的预迁移函数，在 `DB.AutoMigrate()` 之前调用，处理三种数据库的语法差异。

### 2.9 复合唯一索引与软删除

需要唯一约束且支持软删除的模型，将 `DeletedAt` 纳入唯一索引：
```go
ModelName string         `gorm:"uniqueIndex:uk_model_name_delete_at,priority:1"`
DeletedAt gorm.DeletedAt `gorm:"index;uniqueIndex:uk_model_name_delete_at,priority:2"`
```

### 2.10 可空字段

使用指针类型表示数据库可空字段：
```go
OpenAIOrganization *string `json:"openai_organization"`
AccessToken        *string `json:"access_token" gorm:"type:char(32);uniqueIndex"`
AllowIps           *string `json:"allow_ips" gorm:"default:''"`
```

### 2.11 敏感字段标记

不存储到数据库的字段使用 `gorm:"-:all"`：
```go
OriginalPassword   string `json:"original_password" gorm:"-:all"`
VerificationCode   string `json:"verification_code" gorm:"-:all"`
```

仅内存使用的缓存字段使用 `gorm:"-"`：
```go
Keys []string `json:"-" gorm:"-"`
```

---

## 3. 命名约定汇总

### 3.1 Go 结构体字段

使用 PascalCase，GORM 自动转换为 snake_case 数据库列名：
- `UserId` -> `user_id`
- `CreatedAt` -> `created_at`
- `ModelName` -> `model_name`

### 3.2 JSON key

使用 snake_case，与数据库列名一致：
- `json:"user_id"`
- `json:"created_at"`

### 3.3 数据库列名

统一 snake_case，GORM 自动转换。需要显式指定时使用 `column` tag：
- `gorm:"column:github_id"`

### 3.4 状态值

统一使用 `1` = 启用，`2` = 禁用。不用 `0`，避免与 Go 零值混淆。
