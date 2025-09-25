package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"tab-sync-backend-refactor/pkg/config"
	"tab-sync-backend-refactor/pkg/database"
	"tab-sync-backend-refactor/pkg/middleware"
	"tab-sync-backend-refactor/pkg/models"
	"tab-sync-backend-refactor/pkg/utils"
)

// AuthHandler 认证处理器
type AuthHandler struct {
    config *config.Config
    db     database.DatabaseInterface
}

// ensureDefaultOrgAndSpace ensures the user has at least one organization and a default space.
// Returns the organization ID if created or existing, otherwise empty string on failure.
func (h *AuthHandler) ensureDefaultOrgAndSpace(user *models.User) (string, error) {
    if user == nil || user.ID == "" {
        return "", fmt.Errorf("invalid user")
    }
    // Check existing orgs
    orgs, err := h.db.ListUserOrganizations(user.ID)
    if err == nil && len(orgs) > 0 {
        return orgs[0].ID, nil
    }
    // Create a default org
    displayName := user.Name
    if strings.TrimSpace(displayName) == "" {
        parts := strings.Split(user.Email, "@")
        if len(parts) > 0 { displayName = parts[0] }
    }
    org := &models.Organization{
        Name:        fmt.Sprintf("%s's Space", displayName),
        Description: "Default organization",
        OwnerID:     user.ID,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    if err := h.db.CreateOrganization(org); err != nil {
        return "", err
    }
    // Create a default space (best-effort)
    _ = h.db.CreateSpace(&models.Space{ OrganizationID: org.ID, Name: "General", Description: "Default space", IsDefault: true })
    return org.ID, nil
}

// GoogleUser Google用户信息结构
type GoogleUser struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// GoogleTokenResponse Google令牌响应结构
type GoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// OAuthRequest OAuth请求结构
type OAuthRequest struct {
	Code  string `json:"code"`
	State string `json:"state,omitempty"`
}

// GitHubUser GitHub用户信息结构
type GitHubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubTokenResponse GitHub令牌响应结构
type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(cfg *config.Config, db database.DatabaseInterface) *AuthHandler {
	return &AuthHandler{
		config: cfg,
		db:     db,
	}
}

// CheckSubscription 检查用户订阅状态（兼容现有实现）
func (h *AuthHandler) CheckSubscription(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req struct {
		Provider string `json:"provider"`
		UserID   string `json:"user_id"`
	}

	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteBadRequestResponse(w, "Invalid request body")
		return
	}

	// 检查是否为check_subscription请求
	if req.Provider != "check_subscription" {
		utils.WriteBadRequestResponse(w, "Invalid provider")
		return
	}

	if req.UserID == "" {
		utils.WriteBadRequestResponse(w, "User ID is required")
		return
	}

	// 获取用户订阅信息
	userWithSub, err := h.db.GetUserWithSubscription(req.UserID)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "User not found: "+err.Error())
		return
	}

	// 返回用户订阅信息
	utils.WriteSuccessResponse(w, map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"id":         userWithSub.ID,
			"email":      userWithSub.Email,
			"tier":       string(userWithSub.Tier),
			"created_at": userWithSub.CreatedAt,
			"updated_at": userWithSub.UpdatedAt,
		},
	})
}

// Register 用户注册
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	utils.WriteErrorResponseWithCode(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
		"User registration not yet implemented", "")
}

// Login 用户登录
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	utils.WriteErrorResponseWithCode(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
		"User login not yet implemented", "")
}

// RefreshToken 刷新令牌
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
    var req struct {
        RefreshToken string `json:"refresh_token"`
    }
    if err := utils.ParseJSONBody(r, &req); err != nil {
        utils.WriteBadRequestResponse(w, "Invalid request body")
        return
    }
    if strings.TrimSpace(req.RefreshToken) == "" {
        utils.WriteBadRequestResponse(w, "refresh_token is required")
        return
    }

    jwtService := utils.NewJWTService(h.config.JWTSecret)
    accessToken, expiresIn, err := jwtService.RefreshAccessToken(req.RefreshToken)
    if err != nil {
        utils.WriteUnauthorizedResponse(w, "Invalid or expired refresh token: "+err.Error())
        return
    }

    utils.WriteSuccessResponse(w, map[string]interface{}{
        "access_token": accessToken,
        "expires_in":   expiresIn,
    })
}

// Logout 用户登出
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	utils.WriteErrorResponseWithCode(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
		"User logout not yet implemented", "")
}

// GoogleOAuth Google OAuth登录 - 处理前端发送的授权码
func (h *AuthHandler) GoogleOAuth(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req struct {
		Code  string `json:"code"`
		State string `json:"state,omitempty"` // 可选的state参数
	}

	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteBadRequestResponse(w, "Invalid request body")
		return
	}

	if req.Code == "" {
		utils.WriteBadRequestResponse(w, "Authorization code is required")
		return
	}

	fmt.Printf("🔄 Google OAuth token exchange request received\n")
	fmt.Printf("   - Code length: %d\n", len(req.Code))
	fmt.Printf("   - State: %s\n", req.State)

	// 如果有state参数，将其添加到请求的查询参数中，以便detectClientType能够读取
	if req.State != "" {
		query := r.URL.Query()
		query.Set("state", req.State)
		r.URL.RawQuery = query.Encode()
		fmt.Printf("🔍 Added state to request URL: %s\n", r.URL.String())
	}

	// 使用现有的Google OAuth流程处理
	h.handleGoogleOAuthFlow(w, r, req.Code)
}

// GitHubOAuth GitHub OAuth登录
func (h *AuthHandler) GitHubOAuth(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req OAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestResponse(w, "Invalid request body")
		return
	}

	if req.Code == "" {
		utils.WriteBadRequestResponse(w, "Authorization code is required")
		return
	}

	// 使用GitHub OAuth流程处理
	h.handleGitHubOAuthFlow(w, r, req.Code)
}

// GeneratePricingSession 生成定价会话
func (h *AuthHandler) GeneratePricingSession(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("🔍 GeneratePricingSession: Request received\n")

	// 从认证中间件获取用户信息
	user, err := middleware.RequireUser(r.Context())
	if err != nil {
		fmt.Printf("❌ GeneratePricingSession: Authentication failed: %v\n", err)
		utils.WriteUnauthorizedResponse(w, "Authentication required")
		return
	}

	fmt.Printf("✅ GeneratePricingSession: User authenticated: %s (%s)\n", user.ID, user.Email)

	// 获取客户端IP
	clientIP := h.getClientIP(r)

	// 生成会话码（简单实现：使用UUID + 时间戳）
	sessionCode, err := h.generateSessionCode(user.ID, user.Email, user.Name, clientIP)
	if err != nil {
		utils.WriteInternalServerErrorResponse(w, "Failed to generate session: "+err.Error())
		return
	}

	// 返回响应
	response := map[string]interface{}{
		"session_code": sessionCode,
		"expires_in":   300, // 5分钟
	}

	utils.WriteSuccessResponse(w, response)
	fmt.Printf("✅ Generated pricing session for user %s\n", user.Email)
}

// OAuthCallback 通用OAuth回调处理 - 返回静态HTML页面
func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("🔄 OAuth callback received - URL: %s\n", r.URL.String())

	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// 处理预检请求
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 只允许GET请求
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// OAuth回调页面HTML（与旧项目完全一致）
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>OAuth Callback - Tab Sync</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #1a1a1a;
            color: #ffffff;
            display: flex;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
            margin: 0;
        }
        .container {
            text-align: center;
            padding: 40px;
            background: #2a2a2a;
            border-radius: 12px;
            box-shadow: 0 20px 40px rgba(0, 0, 0, 0.3);
            max-width: 400px;
        }
        .spinner {
            width: 40px;
            height: 40px;
            border: 4px solid #333;
            border-top: 4px solid #4285F4;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin: 0 auto 20px;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        .error {
            color: #ff6b6b;
            margin-top: 20px;
        }
        .success {
            color: #4CAF50;
            margin-top: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="spinner"></div>
        <h2>Processing OAuth Callback...</h2>
        <p>Please wait while we complete your authentication.</p>
        <div id="message"></div>
    </div>

    <script>
        (function() {
            const messageEl = document.getElementById('message');

            try {
                // 获取URL参数
                const urlParams = new URLSearchParams(window.location.search);
                const code = urlParams.get('code');
                const error = urlParams.get('error');
                const state = urlParams.get('state');

                console.log('OAuth callback received:', { code: code ? 'present' : 'missing', error, state });

                if (error) {
                    // OAuth错误
                    messageEl.innerHTML = '<div class="error">Authentication failed: ' + error + '</div>';

                    // 通知父窗口
                    if (window.opener) {
                        window.opener.postMessage({
                            type: 'OAUTH_ERROR',
                            error: error
                        }, '*');
                        setTimeout(() => window.close(), 2000);
                    }
                    return;
                }

                if (code) {
                    // OAuth成功，获得授权码
                    messageEl.innerHTML = '<div class="success">Authentication successful! Redirecting...</div>';

                    // 通知父窗口
                    if (window.opener) {
                        window.opener.postMessage({
                            type: 'OAUTH_SUCCESS',
                            code: code,
                            state: state
                        }, '*');
                        setTimeout(() => window.close(), 1000);
                    } else {
                        // 如果没有父窗口，显示消息
                        messageEl.innerHTML += '<div>You can close this window now.</div>';
                        setTimeout(() => {
                            try { window.close(); } catch(e) { console.log('Cannot close window'); }
                        }, 3000);
                    }
                } else {
                    // 没有code参数
                    messageEl.innerHTML = '<div class="error">No authorization code received</div>';

                    if (window.opener) {
                        window.opener.postMessage({
                            type: 'OAUTH_ERROR',
                            error: 'No authorization code received'
                        }, '*');
                        setTimeout(() => window.close(), 2000);
                    }
                }
            } catch (err) {
                console.error('OAuth callback error:', err);
                messageEl.innerHTML = '<div class="error">Processing error: ' + err.message + '</div>';

                if (window.opener) {
                    window.opener.postMessage({
                        type: 'OAUTH_ERROR',
                        error: err.message
                    }, '*');
                    setTimeout(() => window.close(), 2000);
                }
            }
        })();
    </script>
</body>
</html>`

	// 输出HTML
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))

	fmt.Printf("✅ Returned OAuth callback HTML page\n")
}

// detectOAuthProvider 检测OAuth提供商
func (h *AuthHandler) detectOAuthProvider(r *http.Request, state string) string {
	// 方法1：检查Referer头
	referer := r.Header.Get("Referer")
	if strings.Contains(referer, "accounts.google.com") {
		return "google"
	}
	if strings.Contains(referer, "github.com") {
		return "github"
	}

	// 方法2：检查state参数（如果前端编码了提供商信息）
	if strings.Contains(strings.ToLower(state), "google") {
		return "google"
	}
	if strings.Contains(strings.ToLower(state), "github") {
		return "github"
	}

	// 默认返回google
	return "google"
}

// handleGoogleOAuthFlow 处理Google OAuth流程
func (h *AuthHandler) handleGoogleOAuthFlow(w http.ResponseWriter, r *http.Request, code string) {
	// 智能检测客户端类型
	clientType := h.detectClientType(r)
	fmt.Printf("🔍 Detected client type: %s\n", clientType)

	// 1. 使用授权码换取访问令牌
	fmt.Printf("🔄 Exchanging Google authorization code for access token...\n")
    accessToken, err := h.exchangeGoogleCodeVerbose(code)
	if err != nil {
		fmt.Printf("❌ Failed to exchange Google code: %v\n", err)
		h.handleOAuthError(w, r, clientType, "token_exchange_failed", "Failed to exchange code for token: "+err.Error())
		return
	}
	fmt.Printf("✅ Successfully obtained Google access token\n")

	// 2. 使用访问令牌获取用户信息
	googleUser, err := h.getGoogleUserInfo(accessToken)
	if err != nil {
		h.handleOAuthError(w, r, clientType, "user_info_failed", "Failed to get user info: "+err.Error())
		return
	}

    // 3. 在数据库中查找或创建用户
    user, err := h.findOrCreateUser(googleUser.Email, googleUser.Name, googleUser.Picture, "google")
    if err != nil {
        h.handleOAuthError(w, r, clientType, "user_creation_failed", "Failed to create user: "+err.Error())
        return
    }

    // 3.1 首次登录引导：若无任何组织，则创建默认组织和空间
    orgID, _ := h.ensureDefaultOrgAndSpace(user)

    // 4. 生成JWT令牌
    jwtService := utils.NewJWTService(h.config.JWTSecret)
    accessTokenJWT, refreshToken, expiresIn, err := jwtService.GenerateTokenPair(user.ID, user.Email)
    if err != nil {
        h.handleOAuthError(w, r, clientType, "token_generation_failed", "Failed to generate tokens: "+err.Error())
        return
    }

    // 5. 返回响应 - 根据客户端类型选择格式
    h.handleOAuthSuccess(w, r, clientType, user, accessTokenJWT, refreshToken, expiresIn, orgID)
}

// handleGitHubOAuthFlow 处理GitHub OAuth流程
func (h *AuthHandler) handleGitHubOAuthFlow(w http.ResponseWriter, r *http.Request, code string) {
	fmt.Printf("🔄 GitHub OAuth token exchange request received\n")
	fmt.Printf("   - Code length: %d\n", len(code))

	// 1. 检测客户端类型
	clientType := h.detectClientType(r)
	fmt.Printf("🔍 Detected client type: %s\n", clientType)

	// 2. 交换授权码为访问令牌
	accessToken, err := h.exchangeGitHubCodeForToken(code)
	if err != nil {
		h.handleOAuthError(w, r, clientType, "token_exchange_failed", "Failed to exchange code for token: "+err.Error())
		return
	}

	// 3. 获取用户信息
	githubUser, err := h.getGitHubUserInfo(accessToken)
	if err != nil {
		h.handleOAuthError(w, r, clientType, "user_info_failed", "Failed to get user info: "+err.Error())
		return
	}

	// 4. 创建或更新用户
	user := &models.User{
		Email:     githubUser.Email,
		Name:      githubUser.Name,
		Avatar:    githubUser.AvatarURL,
		Provider:  "github",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 检查用户是否已存在
	existingUser, err := h.db.GetUserByEmail(user.Email)
	if err == nil && existingUser != nil {
		// 更新现有用户
		user.ID = existingUser.ID
		user.CreatedAt = existingUser.CreatedAt
		err = h.db.UpdateUser(user)
		if err != nil {
			h.handleOAuthError(w, r, clientType, "user_update_failed", "Failed to update user: "+err.Error())
			return
		}
		fmt.Printf("👤 Found existing user %s, updated OAuth info (provider: github)\n", user.Email)
	} else {
		// 创建新用户
		err = h.db.CreateUser(user)
		if err != nil {
			h.handleOAuthError(w, r, clientType, "user_creation_failed", "Failed to create user: "+err.Error())
			return
		}
		fmt.Printf("👤 Created new user %s via GitHub OAuth\n", user.Email)
	}

    // 5. 生成JWT令牌
    jwtService := utils.NewJWTService(h.config.JWTSecret)
    accessTokenJWT, refreshToken, expiresIn, err := jwtService.GenerateTokenPair(user.ID, user.Email)
    if err != nil {
        h.handleOAuthError(w, r, clientType, "token_generation_failed", "Failed to generate tokens: "+err.Error())
        return
    }

    // 5.1 首次登录引导
    orgID, _ := h.ensureDefaultOrgAndSpace(user)

    // 6. 返回响应
    h.handleOAuthSuccess(w, r, clientType, user, accessTokenJWT, refreshToken, expiresIn, orgID)
}

// GoogleOAuthCallback Google OAuth回调
func (h *AuthHandler) GoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state") // 获取state参数用于客户端类型检测
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		utils.WriteBadRequestResponse(w, "Google OAuth error: "+errorParam)
		return
	}

	if code == "" {
		utils.WriteBadRequestResponse(w, "Missing Google authorization code")
		return
	}

	fmt.Printf("🔍 Google OAuth callback - Code: %s, State: %s\n", code[:10]+"...", state)

	// 使用完整的OAuth流程处理，包括客户端类型检测
	h.handleGoogleOAuthFlow(w, r, code)
}

// GitHubOAuthCallback GitHub OAuth回调
func (h *AuthHandler) GitHubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		utils.WriteBadRequestResponse(w, "GitHub OAuth error: "+errorParam)
		return
	}

	if code == "" {
		utils.WriteBadRequestResponse(w, "Missing GitHub authorization code")
		return
	}

	// TODO: 实现GitHub OAuth令牌交换和用户信息获取
	utils.WriteSuccessResponse(w, map[string]interface{}{
		"message":  "GitHub OAuth callback received",
		"code":     code,
		"state":    state,
		"provider": "github",
		"note":     "GitHub OAuth implementation in progress",
	})
}

// HealthCheck 健康检查
func (h *AuthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// 测试数据库连接
	dbStatus := "healthy"
	if err := h.db.HealthCheck(); err != nil {
		dbStatus = "unhealthy: " + err.Error()
	}

	utils.WriteSuccessResponse(w, map[string]interface{}{
		"service":     "tab-sync-backend-refactor",
		"version":     "1.0.0",
		"environment": h.config.Environment,
		"database":    h.getDatabaseType(),
		"db_status":   dbStatus,
		"timestamp":   time.Now().Unix(),
		"status":      "healthy",
	})
}

// getDatabaseType 获取数据库类型
func (h *AuthHandler) getDatabaseType() string {
    if h.config.PostgresDSN != "" {
        return "postgresql"
    } else if h.config.SupabaseURL != "" && h.config.SupabaseKey != "" {
        return "supabase"
    }
    return "unknown"
}

// exchangeGoogleCode 使用授权码换取访问令牌
func (h *AuthHandler) exchangeGoogleCode(code string) (string, error) {
	// 构建请求参数
	data := url.Values{}
	data.Set("client_id", h.config.GoogleClientID)
	data.Set("client_secret", h.config.GoogleClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", h.config.OAuthRedirectURI)

	fmt.Printf("🔄 Exchanging code with Google OAuth:\n")
	fmt.Printf("   - Client ID: %s\n", h.config.GoogleClientID[:20]+"...")
	fmt.Printf("   - Redirect URI: %s\n", h.config.OAuthRedirectURI)
	fmt.Printf("   - Code length: %d\n", len(code))

	// 发送POST请求到Google
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("📡 Google OAuth response status: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("❌ Google OAuth error response: %s\n", string(body))
		return "", fmt.Errorf("Google token exchange failed: %s", string(body))
	}

	// 解析响应
	var tokenResp GoogleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	fmt.Printf("✅ Successfully obtained access token from Google\n")
	return tokenResp.AccessToken, nil
}

// exchangeGoogleCodeVerbose 和 exchangeGoogleCode 行为一致，但增加更详细的响应体/提示日志，便于本地排查
func (h *AuthHandler) exchangeGoogleCodeVerbose(code string) (string, error) {
    data := url.Values{}
    data.Set("client_id", h.config.GoogleClientID)
    data.Set("client_secret", h.config.GoogleClientSecret)
    data.Set("code", code)
    data.Set("grant_type", "authorization_code")
    data.Set("redirect_uri", h.config.OAuthRedirectURI)

    fmt.Printf("?? Exchanging code with Google OAuth (verbose)\n")
    if len(h.config.GoogleClientID) >= 8 {
        fmt.Printf("   - Client ID: %s...\n", h.config.GoogleClientID[:8])
    } else {
        fmt.Printf("   - Client ID: %s\n", h.config.GoogleClientID)
    }
    fmt.Printf("   - Redirect URI: %s\n", h.config.OAuthRedirectURI)
    fmt.Printf("   - Code length: %d\n", len(code))

    resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
    if err != nil { return "", fmt.Errorf("failed to exchange code: %w", err) }
    defer resp.Body.Close()

    fmt.Printf("?? Google OAuth response status: %d\n", resp.StatusCode)
    if ct := resp.Header.Get("Content-Type"); ct != "" { fmt.Printf("   - Content-Type: %s\n", ct) }
    if v := resp.Header.Get("Date"); v != "" { fmt.Printf("   - Date: %s\n", v) }

    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != http.StatusOK {
        msg := string(body)
        fmt.Printf("❌ Google OAuth error response (%d): %s\n", resp.StatusCode, msg)
        lower := strings.ToLower(msg)
        if strings.Contains(lower, "redirect_uri_mismatch") {
            fmt.Printf("💡 Hint: Check OAUTH_REDIRECT_URI and Google Console Authorized redirect URIs.\n")
        }
        if strings.Contains(lower, "invalid_client") {
            fmt.Printf("💡 Hint: Check GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET.\n")
        }
        if strings.Contains(lower, "invalid_grant") {
            fmt.Printf("💡 Hint: Code reused/expired or redirect_uri mismatch; re-initiate OAuth and ensure exact match.\n")
        }
        return "", fmt.Errorf("Google token exchange failed: %s", msg)
    }

    if len(body) == 0 { return "", fmt.Errorf("empty token response from Google") }
    var tokenResp GoogleTokenResponse
    if err := json.Unmarshal(body, &tokenResp); err != nil {
        fmt.Printf("❌ Failed to decode Google token JSON. Raw: %s\n", string(body))
        return "", fmt.Errorf("failed to decode token response: %w", err)
    }
    fmt.Printf("✅ Successfully obtained access token from Google\n")
    return tokenResp.AccessToken, nil
}

// getGoogleUserInfo 使用访问令牌获取用户信息
func (h *AuthHandler) getGoogleUserInfo(accessToken string) (*GoogleUser, error) {
	// 创建请求
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置Authorization头
	req.Header.Set("Authorization", "Bearer "+accessToken)

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google user info request failed: %s", string(body))
	}

	// 解析用户信息
	var user GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &user, nil
}

// findOrCreateUser 查找或创建用户
func (h *AuthHandler) findOrCreateUser(email, name, avatar, provider string) (*models.User, error) {
	// 先尝试查找现有用户
	user, err := h.db.GetUserByEmail(email)
	if err == nil {
		// 用户已存在，更新OAuth信息
		user.Name = name
		user.Avatar = avatar
		user.Provider = provider
		user.UpdatedAt = time.Now()

		if err := h.db.UpdateUser(user); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}

		fmt.Printf("👤 Found existing user %s, updated OAuth info (provider: %s)\n", user.Email, provider)
		return user, nil
	}

	// 用户不存在，创建新用户
	newUser := &models.User{
		// ID will be auto-generated by PostgreSQL
		Email:     email,
		Name:      name,
		Provider:  provider,
		Avatar:    avatar,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.db.CreateUser(newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Printf("👤 Created new OAuth user %s (provider: %s)\n", newUser.Email, provider)
	return newUser, nil
}

// handleChromeExtensionSuccess 处理Chrome扩展的成功响应
func (h *AuthHandler) handleChromeExtensionSuccess(w http.ResponseWriter, r *http.Request, user *models.User, accessToken, refreshToken string, expiresIn int64) {
	// 对于Chrome扩展，我们需要重定向到一个包含token信息的URL
	// Chrome Identity API会捕获这个重定向URL并提取参数
	// 使用一个特殊的回调URL格式，让前端能够解析
    orgID, _ := h.ensureDefaultOrgAndSpace(user)
    redirectURL := fmt.Sprintf("%s/api/oauth/extension/callback?success=true&access_token=%s&refresh_token=%s&expires_in=%d&user_id=%s&email=%s&name=%s&avatar=%s&provider=%s&org_id=%s",
        h.config.BaseURL,
        accessToken,
        refreshToken,
        expiresIn,
        user.ID,
        user.Email,
        url.QueryEscape(user.Name),
        url.QueryEscape(user.Avatar),
        user.Provider,
        orgID,
    )

	fmt.Printf("🔄 Redirecting Chrome extension to: %s\n", redirectURL)

	// 重定向到包含令牌的URL
	http.Redirect(w, r, redirectURL, http.StatusFound)

	fmt.Printf("✅ Chrome extension redirect completed\n")
}

// ExtensionOAuthCallback 处理扩展的OAuth回调页面
// 这个端点用于显示OAuth成功页面，让Chrome Identity API能够捕获URL参数
func (h *AuthHandler) ExtensionOAuthCallback(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("🔄 Extension OAuth callback page requested: %s\n", r.URL.String())

	// 获取URL参数
	params := r.URL.Query()
	success := params.Get("success")
	accessToken := params.Get("access_token")
	refreshToken := params.Get("refresh_token")
	expiresIn := params.Get("expires_in")
	userID := params.Get("user_id")
	email := params.Get("email")
	name := params.Get("name")
	avatar := params.Get("avatar")
	provider := params.Get("provider")

	// 构建HTML页面，显示OAuth结果
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>OAuth Success - Chrome Extension</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
        }
        .container {
            text-align: center;
            padding: 2rem;
            background: rgba(255, 255, 255, 0.1);
            border-radius: 10px;
            backdrop-filter: blur(10px);
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
        }
        .success-icon {
            font-size: 4rem;
            margin-bottom: 1rem;
        }
        .user-info {
            margin: 1rem 0;
            padding: 1rem;
            background: rgba(255, 255, 255, 0.1);
            border-radius: 8px;
        }
        .close-message {
            margin-top: 2rem;
            opacity: 0.8;
            font-size: 0.9rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="success-icon">✅</div>
        <h1>OAuth Authentication Successful!</h1>
        <p>You have successfully authenticated with %s.</p>

        <div class="user-info">
            <h3>Welcome, %s!</h3>
            <p>Email: %s</p>
            <p>User ID: %s</p>
        </div>

        <div class="close-message">
            <p>This window will close automatically.</p>
            <p>You can now return to the extension.</p>
        </div>
    </div>

    <script>
        // 自动关闭窗口（如果是弹窗）
        setTimeout(() => {
            if (window.opener) {
                window.close();
            }
        }, 3000);

        // 记录OAuth参数供Chrome Identity API使用
        console.log('OAuth Success Parameters:', {
            success: '%s',
            access_token: '%s',
            refresh_token: '%s',
            expires_in: '%s',
            user_id: '%s',
            email: '%s',
            name: '%s',
            avatar: '%s',
            provider: '%s'
        });
    </script>
</body>
</html>`,
		provider, name, email, userID,
		success, accessToken, refreshToken, expiresIn, userID, email, name, avatar, provider)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))

	fmt.Printf("✅ Extension OAuth callback page served successfully\n")
}

// handleChromeExtensionError 处理Chrome扩展的错误响应
func (h *AuthHandler) handleChromeExtensionError(w http.ResponseWriter, r *http.Request, errorCode, errorMessage string) {
	// 获取扩展ID（使用Client ID的前32位，但确保长度足够）
	extensionID := h.config.GoogleClientID
	if len(extensionID) > 32 {
		extensionID = extensionID[:32]
	}

	// Chrome扩展的错误重定向URL
	redirectURL := fmt.Sprintf("%s?error=%s&error_description=%s",
		h.config.OAuthRedirectURI,
		errorCode,
		errorMessage,
	)

	// 重定向到Chrome扩展
	http.Redirect(w, r, redirectURL, http.StatusFound)
	fmt.Printf("🔄 Redirected Chrome extension to error URL: %s\n", errorCode)
}

// ClientType 客户端类型枚举
type ClientType string

const (
	ClientTypeWeb       ClientType = "web"
	ClientTypeExtension ClientType = "extension"
	ClientTypeAPI       ClientType = "api"
)

// detectClientType 检测客户端类型
func (h *AuthHandler) detectClientType(r *http.Request) ClientType {
	fmt.Printf("🔍 Detecting client type for request: %s\n", r.URL.String())

	// 检查URL参数中的客户端类型标识
	if clientType := r.URL.Query().Get("client_type"); clientType != "" {
		fmt.Printf("🔍 Found client_type in URL params: %s\n", clientType)
		switch clientType {
		case "extension":
			return ClientTypeExtension
		case "web":
			return ClientTypeWeb
		case "api":
			return ClientTypeAPI
		}
	}

	// 检查state参数中的客户端类型（JSON格式）
	if state := r.URL.Query().Get("state"); state != "" {
		fmt.Printf("🔍 Found state parameter: %s\n", state)
		var stateData map[string]interface{}
		if err := json.Unmarshal([]byte(state), &stateData); err == nil {
			fmt.Printf("🔍 Parsed state data: %+v\n", stateData)
			if clientType, ok := stateData["client_type"].(string); ok {
				fmt.Printf("🔍 Found client_type in state: %s\n", clientType)
				switch clientType {
				case "extension":
					fmt.Printf("✅ Detected as Extension client\n")
					return ClientTypeExtension
				case "web":
					fmt.Printf("✅ Detected as Web client\n")
					return ClientTypeWeb
				case "api":
					fmt.Printf("✅ Detected as API client\n")
					return ClientTypeAPI
				}
			} else {
				fmt.Printf("❌ No client_type found in state data\n")
			}
		} else {
			fmt.Printf("❌ Failed to parse state JSON: %v\n", err)
		}
	} else {
		fmt.Printf("❌ No state parameter found\n")
	}

	// 检查Referer头
	referer := r.Header.Get("Referer")
	if strings.Contains(referer, "chrome-extension://") {
		return ClientTypeExtension
	}

	// 检查User-Agent
	userAgent := r.Header.Get("User-Agent")
	if strings.Contains(userAgent, "Chrome") && r.Header.Get("X-Requested-With") == "" {
		// 可能是Chrome扩展，但不确定
		return ClientTypeWeb // 默认为Web，除非明确指定
	}

	// 检查Accept头
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return ClientTypeAPI
	}

	// 默认为Web客户端
	return ClientTypeWeb
}

// handleOAuthSuccess 处理OAuth成功响应
func (h *AuthHandler) handleOAuthSuccess(w http.ResponseWriter, r *http.Request, clientType ClientType, user *models.User, accessToken, refreshToken string, expiresIn int64, orgID string) {
	// 对于POST请求（如/api/auth/oauth/google），无论客户端类型如何，都返回JSON
	// 只有GET请求的OAuth回调才使用重定向
	if r.Method == http.MethodPost {
        response := models.UserLoginResponse{
            User:         *user,
            AccessToken:  accessToken,
            RefreshToken: refreshToken,
            ExpiresIn:    expiresIn,
            OrgID:        orgID,
        }
        utils.WriteSuccessResponse(w, response)
        return
    }

	// GET请求的OAuth回调根据客户端类型处理
	switch clientType {
	case ClientTypeExtension:
		h.handleChromeExtensionSuccess(w, r, user, accessToken, refreshToken, expiresIn)
    case ClientTypeAPI:
        // API客户端返回JSON
        response := models.UserLoginResponse{
            User:         *user,
            AccessToken:  accessToken,
            RefreshToken: refreshToken,
            ExpiresIn:    expiresIn,
            OrgID:        orgID,
        }
        utils.WriteSuccessResponse(w, response)
    case ClientTypeWeb:
        // Web客户端重定向到前端页面
        h.handleWebClientSuccess(w, r, user, accessToken, refreshToken, expiresIn, orgID)
    default:
        // 默认返回JSON
        response := models.UserLoginResponse{
            User:         *user,
            AccessToken:  accessToken,
            RefreshToken: refreshToken,
            ExpiresIn:    expiresIn,
            OrgID:        orgID,
        }
        utils.WriteSuccessResponse(w, response)
    }
}

// handleOAuthError 处理OAuth错误响应
func (h *AuthHandler) handleOAuthError(w http.ResponseWriter, r *http.Request, clientType ClientType, errorCode, errorMessage string) {
	switch clientType {
	case ClientTypeExtension:
		h.handleChromeExtensionError(w, r, errorCode, errorMessage)
	case ClientTypeAPI:
		utils.WriteInternalServerErrorResponse(w, errorMessage)
	case ClientTypeWeb:
		h.handleWebClientError(w, r, errorCode, errorMessage)
	default:
		utils.WriteInternalServerErrorResponse(w, errorMessage)
	}
}

// handleWebClientSuccess 处理Web客户端的成功响应
func (h *AuthHandler) handleWebClientSuccess(w http.ResponseWriter, r *http.Request, user *models.User, accessToken, refreshToken string, expiresIn int64, orgID string) {
	// 获取前端回调URL（可以从环境变量或配置中获取）
    frontendURL := h.getFrontendCallbackURL()

    // Set HttpOnly cookie for same-origin web clients so subsequent
    // requests can be authorized without manually injecting headers.
    http.SetCookie(w, &http.Cookie{
        Name:     "access_token",
        Value:    accessToken,
        Path:     "/",
        MaxAge:   int(expiresIn),
        HttpOnly: true,
        Secure:   strings.HasPrefix(strings.ToLower(h.config.BaseURL), "https://"),
        SameSite: http.SameSiteLaxMode,
    })

	// 构建重定向URL，将令牌作为URL参数传递
    redirectURL := fmt.Sprintf("%s?success=true&access_token=%s&refresh_token=%s&expires_in=%d&user_id=%s&email=%s&name=%s&org_id=%s",
        frontendURL,
        accessToken,
        refreshToken,
        expiresIn,
        user.ID,
        user.Email,
        user.Name,
        orgID,
    )

	// 重定向到前端
	http.Redirect(w, r, redirectURL, http.StatusFound)
	fmt.Printf("🔄 Redirected web client to frontend: %s\n", frontendURL)
}

// handleWebClientError 处理Web客户端的错误响应
func (h *AuthHandler) handleWebClientError(w http.ResponseWriter, r *http.Request, errorCode, errorMessage string) {
	// 获取前端错误页面URL
	frontendURL := h.getFrontendCallbackURL()

	// 构建错误重定向URL
	redirectURL := fmt.Sprintf("%s?error=%s&error_description=%s",
		frontendURL,
		errorCode,
		errorMessage,
	)

	// 重定向到前端错误页面
	http.Redirect(w, r, redirectURL, http.StatusFound)
	fmt.Printf("🔄 Redirected web client to frontend error page: %s\n", errorCode)
}

// getFrontendCallbackURL 获取前端回调URL
func (h *AuthHandler) getFrontendCallbackURL() string {
	// 从环境变量获取前端回调URL，支持多种客户端类型
    if frontendURL := os.Getenv("FRONTEND_CALLBACK_URL"); frontendURL != "" {
        return strings.TrimSpace(frontendURL)
    }

	// 默认使用Chrome扩展的回调页面
	// Chrome扩展使用 chrome-extension:// 协议，但这里我们返回一个通用的本地页面
	// 实际上，对于Chrome扩展，我们应该直接返回JSON响应而不是重定向
	return "http://localhost:3000/auth/callback"
}

// exchangeGitHubCodeForToken 交换GitHub授权码为访问令牌
func (h *AuthHandler) exchangeGitHubCodeForToken(code string) (string, error) {
	// 构建请求数据
	data := url.Values{}
	data.Set("client_id", h.config.GitHubClientID)
	data.Set("client_secret", h.config.GitHubClientSecret)
	data.Set("code", code)

	fmt.Printf("🔄 Exchanging code with GitHub OAuth:\n")
	fmt.Printf("   - Client ID: %s\n", h.config.GitHubClientID[:10]+"...")
	fmt.Printf("   - Redirect URI: %s\n", h.config.OAuthRedirectURI)
	fmt.Printf("   - Code length: %d\n", len(code))

	// 发送POST请求到GitHub
	resp, err := http.PostForm("https://github.com/login/oauth/access_token", data)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("📡 GitHub OAuth response status: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub OAuth failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// GitHub返回的是URL编码格式，需要解析
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	accessToken := values.Get("access_token")
	if accessToken == "" {
		return "", fmt.Errorf("no access token in response: %s", string(body))
	}

	fmt.Printf("✅ Successfully obtained GitHub access token\n")
	return accessToken, nil
}

// getGitHubUserInfo 获取GitHub用户信息
func (h *AuthHandler) getGitHubUserInfo(accessToken string) (*GitHubUser, error) {
	// 创建请求
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置Authorization头
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析用户信息
	var githubUser GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	// 如果用户没有公开邮箱，需要单独获取
	if githubUser.Email == "" {
		email, err := h.getGitHubUserEmail(accessToken)
		if err != nil {
			fmt.Printf("⚠️ Failed to get GitHub user email: %v\n", err)
		} else {
			githubUser.Email = email
		}
	}

	fmt.Printf("👤 Retrieved GitHub user info: %s (%s)\n", githubUser.Login, githubUser.Email)
	return &githubUser, nil
}

// getGitHubUserEmail 获取GitHub用户的主邮箱
func (h *AuthHandler) getGitHubUserEmail(accessToken string) (string, error) {
	// 创建请求
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置Authorization头
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get user emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub emails API failed with status %d", resp.StatusCode)
	}

	// 解析邮箱列表
	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("failed to decode emails: %w", err)
	}

	// 查找主邮箱
	for _, email := range emails {
		if email.Primary {
			return email.Email, nil
		}
	}

	// 如果没有主邮箱，返回第一个
	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", fmt.Errorf("no email found")
}

// getClientIP 获取客户端IP地址
func (h *AuthHandler) getClientIP(r *http.Request) string {
	// 检查X-Forwarded-For头（代理环境）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// 检查X-Real-IP头
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// 使用RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}

// generateSessionCode 生成会话码
func (h *AuthHandler) generateSessionCode(userID, email, name, clientIP string) (string, error) {
	// 简单实现：使用JWT作为session code
	// 创建包含用户信息的临时token
	jwtService := utils.NewJWTService(h.config.JWTSecret)

	// 生成一个短期的session token（5分钟有效期）
	sessionToken, _, _, err := jwtService.GenerateTokenPair(userID, email)
	if err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}

	fmt.Printf("🔑 Generated session code for user %s from IP %s\n", email, clientIP)
	return sessionToken, nil
}

// ExchangeSession 交换会话码获取用户信息
func (h *AuthHandler) ExchangeSession(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("🔍 ExchangeSession: Request received\n")

	// 解析请求体
	var req struct {
		SessionCode string `json:"session_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("❌ ExchangeSession: Invalid request body: %v\n", err)
		utils.WriteBadRequestResponse(w, "Invalid request body")
		return
	}

	if req.SessionCode == "" {
		fmt.Printf("❌ ExchangeSession: Session code is empty\n")
		utils.WriteBadRequestResponse(w, "Session code is required")
		return
	}

	fmt.Printf("🔍 ExchangeSession: Session code received (length: %d)\n", len(req.SessionCode))

	// 解析JWT token获取用户信息
	_, email, err := h.validateSessionCode(req.SessionCode)
	if err != nil {
		fmt.Printf("❌ Session validation failed: %v\n", err)
		utils.WriteUnauthorizedResponse(w, "Invalid or expired session")
		return
	}

	// 从数据库获取完整的用户信息
	user, err := h.db.GetUserByEmail(email)
	if err != nil {
		fmt.Printf("❌ Failed to get user info: %v\n", err)
		utils.WriteNotFoundResponse(w, "User not found")
		return
	}

	// 返回用户信息
	response := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	}

	utils.WriteSuccessResponse(w, response)
	fmt.Printf("✅ Session exchanged for user %s\n", user.Email)
}

// validateSessionCode 验证会话码并提取用户信息
func (h *AuthHandler) validateSessionCode(sessionCode string) (userID, email string, err error) {
	// 解析JWT token
	parts := strings.Split(sessionCode, ".")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid session code format")
	}

	// 直接解析payload部分
	payload := parts[1]
	// 添加padding如果需要
	for len(payload)%4 != 0 {
		payload += "="
	}

	payloadBytes, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode payload: %w", err)
	}

	// 解析JSON
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", "", fmt.Errorf("failed to parse claims: %w", err)
	}

	// 检查过期时间
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return "", "", fmt.Errorf("session expired")
		}
	}

	// 提取用户信息
	if userIDClaim, ok := claims["user_id"].(string); ok {
		userID = userIDClaim
	} else {
		return "", "", fmt.Errorf("missing user_id in session")
	}

	if emailClaim, ok := claims["email"].(string); ok {
		email = emailClaim
	} else {
		return "", "", fmt.Errorf("missing email in session")
	}

	return userID, email, nil
}
