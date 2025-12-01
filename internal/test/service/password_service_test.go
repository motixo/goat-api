package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mot0x0/goth-api/internal/config"
	"github.com/mot0x0/goth-api/internal/domain/errors"
	"github.com/mot0x0/goth-api/internal/domain/service"
)

func TestPasswordService_HashAndVerify(t *testing.T) {
	cfg := &config.Config{PasswordPepper: "secret-pepper"}
	svc := service.NewPasswordService(cfg)
	ctx := context.Background()

	validPassword := "Abc123!@#"

	hashed, err := svc.Hash(ctx, validPassword)
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}

	if hashed.Value() == "" {
		t.Fatal("hashed password should not be empty")
	}

	if !svc.Verify(ctx, validPassword, hashed) {
		t.Fatal("Verify should return true for correct password")
	}

	if svc.Verify(ctx, "WrongPassword1!", hashed) {
		t.Fatal("Verify should return false for incorrect password")
	}
}

func TestPasswordService_ValidatePolicy(t *testing.T) {
	cfg := &config.Config{PasswordPepper: "pepper"}
	svc := service.NewPasswordService(cfg)
	ctx := context.Background()

	longPassword := "A" + "b1!" + strings.Repeat("x", 70)

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"valid password", "Abc123!@", nil},
		{"too short", "Ab1!", errors.ErrPasswordTooShort},
		{"too long", longPassword, errors.ErrPasswordTooLong},
		{"no upper", "abc123!@#", errors.ErrPasswordPolicyViolation},
		{"no lower", "ABC123!@#", errors.ErrPasswordPolicyViolation},
		{"no digit", "Abcdef!@#", errors.ErrPasswordPolicyViolation},
		{"no special", "Abc123456", errors.ErrPasswordPolicyViolation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Hash(ctx, tt.password)
			if err != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
