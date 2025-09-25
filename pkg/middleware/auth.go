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

// ContextKey 在 context 中存储用户信息的键
type ContextKey string

const (
    UserContextKey ContextKey = "user"
)

// AuthMiddleware JWT 鉴权中间件
// 生产环境默认不打印调试日志，避免噪音；当 cfg.Debug=true 时输出详细过程。
func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            debugf := func(format string, a ...interface{}) {
                if cfg != nil && cfg.Debug {
                    fmt.Printf(format, a...)
                }
            }
            debugf("Auth middleware: Processing request to %s\n", r.URL.Path)

            // 从 Authorization 头或 Cookie 获取 token
            var tokenString string
            if authHeader := r.Header.Get("Authorization"); authHeader != "" {
                if strings.HasPrefix(authHeader, "Bearer ") {
                    tokenString = strings.TrimPrefix(authHeader, "Bearer ")
                } else {
                    debugf("Auth middleware: Invalid authorization header format\n")
                    utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid authorization header format")
                    return
                }
            } else if c, err := r.Cookie("access_token"); err == nil && c != nil && c.Value != "" {
                tokenString = c.Value
                debugf("Auth middleware: Using token from cookie\n")
            } else {
                debugf("Auth middleware: Missing authorization header and cookie\n")
                utils.WriteErrorResponse(w, http.StatusUnauthorized, "Missing authorization header")
                return
            }

            debugf("Auth middleware: Token received (length: %d)\n", len(tokenString))

            // 解析并验证 JWT token
            token, err := jwt.ParseWithClaims(tokenString, &models.TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
                // 验证签名算法
                if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                    return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                }
                return []byte(cfg.JWTSecret), nil
            })

            if err != nil {
                debugf("Auth middleware: Token parsing failed: %v\n", err)
                utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid token: "+err.Error())
                return
            }

            // 检查 token 是否有效
            if !token.Valid {
                debugf("Auth middleware: Token is not valid\n")
                utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid token")
                return
            }

            // 获取 claims
            claims, ok := token.Claims.(*models.TokenClaims)
            if !ok {
                debugf("Auth middleware: Invalid token claims\n")
                utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid token claims")
                return
            }

            debugf("Auth middleware: Claims parsed - UserID: %s, Email: %s, Type: %s\n", claims.UserID, claims.Email, claims.Type)

            // 仅允许 access token
            if claims.Type != "access" {
                debugf("Auth middleware: Invalid token type: %s\n", claims.Type)
                utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid token type")
                return
            }

            // 过期校验
            if time.Now().Unix() > claims.Exp {
                debugf("Auth middleware: Token expired. Current: %d, Exp: %d\n", time.Now().Unix(), claims.Exp)
                utils.WriteErrorResponse(w, http.StatusUnauthorized, "Token expired")
                return
            }

            // 将用户信息注入 context
            user := &models.User{
                ID:    claims.UserID,
                Email: claims.Email,
            }

            debugf("Auth middleware: Authentication successful for user %s (%s)\n", user.ID, user.Email)

            ctx := context.WithValue(r.Context(), UserContextKey, user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// OptionalAuthMiddleware 可选鉴权中间件（不强制要求鉴权）
func OptionalAuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 尝试获取 Authorization 头
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                next.ServeHTTP(w, r)
                return
            }

            // 解析 Bearer 前缀
            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            if tokenString == authHeader {
                next.ServeHTTP(w, r)
                return
            }

            // 尝试解析 JWT token
            token, err := jwt.ParseWithClaims(tokenString, &models.TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
                if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                    return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                }
                return []byte(cfg.JWTSecret), nil
            })

            if err == nil && token.Valid {
                if claims, ok := token.Claims.(*models.TokenClaims); ok {
                    if claims.Type == "access" && time.Now().Unix() <= claims.Exp {
                        user := &models.User{ID: claims.UserID, Email: claims.Email}
                        ctx := context.WithValue(r.Context(), UserContextKey, user)
                        next.ServeHTTP(w, r.WithContext(ctx))
                        return
                    }
                }
            }

            // 未通过可选鉴权，继续后续处理
            next.ServeHTTP(w, r)
        })
    }
}

// GetUserFromContext 从 context 获取用户信息
func GetUserFromContext(ctx context.Context) (*models.User, bool) {
    user, ok := ctx.Value(UserContextKey).(*models.User)
    return user, ok
}

// RequireUser 需要用户已通过鉴权的辅助函数
func RequireUser(ctx context.Context) (*models.User, error) {
    user, ok := GetUserFromContext(ctx)
    if !ok || user == nil {
        return nil, fmt.Errorf("user not authenticated")
    }
    return user, nil
}
