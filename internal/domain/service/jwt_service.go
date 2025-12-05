package service

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type JWTService interface {
	GenerateAccessToken(userID, sessionID, jti string, duration time.Duration) (string, *valueobject.JWTClaims, error)
	GenerateRefreshToken(userID, jti string, duration time.Duration) (string, *valueobject.JWTClaims, error)
	ParseAndValidate(tokenStr string) (*valueobject.JWTClaims, error)
	ValidateClaims(claims *valueobject.JWTClaims) error
}
