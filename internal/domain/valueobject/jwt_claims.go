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
	UserID            string
	SessionID         string
	CredentialVersion int64
	Role              UserRole
	Permissions       PermissionSet
	TokenType         TokenType
	JTI               string
	Issuer            string
	Subject           string
	Audience          []string
	ExpiresAt         time.Time
	IssuedAt          time.Time
	NotBefore         time.Time
}

type TokenIdentity struct {
	UserID            string
	SessionID         string
	JTI               string
	CredentialVersion int64
}

type AuthorizationSnapshot struct {
	Role        UserRole
	Permissions PermissionSet
}

func NewJWTClaims(
	identity TokenIdentity,
	tokenType TokenType,
	expiresAt time.Time,
	issuedAt time.Time,
	snapshot AuthorizationSnapshot,
) (*JWTClaims, error) {
	issuedAt = issuedAt.UTC()
	expiresAt = expiresAt.UTC()
	if identity.UserID == "" || identity.SessionID == "" || identity.JTI == "" ||
		identity.CredentialVersion <= 0 {
		return nil, errors.ErrInvalidInput
	}
	if tokenType != TokenTypeAccess && tokenType != TokenTypeRefresh {
		return nil, errors.ErrInvalidInput
	}
	if !expiresAt.After(issuedAt) {
		return nil, errors.ErrInvalidInput
	}
	if tokenType == TokenTypeAccess && snapshot.Role == RoleUnknown {
		return nil, errors.ErrInvalidInput
	}

	claims := &JWTClaims{
		UserID:            identity.UserID,
		SessionID:         identity.SessionID,
		CredentialVersion: identity.CredentialVersion,
		Role:              snapshot.Role,
		Permissions:       snapshot.Permissions,
		TokenType:         tokenType,
		JTI:               identity.JTI,
		Issuer:            TokenIssuer,
		Subject:           string(tokenType),
		Audience:          []string{TokenAudience},
		ExpiresAt:         expiresAt,
		IssuedAt:          issuedAt,
		NotBefore:         issuedAt,
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
	return c.IsExpiredAt(time.Now())
}

func (c *JWTClaims) IsExpiredAt(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}
