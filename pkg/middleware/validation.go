package middleware

import (
	"net/http"
	"strings"

	"tab-sync-backend-refactor/pkg/utils"
)

// ContentTypeJSON 验证请求Content-Type为application/json
func ContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 只对POST、PUT、PATCH请求验证Content-Type
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			contentType := r.Header.Get("Content-Type")
			if contentType == "" {
				utils.WriteBadRequestResponse(w, "Content-Type header is required")
				return
			}

			// 检查是否为application/json（忽略charset等参数）
			if !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
				utils.WriteBadRequestResponse(w, "Content-Type must be application/json")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// MaxBodySize 限制请求体大小
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 限制请求体大小
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireUserAgent 要求User-Agent头
func RequireUserAgent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		if userAgent == "" {
			utils.WriteBadRequestResponse(w, "User-Agent header is required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ValidateAPIKey 验证API密钥（如果需要）
func ValidateAPIKey(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				// 如果没有配置API密钥，跳过验证
				next.ServeHTTP(w, r)
				return
			}

			// 从头部或查询参数获取API密钥
			providedKey := r.Header.Get("X-API-Key")
			if providedKey == "" {
				providedKey = r.URL.Query().Get("api_key")
			}

			if providedKey != apiKey {
				utils.WriteUnauthorizedResponse(w, "Invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByIP 简单的IP限流（内存版本，适合单实例）
func RateLimitByIP(requestsPerMinute int) func(http.Handler) http.Handler {
	// 这里可以实现一个简单的内存限流器
	// 生产环境建议使用Redis等外部存储
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 简化实现：暂时跳过限流
			// TODO: 实现真正的限流逻辑
			next.ServeHTTP(w, r)
		})
	}
}
