package valueobject

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mot0x0/gopi/internal/domain/errors"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type JWTClaims struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	TokenType TokenType `json:"token_type"`
	JTI       string    `json:"jti,omitempty"`
	jwt.RegisteredClaims
}

// NewAccessToken
func NewAccessToken(userID string, email, secret string) (string, string, time.Time, error) {
	return newToken(userID, email, secret, TokenTypeAccess, 15*time.Minute)
}

// NewRefreshToken
func NewRefreshToken(userID string, email, secret string) (string, string, time.Time, error) {
	return newToken(userID, email, secret, TokenTypeRefresh, 14*24*time.Hour)
}

func newToken(userID string, email, secret string, tokenType TokenType, duration time.Duration) (string, string, time.Time, error) {
	expiresAt := time.Now().UTC().Add(duration)
	jti := uuid.New().String()

	claims := JWTClaims{
		UserID:    userID,
		Email:     email,
		TokenType: tokenType,
		JTI:       jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "gopi",
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
	return signed, jti, expiresAt, err
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
