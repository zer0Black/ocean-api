# 架构分析

## 项目定位

AI API 网关/代理平台。将 40+ 上游 AI Provider 聚合在统一的 OpenAI 兼容 API 之后，提供用户管理、计费、限流和管理后台。

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.22+, Gin, GORM v2 |
| 前端 Default | React 19, TypeScript 5.9, Rsbuild, TanStack Router, Tailwind 4, Base UI + Radix UI |
| 前端 Classic | React 18, TypeScript 4.4, Vite, React Router 6, Semi UI |
| 桌面端 | Electron（内嵌 Go 后端二进制） |
| 数据库 | SQLite / MySQL >= 5.7.8 / PostgreSQL >= 9.6（三库同时兼容） |
| 缓存 | Redis + 内存混合缓存（Redis 不可用时自动降级） |
| 认证 | JWT, WebAuthn Passkeys, OAuth (GitHub, Discord, OIDC 等) |
| 国际化 | 后端 go-i18n (en/zh), 前端 i18next (en/zh/fr/ru/ja/vi) |

## 整体架构

```
+------------------------------------------------------------------+
|                        Client (API Consumer)                      |
+------------------------------------------------------------------+
         |  OpenAI-compatible API  (/v1/chat/completions, etc.)
         v
+------------------------------------------------------------------+
|                          Gin HTTP Server                          |
|  +------------------------------------------------------------+  |
|  |                    Middleware Chain                         |  |
|  |  CORS -> Gzip -> PerfMonitor -> Auth -> Distributor -> ... |  |
|  +------------------------------------------------------------+  |
|         |                          |                             |
|         v                          v                             |
|  +------------------+    +------------------+                   |
|  |   API Router     |    |  Relay Router    |                   |
|  | (管理/用户/配置)  |    | (AI 模型转发)     |                   |
|  +------------------+    +------------------+                   |
|         |                          |                             |
|         v                          v                             |
|  +------------------+    +------------------+                   |
|  |  Controller 层   |    | Relay Adaptor    |                   |
|  |  (72 个控制器)    |    |  Factory         |                   |
|  +------------------+    +------------------+                   |
|         |                          |                             |
|         v                          v                             |
|  +------------------+    +------------------+                   |
|  |  Service 层      |    | Provider         |                   |
|  | (格式转换/频道   |    |  Adaptors (42)   |                   |
|  |  选择/计费会话)  |    | OpenAI/Claude/   |                   |
|  +------------------+    | Gemini/AWS/...   |                   |
|         |                +------------------+                   |
|         |                         |                             |
|         v                         v                             |
|  +------------------------------------------------------------+  |
|  |                    Model 层 (GORM)                          |  |
|  |   User / Token / Channel / Log / Option / Ability          |  |
|  +------------------------------------------------------------+  |
|                              |                                    |
|                              v                                    |
|  +------------------------------------------------------------+  |
|  |         SQLite / MySQL / PostgreSQL                         |  |
|  +------------------------------------------------------------+  |
+------------------------------------------------------------------+
         |                              |
         v                              v
+------------------+          +------------------+
|  Redis Cache     |          | Upstream AI APIs |
|  (可选，降级为    |          | OpenAI / Claude  |
|   内存缓存)       |          | Gemini / Bedrock |
+------------------+          | / Azure / ...    |
                              +------------------+
```

## 后端分层详解

### 入口和初始化

`main.go` 的启动顺序：环境变量 → 日志 → 模型设置 → HTTP 客户端 → Token 编码器 → 数据库（主库 + 日志库可选分离）→ Redis → i18n → OAuth → 后台任务（频道缓存同步、配置热更新、数据看板、频道自动测试、订阅配额重置）→ HTTP 服务。

### Router 层

按职责分为五个模块：

| 文件 | 职责 |
|---|---|
| `api-router.go` | 业务 API（用户、频道、Token、日志、数据、模型、部署等） |
| `relay-router.go` | AI 模型转发路由，兼容 OpenAI/Claude/Gemini 路径格式 |
| `video-router.go` | 视频处理路由 |
| `web-router.go` | 前端静态资源 |
| `dashboard.go` | 管理面板 |

### Middleware 层

按执行顺序排列：

1. **CORS**：跨域资源共享
2. **Gzip 解压**：处理压缩请求体
3. **性能监控**：系统性能检查
4. **Auth**（`middleware/auth.go`，约 12,700 行）：Token 认证 / Session 认证，处理 WebSocket、Gemini API、Anthropic API 的特殊认证方式，验证用户状态、IP 限制、模型访问权限
5. **Distributor**（`middleware/distributor.go`，约 17,300 行）：**核心中间件**，从请求提取模型名称，根据 Token 分组配置选择渠道，支持 auto 分组自动选择最优渠道，实现渠道亲和性（Channel Affinity）

### Controller / Service / Model

Controller 层 72 个文件按功能模块划分（用户、频道、OAuth、日志、模型等）。Service 层处理核心业务逻辑：格式转换（`convert.go`，约 32,800 行）、频道亲和性（`channel_affinity.go`）、计费会话（`billing_session.go`）、频道选择算法。

Model 层核心实体关联：

```
User 1--* Token（API Key）
User 1--* Log（消费记录）
User 1--* Subscription（订阅）
Channel *--* Ability（支持的模型能力）
Channel 1--* Log（请求日志）
Token 1--* Log（使用记录）
```

## Relay 系统（核心）

Relay 是整个项目的核心子系统，负责将统一格式的请求转换并转发到不同的上游 AI Provider。

### 架构

```
Client Request (OpenAI 格式)
        |
        v
+------------------+
| Relay Router     |  路由匹配，识别请求类型
+------------------+
        |
        v
+------------------+
| Distributor      |  选择渠道，设置上下文
| Middleware       |  (Channel ID, Type, Key 轮换)
+------------------+
        |
        v
+------------------+
| Adaptor Factory  |  根据 ChannelType 创建对应 Adaptor
+------------------+
        |
        v
+---------------------------+
| Adaptor Interface         |  统一接口
|                           |
| Init()                    |
| GetRequestURL()           |
| SetupRequestHeader()      |
| ConvertOpenAIRequest()    |  <-- 格式转换核心
| DoRequest()               |
| DoResponse()              |  <-- 流式/非流式处理
| GetModelList()            |
| GetChannelName()          |
+---------------------------+
        |                    \
        v                     \
+------------------+    +------------------+
| OpenAI Adaptor   |    | Claude Adaptor   |  ...
| (含 Azure/      |    | (请求格式转换     |
|  Vertex 变体)   |    |  流式 SSE 处理)   |
+------------------+    +------------------+
        |                     |
        v                     v
   api.openai.com         api.anthropic.com
```

### 已支持的 42 个 Provider

AI360, Ali (通义千问), AWS Bedrock, Baidu (文心), Baidu_V2, Claude (Anthropic), Cloudflare, Codex, Cohere, Coze, DeepSeek, Dify, Gemini (Google), Jimeng, Jina, Lingyiwanwu (零一万物), Minimax, Mistral, MokaAI, Moonshot (月之暗面), Ollama, OpenAI (含 Azure), OpenRouter, Palm, Perplexity, Replicate, SiliconFlow, Submodel, Tencent (混元), Vertex AI, Volcengine (火山引擎), XAI, Xinference, Xunfei (讯飞), Zhipu (智谱), Zhipu_4V 等。

### 设计模式

三种模式协同工作：

- **适配器模式**：每个 Provider 实现统一的 `Adaptor` 接口，封装 API 差异
- **策略模式**：运行时根据渠道类型选择具体适配器，每个适配器有独立的计费策略
- **模板方法模式**：`api_request.go` 定义统一的请求处理流程（初始化 → URL 构建 → Header 设置 → 格式转换 → 发送请求 → 处理响应），各适配器实现具体步骤

新增 Provider 只需：实现 `Adaptor` 接口 → 在 `constant/` 注册渠道类型 → 在工厂方法中添加创建逻辑。对核心流程零侵入。

### 任务型适配器

除标准请求-响应模式外，还有 `TaskAdaptor` 接口处理异步任务类 API（视频生成、图像生成等），支持任务提交 → 状态轮询 → 结果获取的完整生命周期。

## 计费系统

### 数据流

```
请求到达 → 估算费用 → 预扣费 → 转发请求 → 接收响应 → 实际结算 → 异步写日志
              |           |                        |
              v           v                        v
         查模型单价   扣额度/订阅额度        补扣或退还差额
         应用分组倍率  创建 BillingSession
```

### 预扣费机制

1. 检查用户/Token 额度是否充足
2. 执行预扣费（钱包模式直接扣额度，订阅模式预扣订阅额度并记录日志）
3. 信任额度充足的 VIP 用户可跳过预扣费

### 计费表达式引擎

基于 `expr-lang/expr` 实现动态计费规则，支持变量（prompt/completion/cache tokens、输入长度、图像/音频 tokens）和内置函数（分层计价 tier()、读取请求头 header()、读取请求体 param()、时间函数等）。编译后的表达式程序缓存以提升性能。

### 配额层级

用户配额 → 订阅配额 → Token 配额 → 分组配额，多层级检查防止透支。

## 多数据库兼容

通过 GORM 抽象 + 运行时分支处理三库差异：

| 差异点 | MySQL / SQLite | PostgreSQL |
|---|---|---|
| 列名引号 | \`column\` | "column" |
| 布尔值 | 1 / 0 | true / false |
| JSON 存储 | TEXT 类型 | 可用 TEXT 替代 JSONB |
| 聚合函数 | GROUP_CONCAT | STRING_AGG |

项目使用 `commonGroupCol`、`commonKeyCol` 变量处理 `group`、`key` 等保留字列名，使用 `common.UsingPostgreSQL`、`common.UsingSQLite`、`common.UsingMySQL` 标志分支数据库特定逻辑。日志数据库可通过 `LOG_SQL_DSN` 环境变量独立配置。

## 缓存策略

混合缓存（`pkg/cachex/`）：Redis 优先，不可用时自动降级到内存缓存（`hot.HotCache`）。使用场景包括渠道信息缓存、用户基本信息缓存、编译后的计费表达式缓存、系统配置缓存。不同数据类型通过命名空间隔离，配置更新时主动清理对应缓存。

## 前端架构

### 双前端并存

| | Default | Classic |
|---|---|---|
| 定位 | 现代化，新功能开发 | 稳定维护，向后兼容 |
| React | 19.2 | 18.2 |
| 构建 | Rsbuild (RSPack) | Vite 5 |
| 路由 | TanStack Router（文件系统路由） | React Router 6 |
| UI 库 | Base UI + Radix UI | Semi UI |
| 样式 | Tailwind CSS 4 | Tailwind CSS 3 |
| 状态管理 | Zustand + TanStack Query | 未统一 |

### Default 前端组织模式

采用 Feature-First 架构：

```
src/
├── components/ui/       # 50+ 基础 UI 组件
├── features/            # 按业务功能组织
│   ├── auth/           # 认证
│   ├── channels/       # 渠道管理
│   ├── chat/           # 聊天
│   ├── dashboard/      # 仪表板
│   ├── models/         # 模型管理
│   └── users/          # 用户管理
├── routes/             # 文件系统路由
├── stores/             # Zustand 状态（auth, notification, system-config）
├── hooks/              # 自定义 Hooks
├── lib/                # 工具函数和 API 封装
└── i18n/               # 国际化
```

### API 交互

通过 Axios 封装（`lib/api.ts`）：同源请求、请求去重、统一业务响应格式处理（`{success, message, data}`）、全局错误 Toast、401 自动登出。与 TanStack Query 集成，路由级别的 QueryClient 上下文，支持意图预加载。

## 关键架构特征

**高度可扩展**：Provider 适配器机制使新增上游只需实现接口，计费表达式引擎支持动态定价规则，插件化的中间件链。

**零外部依赖可运行**：SQLite + 内存缓存模式下单二进制部署，无需 MySQL/PostgreSQL/Redis。生产环境再切换到完整数据库和 Redis。

**生产级可靠性**：预扣费防透支、多级重试和自动分组切换、异步日志写入、独立日志数据库、渠道亲和性优化。

**多租户隔离**：分组（Group）机制隔离不同租户的渠道和计费规则，灵活的订阅和钱包双计费模式。
