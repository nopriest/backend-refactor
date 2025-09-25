# AGENTS.md

本文件为协作型智能体在本仓库的工作指引（Scope 为本目录树）。

## 项目概述

- 技术栈：Go 1.21、Chi 路由、JWT、Vercel 无服务器
- 统一入口：`api/index.go` 作为唯一 Serverless 入口，由 Chi 统一注册路由
- 中间件：RequestID、RealIP、Normalize、Logger、Recover、Timeout、Compress、CORS、Auth
- 配置与环境：`pkg/config/config.go` 通过环境变量加载；仅支持 PostgreSQL 与 Supabase；Vercel 环境具备连接优化
- 数据访问：以 `pkg/database/interface.go` 为契约，提供 `postgres`/`supabase` 实现与连接池/优化器
- 统一响应：`pkg/utils/response.go` 定义标准 APIResponse

## 目录结构与关键路径

- `api/index.go`：Vercel 入口 + 路由汇总
- `pkg/config/`：配置装载与校验（.env、环境判断、CORS）
- `pkg/handlers/`：业务 Handler（auth、snapshots、webhook、collections、orgs）
- `pkg/middleware/`：中间件（鉴权、CORS、日志、恢复、校验、Normalize）
- `pkg/models/`：领域模型（用户、组织/空间、集合/条目、订阅、AI 额度、Claims）
- `pkg/database/`：接口与实现（interface.go、postgres.go、supabase.go、pool.go、vercel_optimizer.go）
- `pkg/utils/`：通用工具（响应、JWT）
- `scripts/`：PostgreSQL 初始化脚本与辅助工具
- `vercel.json`：Vercel 配置（rewrite 到 `api/index.go`，统一 CORS 头）

## 开发指引

- 依赖：Go 1.21+、Vercel CLI（可选）、Docker/PostgreSQL（可选）
- 本地开发：
  1) 准备环境：`cp .env.example .env.local`（若存在示例文件）
  2) 配置数据库：设置 `POSTGRES_DSN=...` 或 `SUPABASE_URL`/`SUPABASE_SERVICE_KEY`
  3) 安装依赖：`go mod tidy`
  4) 启动：`vercel dev --listen 3000` 或 `make dev`
  5) 健康检查：GET `http://localhost:3000/`
- PostgreSQL 初始化：`go run scripts/setup_db.go`（或参考 SQL 脚本）
- 调试端点（开发环境）：
  - `GET /debug/db-pool`：连接池/优化器状态
  - `GET /debug/db-schema`：数据库字段变更信息
  - `GET /debug/env-check`：关键环境变量检测

## 部署到 Vercel

- `vercel.json` rewrite 至 `api/index.go`，函数 `maxDuration=30s`，应用路由超时 25s
- Vercel 文件系统不可持久化，必须配置外部数据库（推荐 Supabase；PostgreSQL 在部分区域可能存在 IPv6 问题）
- 关键配置：`ENVIRONMENT=production`、`JWT_SECRET`、数据库/鉴权变量

## 配置项说明

- 基础：`ENVIRONMENT`、`PORT`、`DEBUG`
- JWT：`JWT_SECRET`
- 数据库：`POSTGRES_DSN` 或 `SUPABASE_URL` + `SUPABASE_SERVICE_KEY`
- OAuth：`GOOGLE_CLIENT_ID`、`GOOGLE_CLIENT_SECRET`、`GITHUB_CLIENT_ID`、`GITHUB_CLIENT_SECRET`、`OAUTH_REDIRECT_URI`、`BASE_URL`
- CORS：`ALLOWED_ORIGINS`（逗号分隔，或 `*`）
- Paddle（可选）：`PADDLE_API_KEY`、`PADDLE_ENVIRONMENT`、`PADDLE_WEBHOOK_SECRET`、`PADDLE_PRO_PRICE_ID`、`PADDLE_POWER_PRICE_ID`
- 前端回调（可选）：`FRONTEND_CALLBACK_URL`

## 数据库选择策略

- Vercel 环境：优先 Supabase → 其次 PostgreSQL → 未配置则报错
- 非 Vercel 环境：优先 PostgreSQL → 其次 Supabase → 未配置则报错

## 代码风格与约定

- 使用 Go 1.21；`go fmt` 或 `make fmt` 统一格式化
- 命名与可读性：导出类型/方法使用驼峰，私有使用小写；包/文件命名一致
- 日志：避免打印敏感信息（Token/Secret/DSN），必要时脱敏；结构化日志详见 `middleware/logging.go`
- 中间件顺序：RequestID → RealIP → Normalize → Logger → Recover → Timeout → Compress → CORS → 业务路由
- 连接复用：`pkg/database/pool.go` 与 `vercel_optimizer.go`

## 迁移到外部数据库（从 local 模式）

针对历史使用本地文件数据库（local 模式）的项目：

1) 删除 `.env.local`/`.env.production` 中所有 `USE_LOCAL_DB=` 行
2) 选择一种外部数据源并配置：
   - PostgreSQL：设 `POSTGRES_DSN=postgres://user:pass@host:5432/db?sslmode=disable`（或 `require`）
   - Supabase：设 `SUPABASE_URL` 与 `SUPABASE_SERVICE_KEY`（或使用 Supabase 提供的 PostgreSQL DSN）
3) 初始化数据库：`go run scripts/setup_db.go`（或执行项目提供的 SQL 脚本）
4) 数据迁移：旧版 local 数据（若存在）位于 `./data/` 下 JSON 文件；当前版本不提供自动迁移脚本，可按 `pkg/models` 结构进行一次性导入（优先 users/snapshots/collections/items 等表）
5) 验证：本地 `curl http://localhost:3000/`；开发模式可用 `GET /debug/db-pool` 检查连接状态；Vercel 上建议优先 Supabase

---
如需扩展规范或新增开发约定，请提交 PR 同步更新本文件。

