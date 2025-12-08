// internal/domain/valueobject/jwt_claims.go
package valueobject

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/errors"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

const (
	TokenIssuer   = "goat-api"
	TokenAudience = "api"
)

type JWTClaims struct {
	UserID    string
	SessionID string
	UserRole  int8
	TokenType TokenType
	JTI       string
	Issuer    string
	Subject   string
	Audience  []string
	ExpiresAt time.Time
	IssuedAt  time.Time
	NotBefore time.Time
}

func NewJWTClaims(userID string, sessionID string, tokenType TokenType, jti string, expiresAt time.Time) (*JWTClaims, error) {
	if userID == "" || jti == "" {
		return nil, errors.ErrInvalidInput
	}
	if !expiresAt.After(time.Now()) {
		return nil, errors.ErrInvalidInput
	}

	claims := &JWTClaims{
		UserID:    userID,
		SessionID: sessionID,
		TokenType: tokenType,
		JTI:       jti,
		Issuer:    TokenIssuer,
		Subject:   string(tokenType),
		Audience:  []string{TokenAudience},
		ExpiresAt: expiresAt,
		IssuedAt:  time.Now(),
		NotBefore: time.Now(),
	}

	return claims, nil
}

func (c *JWTClaims) GetUserID() string       { return c.UserID }
func (c *JWTClaims) GetSessionID() string    { return c.SessionID }
func (c *JWTClaims) GetTokenType() TokenType { return c.TokenType }
func (c *JWTClaims) GetJTI() string          { return c.JTI }
func (c *JWTClaims) GetExpiresAt() time.Time { return c.ExpiresAt }
func (c *JWTClaims) GetIssuedAt() time.Time  { return c.IssuedAt }

func (c *JWTClaims) IsAccess() bool  { return c.TokenType == TokenTypeAccess }
func (c *JWTClaims) IsRefresh() bool { return c.TokenType == TokenTypeRefresh }

func (c *JWTClaims) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}
