package service

import (
	"errors"
	"testing"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
)

func TestJWTManagerClassifiesMalformedTokensSemantically(t *testing.T) {
	manager := NewJWTManager("test-secret")

	_, err := manager.ParseAndValidate("not-a-jwt")

	if !errors.Is(err, domainErrors.ErrTokenInvalid) {
		t.Fatalf("ParseAndValidate() error = %v, want ErrTokenInvalid", err)
	}
	if errors.Is(err, domainErrors.ErrUnauthorized) {
		t.Fatalf("ParseAndValidate() error = %v, must not use generic ErrUnauthorized", err)
	}
}
