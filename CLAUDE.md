[根目录](../../CLAUDE.md) > **backend-refactor**

# Backend Refactor

## 模块职责

提供Vercel优化的Go后端API服务，采用单体路由模式，支持多种数据库选项，提供高性能的云同步和订阅管理功能。

## 入口与启动

- **主入口**: `api/index.go` - Vercel函数入口点
- **路由配置**: `api/index.go` - Chi路由器配置
- **构建配置**: `vercel.json` - Vercel部署配置

## 对外接口

### Vercel函数入口
```go
func Handler(w http.ResponseWriter, r *http.Request)
```

### 主要路由组
```go
// 健康检查
router.Get("/", authHandler.HealthCheck)

// 认证路由
router.Route("/api/auth", func(r chi.Router) {
    r.Post("/register", authHandler.Register)
    r.Post("/login", authHandler.Login)
    r.Post("/refresh", authHandler.RefreshToken)
    r.Post("/logout", authHandler.Logout)
    r.Post("/oauth/google", authHandler.GoogleOAuth)
    r.Post("/oauth/github", authHandler.GitHubOAuth)
})

// OAuth回调路由
router.Route("/api/oauth", func(r chi.Router) {
    r.Get("/callback", authHandler.OAuthCallback)
    r.Get("/google/callback", authHandler.GoogleOAuthCallback)
    r.Get("/github/callback", authHandler.GitHubOAuthCallback)
    r.Get("/extension/callback", authHandler.ExtensionOAuthCallback)
})

// 需要认证的路由
router.Route("/api", func(r chi.Router) {
    r.Use(customMiddleware.AuthMiddleware(cfg))
    
    // 快照管理
    r.Route("/snapshots", func(r chi.Router) {
        r.Get("/", snapshotHandler.ListSnapshots)
        r.Post("/", snapshotHandler.CreateSnapshot)
        r.Get("/{name}", snapshotHandler.GetSnapshot)
        r.Put("/{name}", snapshotHandler.UpdateSnapshot)
        r.Delete("/{name}", snapshotHandler.DeleteSnapshot)
    })
    
    // Webhook路由
    r.Route("/webhooks", func(r chi.Router) {
        r.Post("/paddle", webhookHandler.HandlePaddleWebhook)
    })
})
```

## 关键依赖与配置

### Go模块依赖
- `github.com/go-chi/chi/v5` - HTTP路由器
- `github.com/go-chi/cors` - CORS中间件
- `github.com/golang-jwt/jwt/v5` - JWT认证
- `github.com/google/uuid` - UUID生成
- `github.com/lib/pq` - PostgreSQL驱动

### 环境变量配置
```go
type Config struct {
    UseLocalDB        bool   `env:"USE_LOCAL_DB" envDefault:"true"`
    PostgresDSN       string `env:"POSTGRES_DSN"`
    SupabaseURL       string `env:"SUPABASE_URL"`
    SupabaseKey       string `env:"SUPABASE_SERVICE_KEY"`
    JWTSecret         string `env:"JWT_SECRET"`
    GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
    GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
    GitHubClientID     string `env:"GITHUB_CLIENT_ID"`
    GitHubClientSecret string `env:"GITHUB_CLIENT_SECRET"`
    Debug             bool   `env:"DEBUG" envDefault:"false"`
}
```

### 数据库选项
- **Local File DB**: `USE_LOCAL_DB=true`（开发环境默认）
- **PostgreSQL**: 配置 `POSTGRES_DSN`
- **Supabase**: 配置 `SUPABASE_URL` 和 `SUPABASE_SERVICE_KEY`

## 数据模型

### 用户模型
```go
type User struct {
    ID           string    `json:"id"`
    Email        string    `json:"email"`
    Name         string    `json:"name"`
    Avatar       string    `json:"avatar"`
    Provider     string    `json:"provider"`
    Tier         string    `json:"tier"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}
```

### 标签模型
```go
type Tab struct {
    ID           string            `json:"id"`
    Title        string            `json:"title"`
    URL          string            `json:"url"`
    FavIconURL   string            `json:"fav_icon_url"`
    Domain       string            `json:"domain"`
    Tags         []string          `json:"tags"`
    Metadata     map[string]string `json:"metadata"`
    CreatedAt    time.Time         `json:"created_at"`
    UpdatedAt    time.Time         `json:"updated_at"`
}
```

### 订阅模型
```go
type Subscription struct {
    ID            string    `json:"id"`
    UserID        string    `json:"user_id"`
    PlanID        string    `json:"plan_id"`
    Status        string    `json:"status"`
    CurrentPeriodStart time.Time `json:"current_period_start"`
    CurrentPeriodEnd   time.Time `json:"current_period_end"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}
```

### 快照模型
```go
type Snapshot struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    Name      string    `json:"name"`
    Data      string    `json:"data"`  // JSON字符串
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

## 测试与质量

### 质量工具
- **Go Modules**: 依赖管理
- **Chi Router**: 路由管理
- **JWT**: 认证机制
- **CORS**: 跨域支持

### 测试策略
- 当前未配置单元测试
- 建议添加API端点的单元测试
- 建议添加数据库操作的测试
- 建议添加认证中间件的测试
- 建议添加OAuth流程的测试

## 常见问题 (FAQ)

### Q: 如何选择数据库类型？
A: 开发环境使用Local File DB，生产环境使用PostgreSQL或Supabase。

### Q: 如何处理Vercel的环境限制？
A: 使用Vercel优化器管理数据库连接，避免连接泄漏，设置适当的超时时间。

### Q: 如何实现OAuth认证？
A: 使用Google和GitHub OAuth提供商，配置正确的回调URL和客户端密钥。

### Q: 如何处理JWT认证？
A: 使用中间件验证JWT令牌，实现令牌刷新机制。

### Q: 如何部署到Vercel？
A: 使用 `vercel.json` 配置文件，设置正确的环境变量和函数配置。

## 相关文件清单

### 核心文件
- `api/index.go` - 主入口和路由配置
- `vercel.json` - Vercel部署配置
- `go.mod` - Go模块配置
- `go.sum` - 依赖校验和

### 配置包
- `pkg/config/config.go` - 配置管理
- `pkg/config/load_config.go` - 配置加载

### 数据库包
- `pkg/database/interface.go` - 数据库接口
- `pkg/database/local.go` - 本地文件数据库
- `pkg/database/postgres.go` - PostgreSQL数据库
- `pkg/database/supabase.go` - Supabase数据库
- `pkg/database/pool.go` - 连接池管理
- `pkg/database/vercel_optimizer.go` - Vercel优化器

### 中间件包
- `pkg/middleware/auth.go` - 认证中间件
- `pkg/middleware/cors.go` - CORS中间件
- `pkg/middleware/logging.go` - 日志中间件
- `pkg/middleware/recovery.go` - 恢复中间件
- `pkg/middleware/validation.go` - 验证中间件

### 处理器包
- `pkg/handlers/auth.go` - 认证处理器
- `pkg/handlers/snapshots.go` - 快照处理器
- `pkg/handlers/webhook.go` - Webhook处理器

### 模型包
- `pkg/models/user.go` - 用户模型
- `pkg/models/tab.go` - 标签模型
- `pkg/models/subscription.go` - 订阅模型

### 工具包
- `pkg/utils/response.go` - 响应工具
- `pkg/utils/jwt.go` - JWT工具

### 脚本
- `scripts/setup_db.go` - 数据库设置脚本

## 变更记录 (Changelog)

### 2025-08-29
- 创建模块文档
- 添加Vercel优化和单体路由模式说明
- 完善数据库选项和配置说明
- 添加OAuth和JWT认证流程说明