package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// User represents a user in the system
type User struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password_hash"` // Never return password in JSON
	Name      string    `json:"name,omitempty" db:"name"`
	Avatar    string    `json:"avatar,omitempty" db:"avatar"`
	Provider  string    `json:"provider,omitempty" db:"provider"` // "email", "google", "github"
	Tier      string    `json:"tier,omitempty" db:"tier"`         // "free", "pro", "power"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// UserRegisterRequest represents the request payload for user registration
type UserRegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// UserLoginRequest represents the request payload for user login
type UserLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// UserLoginResponse represents the response payload for user login
type UserLoginResponse struct {
    User         User   `json:"user"`
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
    OrgID        string `json:"org_id,omitempty"`
}

// RefreshTokenRequest represents the request payload for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// TokenClaims represents the JWT token claims
type TokenClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Type   string `json:"type"` // "access" or "refresh"
	Exp    int64  `json:"exp"`
	Iat    int64  `json:"iat"`
}

// GetExpirationTime implements jwt.Claims interface
func (c *TokenClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.Exp, 0)), nil
}

// GetIssuedAt implements jwt.Claims interface
func (c *TokenClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.Iat, 0)), nil
}

// GetNotBefore implements jwt.Claims interface
func (c *TokenClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return nil, nil
}

// GetIssuer implements jwt.Claims interface
func (c *TokenClaims) GetIssuer() (string, error) {
	return "", nil
}

// GetSubject implements jwt.Claims interface
func (c *TokenClaims) GetSubject() (string, error) {
	return c.UserID, nil
}

// GetAudience implements jwt.Claims interface
func (c *TokenClaims) GetAudience() (jwt.ClaimStrings, error) {
	return nil, nil
}
