package auth

import (
	"context"
	"errors"
	"testing"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestRefreshRejectsAccessTokenPurpose(t *testing.T) {
	tokens := &refreshPurposeJWTService{
		claims: &valueobject.JWTClaims{
			UserID:    "user-1",
			TokenType: valueobject.TokenTypeAccess,
		},
	}
	usecase := NewUsecase(nil, nil, nil, tokens, nil, discardAuthLogger{}, 0, 0, 0)

	_, err := usecase.Refresh(context.Background(), RefreshInput{RefreshToken: "access-token"})

	if !errors.Is(err, domainErrors.ErrUnauthorized) {
		t.Fatalf("Refresh() error = %v, want unauthorized", err)
	}
}

type refreshPurposeJWTService struct {
	service.JWTService
	claims *valueobject.JWTClaims
}

func (s *refreshPurposeJWTService) ParseAndValidate(string) (*valueobject.JWTClaims, error) {
	return s.claims, nil
}
