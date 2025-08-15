package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"tab-sync-backend-refactor/pkg/config"
	"tab-sync-backend-refactor/pkg/utils"
)

// Recovery 恢复中间件，处理panic并返回友好的错误信息
func Recovery(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// 记录panic信息
					stack := debug.Stack()
					
					if cfg.IsDevelopment() {
						// 开发环境：显示详细错误信息
						fmt.Printf("❌ PANIC: %v\n", err)
						fmt.Printf("📍 Stack trace:\n%s\n", stack)
						
						utils.WriteErrorResponseWithCode(w, http.StatusInternalServerError, 
							"INTERNAL_SERVER_ERROR", 
							fmt.Sprintf("Internal server error: %v", err),
							string(stack))
					} else {
						// 生产环境：隐藏详细错误信息
						fmt.Printf("❌ PANIC: %v\n", err)
						fmt.Printf("📍 Stack trace:\n%s\n", stack)
						
						utils.WriteInternalServerErrorResponse(w, "Internal server error occurred")
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// ErrorHandler 统一错误处理中间件
func ErrorHandler(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 创建一个自定义的ResponseWriter来捕获错误
			ew := &errorResponseWriter{
				ResponseWriter: w,
				config:         cfg,
			}

			next.ServeHTTP(ew, r)
		})
	}
}

// errorResponseWriter 包装ResponseWriter以捕获错误
type errorResponseWriter struct {
	http.ResponseWriter
	config *config.Config
	written bool
}

func (ew *errorResponseWriter) WriteHeader(statusCode int) {
	if ew.written {
		return
	}
	ew.written = true

	// 如果是错误状态码，记录日志
	if statusCode >= 400 {
		if ew.config.IsDevelopment() {
			fmt.Printf("⚠️ HTTP Error: %d\n", statusCode)
		}
	}

	ew.ResponseWriter.WriteHeader(statusCode)
}

func (ew *errorResponseWriter) Write(data []byte) (int, error) {
	if !ew.written {
		ew.WriteHeader(http.StatusOK)
	}
	return ew.ResponseWriter.Write(data)
}
