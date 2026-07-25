package service

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type JWTService interface {
	GenerateAccessToken(
		identity valueobject.TokenIdentity,
		snapshot valueobject.AuthorizationSnapshot,
		duration time.Duration,
	) (string, *valueobject.JWTClaims, error)
	GenerateRefreshToken(
		identity valueobject.TokenIdentity,
		duration time.Duration,
	) (string, *valueobject.JWTClaims, error)
	ParseAndValidate(tokenStr string) (*valueobject.JWTClaims, error)
	ValidateClaims(claims *valueobject.JWTClaims) error
}
