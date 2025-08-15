package utils

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"tab-sync-backend-refactor/pkg/models"
)

// JWTService JWT服务
type JWTService struct {
	secretKey []byte
}

// NewJWTService 创建JWT服务
func NewJWTService(secretKey string) *JWTService {
	return &JWTService{
		secretKey: []byte(secretKey),
	}
}

// GenerateTokenPair 生成访问令牌和刷新令牌对
func (j *JWTService) GenerateTokenPair(userID, email string) (accessToken, refreshToken string, expiresIn int64, err error) {
	now := time.Now()
	
	// 访问令牌（15分钟有效期）
	accessExpiry := now.Add(15 * time.Minute)
	accessClaims := &models.TokenClaims{
		UserID: userID,
		Email:  email,
		Type:   "access",
		Exp:    accessExpiry.Unix(),
		Iat:    now.Unix(),
	}

	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessTokenObj.SignedString(j.secretKey)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to generate access token: %w", err)
	}

	// 刷新令牌（7天有效期）
	refreshExpiry := now.Add(7 * 24 * time.Hour)
	refreshClaims := &models.TokenClaims{
		UserID: userID,
		Email:  email,
		Type:   "refresh",
		Exp:    refreshExpiry.Unix(),
		Iat:    now.Unix(),
	}

	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTokenObj.SignedString(j.secretKey)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, accessExpiry.Unix(), nil
}

// GenerateAccessToken 生成访问令牌
func (j *JWTService) GenerateAccessToken(userID, email string) (string, int64, error) {
	now := time.Now()
	expiry := now.Add(15 * time.Minute)

	claims := &models.TokenClaims{
		UserID: userID,
		Email:  email,
		Type:   "access",
		Exp:    expiry.Unix(),
		Iat:    now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secretKey)
	if err != nil {
		return "", 0, fmt.Errorf("failed to generate access token: %w", err)
	}

	return tokenString, expiry.Unix(), nil
}

// ValidateToken 验证令牌
func (j *JWTService) ValidateToken(tokenString string) (*models.TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*models.TokenClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// 检查是否过期
	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// ValidateRefreshToken 验证刷新令牌
func (j *JWTService) ValidateRefreshToken(tokenString string) (*models.TokenClaims, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.Type != "refresh" {
		return nil, fmt.Errorf("invalid token type: expected refresh, got %s", claims.Type)
	}

	return claims, nil
}

// RefreshAccessToken 使用刷新令牌生成新的访问令牌
func (j *JWTService) RefreshAccessToken(refreshToken string) (string, int64, error) {
	claims, err := j.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", 0, fmt.Errorf("invalid refresh token: %w", err)
	}

	return j.GenerateAccessToken(claims.UserID, claims.Email)
}

// ExtractUserFromToken 从令牌中提取用户信息
func (j *JWTService) ExtractUserFromToken(tokenString string) (*models.User, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:    claims.UserID,
		Email: claims.Email,
	}, nil
}
