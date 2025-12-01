package valueobject

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mot0x0/goth-api/internal/domain/errors"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type JWTClaims struct {
	UserID    string    `json:"user_id"`
	TokenType TokenType `json:"token_type"`
	SessionID string    `json:"session_id,omitempty"`
	JTI       string    `json:"jti,omitempty"`
	jwt.RegisteredClaims
}

func NewAccessToken(userID, secret, sessionID, jti string) (string, time.Time, error) {
	return newToken(userID, secret, sessionID, jti, TokenTypeAccess, 15*time.Minute)
}

func NewRefreshToken(userID, secret, jti string) (string, time.Time, error) {
	return newToken(userID, secret, "", jti, TokenTypeRefresh, 30*24*time.Hour)
}

func newToken(userID, secret, sessionID, jti string, tokenType TokenType, duration time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().UTC().Add(duration)

	claims := JWTClaims{
		UserID:    userID,
		TokenType: tokenType,
		SessionID: sessionID,
		JTI:       jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "goth-api",
			Subject:   string(tokenType),
			Audience:  jwt.ClaimStrings{"api"},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			NotBefore: jwt.NewNumericDate(time.Now().UTC()),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	return signed, expiresAt, err
}

func ParseAndValidate(tokenStr, secret string) (*JWTClaims, error) {
	claims := &JWTClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	}, jwt.WithLeeway(5*time.Second))

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.ErrUnauthorized
	}

	return claims, nil
}
