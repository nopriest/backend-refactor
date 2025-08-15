package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/cors"
	"tab-sync-backend-refactor/pkg/config"
)

// CORS 创建CORS中间件
func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	// 配置CORS选项
	corsOptions := cors.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodPatch,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-CSRF-Token",
			"X-Requested-With",
			"Cache-Control",
		},
		ExposedHeaders: []string{
			"Link",
			"X-Total-Count",
		},
		AllowCredentials: true,
		MaxAge:           300, // 5分钟
	}

	// 开发环境允许所有来源
	if cfg.IsDevelopment() {
		corsOptions.AllowedOrigins = []string{"*"}
		corsOptions.AllowCredentials = false // 当AllowedOrigins为*时，不能设置AllowCredentials为true
	}

	// 如果配置了特定的允许来源，则使用配置的值
	if len(cfg.AllowedOrigins) > 0 && cfg.AllowedOrigins[0] != "*" {
		corsOptions.AllowedOrigins = cfg.AllowedOrigins
		corsOptions.AllowCredentials = true
	}

	return cors.Handler(corsOptions)
}

// CustomCORS 自定义CORS中间件（如果需要更细粒度的控制）
func CustomCORS(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// 检查是否允许该来源
			if isOriginAllowed(origin, cfg.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if cfg.IsDevelopment() {
				// 开发环境允许所有来源
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Requested-With, Cache-Control")
			w.Header().Set("Access-Control-Expose-Headers", "Link, X-Total-Count")
			w.Header().Set("Access-Control-Max-Age", "300")

			// 只有在非通配符来源时才允许凭据
			if origin != "" && !contains(cfg.AllowedOrigins, "*") {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// 处理预检请求
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isOriginAllowed 检查来源是否被允许
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return false
	}

	// 检查通配符
	if contains(allowedOrigins, "*") {
		return true
	}

	// 检查精确匹配
	if contains(allowedOrigins, origin) {
		return true
	}

	// 检查模式匹配（简单的通配符支持）
	for _, allowed := range allowedOrigins {
		if strings.HasSuffix(allowed, "*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(origin, prefix) {
				return true
			}
		}
	}

	return false
}

// contains 检查切片是否包含指定的字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
