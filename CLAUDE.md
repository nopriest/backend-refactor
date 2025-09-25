[返回目录](../../CLAUDE.md) > **backend-refactor**

# Backend Refactor

提供 Vercel 优化的 Go 后端，采用“单路由入口”模式，支持 PostgreSQL 与 Supabase 两种外部数据库。

## 主要组件

- 入口：`api/index.go`（Vercel Serverless 入口 + Chi 路由）
- 配置：`pkg/config/config.go`（环境变量加载与校验）
- 数据库：`pkg/database/`（接口 + postgres/supabase 实现 + 连接池与 Vercel 优化）
- 中间件：`pkg/middleware/`（鉴权、CORS、日志、恢复、Normalize 等）
- 业务处理：`pkg/handlers/`（auth、snapshots、webhook、collections、orgs）
- 工具：`pkg/utils/`（响应、JWT）

## 核心路由（示例）

```go
// 根路由
router.Get("/", authHandler.HealthCheck)

// Auth
router.Route("/api/auth", func(r chi.Router) {
    r.Post("/register", authHandler.Register)
    r.Post("/login", authHandler.Login)
    r.Post("/refresh", authHandler.RefreshToken)
    r.Post("/logout", authHandler.Logout)
    r.Post("/oauth/google", authHandler.GoogleOAuth)
    r.Post("/oauth/github", authHandler.GitHubOAuth)
})

// OAuth 回调
router.Route("/api/oauth", func(r chi.Router) {
    r.Get("/callback", authHandler.OAuthCallback)
    r.Get("/google/callback", authHandler.GoogleOAuthCallback)
    r.Get("/github/callback", authHandler.GitHubOAuthCallback)
    r.Get("/extension/callback", authHandler.ExtensionOAuthCallback)
})
```

## 依赖

- `github.com/go-chi/chi/v5`（路由）
- `github.com/go-chi/cors`（CORS）
- `github.com/golang-jwt/jwt/v5`（JWT）
- `github.com/lib/pq`（PostgreSQL）

## 配置结构（简化）

```go
type Config struct {
    Environment string
    Port        string

    // Database (local 模式已移除)
    PostgresDSN string
    SupabaseURL string
    SupabaseKey string

    // Auth
    JWTSecret string

    // OAuth
    GoogleClientID     string
    GoogleClientSecret string
    GitHubClientID     string
    GitHubClientSecret string
    OAuthRedirectURI   string
    BaseURL            string

    Debug bool
}
```

## 数据库选择策略

- Vercel 环境：优先 Supabase → 其次 PostgreSQL → 未配置则报错
- 非 Vercel 环境：优先 PostgreSQL → 其次 Supabase → 未配置则报错

## 文件清单（部分）

- `pkg/database/interface.go` – 数据库接口与选择
- `pkg/database/postgres.go` – PostgreSQL 实现
- `pkg/database/supabase.go` – Supabase 实现
- `pkg/database/pool.go` – 连接池管理
- `pkg/database/vercel_optimizer.go` – Vercel 连接优化

## FAQ

### Q: 如何选择数据库类型？
A: 仅支持外部数据库。请设置 `POSTGRES_DSN` 或 `SUPABASE_URL` + `SUPABASE_SERVICE_KEY`。

### Q: 在 Vercel 上连接数据库有什么注意事项？
A: 推荐 Supabase（REST/直连皆可）；PostgreSQL 在部分区域可能遇到 IPv6 问题。

## 迁移到外部数据库（从 local 模式）

1) 从 `.env.*` 中移除所有 `USE_LOCAL_DB` 相关条目
2) 配置 `POSTGRES_DSN` 或 `SUPABASE_URL` + `SUPABASE_SERVICE_KEY`
3) 初始化数据库：`go run scripts/setup_db.go`（或执行 SQL 脚本）
4) 旧版 local 数据（若存在）位于 `./data/` JSON 文件；当前版本不提供自动迁移，请按 `pkg/models` 结构手动导入关键表

## 变更记录

### 2025-09-24
- 移除 local 文件数据库支持；统一为外部数据库（PostgreSQL/Supabase）
- 更新配置与选择逻辑，完善文档与迁移说明

