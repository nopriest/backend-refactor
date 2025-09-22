# AGENTS.md

面向智能代理与协作者的项目工作说明（Scope：本目录及其子目录）

## 项目概览

- 技术栈：Go 1.21、Chi 路由、JWT、Vercel 无服务器部署。
- 统一入口：`api/index.go` 作为唯一 Serverless 入口，所有路由经由 Chi 统一注册（避免在 Vercel 上分散多个函数）。
- 中间件：请求 ID、真实 IP、日志、恢复、超时、压缩、CORS，以及可选的鉴权中间件。
- 配置与环境：`pkg/config/config.go` 通过环境变量加载，支持本地文件库、PostgreSQL、Supabase 三种后端；Vercel 环境下有连接优化。
- 数据访问：以 `pkg/database/interface.go` 为契约，提供 `local`/`postgres`/`supabase` 实现与连接池/优化器。
- 统一响应：`pkg/utils/response.go` 定义标准的 APIResponse 包装，错误/分页/成功响应有统一写法。

## 目录结构（关键路径）

- `api/index.go`：Vercel 入口 + 路由聚合
- `pkg/config/`：配置装载与校验（.env 加载、环境判断、CORS 源）
- `pkg/handlers/`：业务 Handler（auth、snapshots、webhook）
- `pkg/middleware/`：中间件（鉴权、CORS、日志、恢复、校验）
- `pkg/models/`：数据模型（用户、标签/分组、订阅、AI 配额、Token Claims）
- `pkg/database/`：数据库接口与实现（local/postgres/supabase、连接池、Vercel 优化）
- `pkg/utils/`：通用工具（响应、JWT）
- `scripts/`：PostgreSQL 初始化脚本与一键初始化程序
- `vercel.json`：Vercel 配置（rewrite 到 `api/index.go`，安全与 CORS 头）

## 运行与调试

- 先决条件：Go 1.21+、Vercel CLI、（可选）Docker/PostgreSQL。
- 本地开发（推荐本地文件库）：
  1) 复制环境变量模板：`cp .env.example .env.local`
  2) 设置 `USE_LOCAL_DB=true`（或配置 Postgres/Supabase）
  3) 安装依赖：`make deps` 或 `go mod tidy`
  4) 启动：`vercel dev --listen 3000` 或 `make dev`
  5) 健康检查：GET `http://localhost:3000/`
- PostgreSQL 初始化：`make setup-db`（读取 `scripts/init_db.sql` 创建表并插入测试数据）。
- 调试端点（仅开发环境）：
  - `GET /debug/db-pool`：连接池/优化器状态
  - `GET /debug/db-schema`：数据库字段变更提示
  - `GET /debug/env-check`：关键环境变量就绪状态

## 部署（Vercel）

- 所有请求经 `vercel.json` rewrite 到 `api/index.go`；函数 `maxDuration=30s`，应用中路由层超时为 25s，请避免长耗时操作。
- Vercel 为易失性文件系统：local 文件库仅用于开发/临时；生产请优先 Supabase（REST）或 PostgreSQL。
- 生产环境务必设置：`ENVIRONMENT=production`、`JWT_SECRET`、数据库/授权相关变量（见下）。

## 环境变量（常用）

- 基础：`ENVIRONMENT`（development/production）、`PORT`、`DEBUG`
- JWT：`JWT_SECRET`
- 数据库优先级：`USE_LOCAL_DB` → `POSTGRES_DSN` → (`SUPABASE_URL` + `SUPABASE_SERVICE_KEY`)
- OAuth：`GOOGLE_CLIENT_ID`、`GOOGLE_CLIENT_SECRET`、`GITHUB_CLIENT_ID`、`GITHUB_CLIENT_SECRET`、`OAUTH_REDIRECT_URI`、`BASE_URL`
- CORS：`ALLOWED_ORIGINS`（逗号分隔，开发可为 `*`）
- Webhook/Paddle（可选）：`PADDLE_API_KEY`、`PADDLE_ENVIRONMENT`、`PADDLE_WEBHOOK_SECRET`、`PADDLE_PRO_PRICE_ID`、`PADDLE_POWER_PRICE_ID`
- 前端回调（可选）：`FRONTEND_CALLBACK_URL`

## 代码风格与约定

- 语言与格式：Go 1.21；使用 `go fmt`（可 `make fmt`）；新增依赖更新 `go.mod`。
- 命名与可见性：导出符号使用大写开头；包内私有使用小写；保持与现有包/文件命名一致。
- 日志：开发期允许 `fmt.Printf` 输出；避免泄露敏感信息（Token/Secret/完整 DSN），必要时打码；生产建议精简结构化日志（见 `middleware/logging.go`）。
- 中间件顺序：请求 ID → RealIP → 日志 → 恢复 → CORS → 超时/压缩 → （开发心跳）；鉴权按路由分组启用。
- 统一响应：使用 `pkg/utils/response.go` 提供的方法（如 `WriteSuccessResponse`、`WriteBadRequestResponse`、`WriteErrorResponseWithCode`、`WritePaginatedResponse`）。不要手写 JSON。
- 请求解析：使用 `utils.ParseJSONBody`；变更时补充 `middleware/validation.go` 里的校验（Content-Type、大小限制等）。
- 鉴权：受保护路由需挂载 `middleware.AuthMiddleware(cfg)`；从 `middleware.GetUserFromContext` 读取用户；错误时返回 401/403。
- JWT：通过 `utils.JWTService` 生成/校验，access 15m、refresh 7d（默认）。
- 数据访问：面向 `database.DatabaseInterface` 编程；不要直接耦合具体实现。
- 路由：在 `api/index.go` 聚合；新增路由尽量与现有分组保持一致（`/api/auth`、`/api/snapshots`、`/api/webhooks` 等）。
- 兼容性：保持响应字段与 `pkg/models` 中的 JSON Tag 一致，避免破坏现有客户端。

## 扩展开发指南

新增 API（示例步骤）：
1) 在 `pkg/models/` 定义/扩展请求与响应模型（谨慎命名与 JSON tag）。
2) 在 `pkg/handlers/` 新增或扩展 Handler（拆分逻辑，复用 `utils`）。
3) 在 `api/index.go` 注册路由，必要时挂载鉴权/校验中间件。
4) 如需数据落库，先在 `pkg/database/interface.go` 增加接口方法；为 `local.go` 与（首选）`supabase.go` 实现；PostgreSQL 需要时同步实现 SQL。
5) 更新 `scripts/init_db.sql`（若涉及表结构），并在 README/本文件中同步说明迁移。
6) 根据需要补充环境变量与配置校验（`pkg/config/config.go:Validate`）。

数据库实现注意：
- local 实现用于开发/演示，功能完整度有限，不支持订阅/AI 余额的真实更新。
- supabase 实现优先用于 Vercel 生产（避免 IPv6 出站限制）：基于 REST，注意 header（apikey/Authorization/Prefer）。
- postgres 实现部分方法为 TODO（用户 CRUD、订阅/AI Credits 等），启用前需补全。
- 在 Vercel 下使用 `database.GetOptimizedDatabase` 复用连接；其他环境使用 `GetDatabase` + 连接池。

OAuth 与回调：
- 入口：`POST /api/auth/oauth/google|github` 接收授权码，服务端换 Token 并回填用户信息与 JWT。
- 回调页面：`GET /api/oauth/callback|google/callback|github/callback|extension/callback`；Web/扩展/API 客户端的返回路径与格式已区分处理。
- 确保 `OAUTH_REDIRECT_URI`、`BASE_URL`、对应 OAuth Client 配置与实际域名一致。

Webhook（Paddle）：
- 路由：`POST /api/webhooks/paddle`
- 校验：`Paddle-Signature` HMAC 校验（见 `pkg/handlers/webhook.go`）。
- 计划与价格：通过 `PADDLE_PRO_PRICE_ID`、`PADDLE_POWER_PRICE_ID` 识别；若为测试商品，使用名称兜底。

## 安全基线

- 切勿在日志中打印完整 Token/密钥/密码/完整 DSN。
- 严格使用 `AllowedOrigins` 控制 CORS；生产环境避免 `*`。
- 仅对需要的路由启用鉴权；JWT 过期与类型校验要完整（见中间件）。
- Vercel 文件系统不可持久化，生产不要依赖 `local` 数据库保存状态。
- 存在的速率限制为占位实现（`RateLimitByIP`）；如面向公网，建议尽快落地。

## 命令与脚本

- 依赖安装：`make deps`
- 本地开发：`make dev`（等价 `vercel dev --listen 3000`）
- 初始化数据库：`make setup-db`
- 构建工具脚本：`make build`
- 代码格式化：`make fmt`
- 清理：`make clean`

## 测试

- 当前仓库未包含实质性单元测试；可在各包下新增 `_test.go` 文件，按 Go testing 规范组织。
- 优先对 Handler 的输入输出与 `utils` 工具进行覆盖；数据库实现可通过接口注入 mock。

## 禁改/注意事项

- 不要新增/改动 Vercel 入口路径（保持 `api/index.go` 为唯一入口），除非业务强需求确认。
- 不要绕过 `utils/response.go` 直接写 JSON；保持统一响应格式。
- 不要在生产日志中输出敏感变量；如需排障请保证打码。
- 新增接口务必更新接口文档（README/本文件），并考虑客户端兼容性。
- 变更数据库契约需同步三处：接口 + 至少两份实现（local/supabase）+ 初始化脚本（如涉及 DDL）。

## 已知限制与后续

- PostgreSQL 实现尚有若干 TODO（用户 CRUD、订阅、AI 额度等）。
- 速率限制/防滥用策略未完全实现。
- README 在部分终端环境可能出现编码显示异常，优先参考本文件与源码注释。

---
本文件旨在为智能代理与协作者提供一致的改动准则与操作手册。如需扩展本规范，请在 PR 中一并更新本文件。

