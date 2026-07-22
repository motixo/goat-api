package service

import (
	"context"
	"testing"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestPasswordServiceVerifiesRehydratedHash(t *testing.T) {
	passwordService := NewPasswordService(&config.Config{PasswordPepper: "test-pepper"})
	hashed, err := passwordService.Hash(context.Background(), "Password1!")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	rehydrated := valueobject.PasswordFromHash(hashed.Encoded())
	if !passwordService.Verify(context.Background(), "Password1!", rehydrated) {
		t.Fatal("Verify() rejected the original password after hash rehydration")
	}
	if passwordService.Verify(context.Background(), "WrongPassword1!", rehydrated) {
		t.Fatal("Verify() accepted an incorrect password")
	}
}
