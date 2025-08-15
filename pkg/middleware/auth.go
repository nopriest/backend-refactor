package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tab-sync-backend-refactor/pkg/config"
	"tab-sync-backend-refactor/pkg/models"
	"tab-sync-backend-refactor/pkg/utils"

	"github.com/golang-jwt/jwt/v5"
)

// ContextKey 用于在context中存储用户信息的键
type ContextKey string

const (
	UserContextKey ContextKey = "user"
)

// AuthMiddleware JWT认证中间件
func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("🔍 Auth middleware: Processing request to %s\n", r.URL.Path)

			// 从Authorization头获取token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				fmt.Printf("❌ Auth middleware: Missing authorization header\n")
				utils.WriteErrorResponse(w, http.StatusUnauthorized, "Missing authorization header")
				return
			}

			// 检查Bearer前缀
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				fmt.Printf("❌ Auth middleware: Invalid authorization header format\n")
				utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid authorization header format")
				return
			}

			fmt.Printf("🔍 Auth middleware: Token received (length: %d)\n", len(tokenString))

			// 解析和验证JWT token
			token, err := jwt.ParseWithClaims(tokenString, &models.TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
				// 验证签名方法
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(cfg.JWTSecret), nil
			})

			if err != nil {
				fmt.Printf("❌ Auth middleware: Token parsing failed: %v\n", err)
				utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid token: "+err.Error())
				return
			}

			// 检查token是否有效
			if !token.Valid {
				fmt.Printf("❌ Auth middleware: Token is not valid\n")
				utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			// 获取claims
			claims, ok := token.Claims.(*models.TokenClaims)
			if !ok {
				fmt.Printf("❌ Auth middleware: Invalid token claims\n")
				utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid token claims")
				return
			}

			fmt.Printf("🔍 Auth middleware: Claims parsed - UserID: %s, Email: %s, Type: %s\n", claims.UserID, claims.Email, claims.Type)

			// 检查token类型（只接受access token）
			if claims.Type != "access" {
				fmt.Printf("❌ Auth middleware: Invalid token type: %s\n", claims.Type)
				utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid token type")
				return
			}

			// 检查token是否过期
			if time.Now().Unix() > claims.Exp {
				fmt.Printf("❌ Auth middleware: Token expired. Current: %d, Exp: %d\n", time.Now().Unix(), claims.Exp)
				utils.WriteErrorResponse(w, http.StatusUnauthorized, "Token expired")
				return
			}

			// 创建用户对象并添加到context
			user := &models.User{
				ID:    claims.UserID,
				Email: claims.Email,
			}

			fmt.Printf("✅ Auth middleware: Authentication successful for user %s (%s)\n", user.ID, user.Email)

			// 将用户信息添加到请求context中
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthMiddleware 可选的认证中间件（不强制要求认证）
func OptionalAuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 尝试获取Authorization头
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// 没有认证头，继续处理请求
				next.ServeHTTP(w, r)
				return
			}

			// 检查Bearer前缀
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				// 格式不正确，继续处理请求（不返回错误）
				next.ServeHTTP(w, r)
				return
			}

			// 尝试解析JWT token
			token, err := jwt.ParseWithClaims(tokenString, &models.TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(cfg.JWTSecret), nil
			})

			// 如果解析成功且token有效，则添加用户信息到context
			if err == nil && token.Valid {
				if claims, ok := token.Claims.(*models.TokenClaims); ok {
					if claims.Type == "access" && time.Now().Unix() <= claims.Exp {
						user := &models.User{
							ID:    claims.UserID,
							Email: claims.Email,
						}
						ctx := context.WithValue(r.Context(), UserContextKey, user)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}

			// 如果token无效或解析失败，继续处理请求（不返回错误）
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserFromContext 从context中获取用户信息
func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*models.User)
	return user, ok
}

// RequireUser 要求用户必须已认证的辅助函数
func RequireUser(ctx context.Context) (*models.User, error) {
	user, ok := GetUserFromContext(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("user not authenticated")
	}
	return user, nil
}
