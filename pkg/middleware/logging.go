package middleware

import (
	"fmt"
	"net/http"
	"time"

	"tab-sync-backend-refactor/pkg/config"

	"github.com/go-chi/chi/v5/middleware"
)

// Logger 创建日志中间件
func Logger(cfg *config.Config) func(http.Handler) http.Handler {
	// 统一使用Chi的默认日志中间件
	return middleware.Logger
}

// CustomLogger 自定义日志中间件
func CustomLogger(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 创建响应写入器包装器来捕获状态码
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// 处理请求
			next.ServeHTTP(ww, r)

			// 计算处理时间
			duration := time.Since(start)

			// 获取用户信息（如果有）
			userInfo := "anonymous"
			if user, ok := GetUserFromContext(r.Context()); ok && user != nil {
				userInfo = user.Email
			}

			// 根据环境选择日志格式
			if cfg.IsProduction() {
				// 生产环境：结构化日志
				logProductionRequest(r, ww, duration, userInfo)
			} else {
				// 开发环境：彩色日志
				logDevelopmentRequest(r, ww, duration, userInfo)
			}
		})
	}
}

// logProductionRequest 生产环境日志格式
func logProductionRequest(r *http.Request, ww middleware.WrapResponseWriter, duration time.Duration, userInfo string) {
	fmt.Printf(`{"time":"%s","method":"%s","path":"%s","status":%d,"duration":"%s","user":"%s","ip":"%s","user_agent":"%s"}`+"\n",
		time.Now().Format(time.RFC3339),
		r.Method,
		r.URL.Path,
		ww.Status(),
		duration,
		userInfo,
		getClientIP(r),
		r.UserAgent(),
	)
}

// logDevelopmentRequest 开发环境日志格式
func logDevelopmentRequest(r *http.Request, ww middleware.WrapResponseWriter, duration time.Duration, userInfo string) {
	// 根据状态码选择颜色
	statusColor := getStatusColor(ww.Status())
	methodColor := getMethodColor(r.Method)

	fmt.Printf("%s %s %s%s%s %s%d%s %s %s %s\n",
		time.Now().Format("15:04:05"),
		methodColor+r.Method+"\033[0m",
		"\033[36m", // 青色
		r.URL.Path,
		"\033[0m",
		statusColor,
		ww.Status(),
		"\033[0m",
		duration,
		userInfo,
		getClientIP(r),
	)
}

// getClientIP 获取客户端IP地址
func getClientIP(r *http.Request) string {
	// 检查X-Forwarded-For头（代理/负载均衡器）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// 检查X-Real-IP头
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// 使用RemoteAddr
	return r.RemoteAddr
}

// getStatusColor 根据HTTP状态码返回颜色代码
func getStatusColor(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "\033[32m" // 绿色
	case status >= 300 && status < 400:
		return "\033[33m" // 黄色
	case status >= 400 && status < 500:
		return "\033[31m" // 红色
	case status >= 500:
		return "\033[35m" // 紫色
	default:
		return "\033[0m" // 默认
	}
}

// getMethodColor 根据HTTP方法返回颜色代码
func getMethodColor(method string) string {
	switch method {
	case "GET":
		return "\033[34m" // 蓝色
	case "POST":
		return "\033[32m" // 绿色
	case "PUT":
		return "\033[33m" // 黄色
	case "DELETE":
		return "\033[31m" // 红色
	case "PATCH":
		return "\033[36m" // 青色
	case "OPTIONS":
		return "\033[37m" // 白色
	default:
		return "\033[0m" // 默认
	}
}
