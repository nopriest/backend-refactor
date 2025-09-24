package handler

import (
	"fmt"
	"net/http"
	"time"

	"tab-sync-backend-refactor/pkg/config"
	"tab-sync-backend-refactor/pkg/database"
	"tab-sync-backend-refactor/pkg/handlers"
	customMiddleware "tab-sync-backend-refactor/pkg/middleware"
	"tab-sync-backend-refactor/pkg/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Handler 是Vercel函数的入口点
// 这个函数实现了"单体路由模式"，将所有API端点集中在一个Chi路由器中管理
func Handler(w http.ResponseWriter, r *http.Request) {
	// 加载配置
	cfg := config.GetCached()

	// 验证配置
	if err := cfg.Validate(); err != nil {
		utils.WriteInternalServerErrorResponse(w, "Configuration error: "+err.Error())
		return
	}

	// 获取优化的数据库连接（自动适配Vercel环境）
	db := database.GetOptimizedDatabase(database.DatabaseConfig{
		UseLocalDB:  cfg.UseLocalDB,
		PostgresDSN: cfg.PostgresDSN,
		SupabaseURL: cfg.SupabaseURL,
		SupabaseKey: cfg.SupabaseKey,
		Debug:       cfg.Debug,
	})
	// 注意：连接由优化器管理，无需手动关闭

	// 创建Chi路由器
	router := chi.NewRouter()

	// 设置全局中间件
	setupMiddleware(router, cfg)

	// 设置路由
	setupRoutes(router, cfg, db)

	// 将请求传递给Chi路由器处理
	router.ServeHTTP(w, r)
}

// setupMiddleware 设置全局中间件
func setupMiddleware(router *chi.Mux, cfg *config.Config) {
	// 基础中间件
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	// Normalize path and restore scheme/host before logging and routing
	router.Use(customMiddleware.Normalize())
	router.Use(customMiddleware.Logger(cfg))
	router.Use(middleware.Recoverer)

	// CORS中间件
	router.Use(customMiddleware.CORS(cfg))

	// 超时中间件（Vercel函数有时间限制）
	router.Use(middleware.Timeout(25 * time.Second)) // 留5秒缓冲

	// 压缩中间件
	router.Use(middleware.Compress(5))

	// 开发环境额外中间件
	if cfg.IsDevelopment() {
		router.Use(middleware.Heartbeat("/ping"))
	}
}

// setupRoutes 设置所有API路由
func setupRoutes(router *chi.Mux, cfg *config.Config, db database.DatabaseInterface) {
	// 创建处理器
	authHandler := handlers.NewAuthHandler(cfg, db)
	snapshotHandler := handlers.NewSnapshotHandler(cfg, db)
	webhookHandler := handlers.NewWebhookHandler(cfg, db)

	// 健康检查端点
	router.Get("/", authHandler.HealthCheck)

	// 数据库连接池状态端点（调试用）
	if cfg.IsDevelopment() {
		router.Get("/debug/db-pool", func(w http.ResponseWriter, r *http.Request) {
			var stats map[string]interface{}

			if database.IsVercelEnvironment() {
				// Vercel环境显示优化器状态
				optimizer := database.GetVercelOptimizer()
				stats = optimizer.GetStats()
				stats["optimizer_type"] = "vercel"
			} else {
				// 非Vercel环境显示连接池状态
				stats = database.GetConnectionStats()
				stats["optimizer_type"] = "standard"
			}

			utils.WriteSuccessResponse(w, stats)
		})

		// 数据库表结构检查端点
		router.Get("/debug/db-schema", func(w http.ResponseWriter, r *http.Request) {
			utils.WriteSuccessResponse(w, map[string]interface{}{
				"message":      "Database schema updated successfully",
				"fields_added": []string{"name", "avatar", "provider"},
				"note":         "OAuth fields are now available in the users table",
			})
		})

		// 环境变量检查端点
		router.Get("/debug/env-check", func(w http.ResponseWriter, r *http.Request) {
			envStatus := map[string]interface{}{
				"google_client_id":     cfg.GoogleClientID != "",
				"google_client_secret": cfg.GoogleClientSecret != "",
				"oauth_redirect_uri":   cfg.OAuthRedirectURI,
				"jwt_secret":           cfg.JWTSecret != "",
			}
			utils.WriteSuccessResponse(w, envStatus)
		})
	}

	// API路由组
	router.Route("/api", func(r chi.Router) {
		// 公开路由（不需要认证）
		r.Route("/auth", func(r chi.Router) {
			// 认证相关路由
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.RefreshToken)
			r.Post("/logout", authHandler.Logout)

			// OAuth路由
			r.Post("/oauth/google", authHandler.GoogleOAuth)
			r.Post("/oauth/github", authHandler.GitHubOAuth)

			// 订阅状态检查（支持现有的check_subscription请求）
			r.Post("/", authHandler.CheckSubscription)

			// 交换会话码（公开路由，不需要认证）
			r.Post("/exchange-session", authHandler.ExchangeSession)
		})

		// OAuth回调路由（在API路由组内）
		r.Route("/oauth", func(r chi.Router) {
			r.Get("/callback", authHandler.OAuthCallback)
			r.Get("/google/callback", authHandler.GoogleOAuthCallback)
			r.Get("/github/callback", authHandler.GitHubOAuthCallback)
			// 扩展专用回调路由
			r.Get("/extension/callback", authHandler.ExtensionOAuthCallback)
		})

		// 需要认证的路由
		// 需要认证的路由
		r.Group(func(r chi.Router) {
			// 应用认证中间件
			r.Use(customMiddleware.AuthMiddleware(cfg))

			// 认证相关的需要认证的路由（使用不同的路径避免冲突）
			r.Route("/session", func(r chi.Router) {
				// 生成定价会话（需要认证）
				r.Post("/generate-pricing", authHandler.GeneratePricingSession)
			})

			// 用户相关路由
			r.Route("/user", func(r chi.Router) {
				r.Get("/profile", handleNotImplemented)
				r.Put("/profile", handleNotImplemented)
				r.Delete("/account", handleNotImplemented)
			})

			// 快照管理路由
			// Organizations & Spaces
			orgsHandler := handlers.NewOrgsHandler(cfg, db)
			r.Route("/orgs", func(r chi.Router) {
				r.Get("/", orgsHandler.ListMyOrganizations)
				r.Post("/", orgsHandler.CreateOrganization)
				r.Get("/members", orgsHandler.ListMembers) // expects ?org_id=
				r.Get("/spaces", orgsHandler.ListSpaces)   // expects ?org_id=
				r.Post("/spaces", orgsHandler.CreateSpace)
				r.Post("/invite", orgsHandler.InviteMember)
				r.Put("/spaces/permissions", orgsHandler.SetSpacePermission)
			})

			// Invitations
			r.Route("/invitations", func(r chi.Router) {
				r.Get("/my", orgsHandler.ListMyInvitations)
				r.Post("/accept", orgsHandler.AcceptInvitation)
			})

			r.Route("/snapshots", func(r chi.Router) {
				r.Get("/", snapshotHandler.ListSnapshots)           // 列出快照
				r.Post("/", snapshotHandler.CreateSnapshot)         // 创建快照
				r.Get("/{name}", snapshotHandler.GetSnapshot)       // 获取快照
				r.Put("/{name}", snapshotHandler.UpdateSnapshot)    // 更新快照
				r.Delete("/{name}", snapshotHandler.DeleteSnapshot) // 删除快照
			})

			// 订阅管理路由
			r.Route("/subscription", func(r chi.Router) {
				r.Get("/", handleNotImplemented)    // 获取订阅状态
				r.Post("/", handleNotImplemented)   // 创建订阅
				r.Put("/", handleNotImplemented)    // 更新订阅
				r.Delete("/", handleNotImplemented) // 取消订阅
			})

			// AI功能路由
			r.Route("/ai", func(r chi.Router) {
				r.Get("/credits", handleNotImplemented)   // 获取AI积分
				r.Post("/generate", handleNotImplemented) // AI生成内容
			})
		})

		// Webhook路由（不需要认证，但需要验证签名）
		r.Route("/webhooks", func(r chi.Router) {
			r.Post("/paddle", webhookHandler.HandlePaddleWebhook) // Paddle支付回调
		})
	})

	// 404处理
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		utils.WriteNotFoundResponse(w, fmt.Sprintf("Route not found: %s %s", r.Method, r.URL.Path))
	})

	// 405处理
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		utils.WriteErrorResponseWithCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED",
			fmt.Sprintf("Method %s not allowed for %s", r.Method, r.URL.Path), "")
	})
}

// handleNotImplemented 临时处理器，用于标记未实现的端点
func handleNotImplemented(w http.ResponseWriter, r *http.Request) {
	utils.WriteErrorResponseWithCode(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
		"This endpoint is not yet implemented", "")
}
