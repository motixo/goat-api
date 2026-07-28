package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestPasswordServiceVerifiesRehydratedHash(t *testing.T) {
	passwordService := mustNewPasswordService(t, "test-pepper", 2)
	hashed, err := passwordService.Hash(context.Background(), testPlainPassword("Password1!"))
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	rehydrated := testPasswordDigest(hashed.Encoded())
	verified, err := passwordService.Verify(context.Background(), testPlainPassword("Password1!"), rehydrated)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !verified {
		t.Fatal("Verify() rejected the original password after hash rehydration")
	}
	verified, err = passwordService.Verify(context.Background(), testPlainPassword("WrongPassword1!"), rehydrated)
	if err != nil {
		t.Fatalf("Verify() incorrect-password error = %v", err)
	}
	if verified {
		t.Fatal("Verify() accepted an incorrect password")
	}
	differentPepper := mustNewPasswordService(t, "different-pepper", 2)
	verified, err = differentPepper.Verify(
		context.Background(),
		testPlainPassword("Password1!"),
		rehydrated,
	)
	if err != nil {
		t.Fatalf("Verify() different-pepper error = %v", err)
	}
	if verified {
		t.Fatal("Verify() accepted a hash created with a different pepper")
	}
	if !strings.HasPrefix(hashed.Encoded(), "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("Hash() changed Argon2id parameters: %q", hashed.Encoded())
	}
}

func TestPasswordServiceHashesHashLookingPlaintext(t *testing.T) {
	const raw = "$argon2id$Password1!"
	password, err := valueobject.NewPlainPassword(raw)
	if err != nil {
		t.Fatalf("NewPlainPassword() error = %v", err)
	}
	passwordService := mustNewPasswordService(t, "hash-looking-plaintext-pepper", 1)
	digest, err := passwordService.Hash(context.Background(), password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if digest.Encoded() == raw {
		t.Fatal("Hash() treated hash-looking plaintext as an existing digest")
	}
	verified, err := passwordService.Verify(context.Background(), password, digest)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !verified {
		t.Fatal("Verify() rejected hash-looking plaintext after hashing")
	}
}

func TestPasswordServiceFormattingRedactsPepper(t *testing.T) {
	t.Parallel()

	const pepper = "formatting-must-not-expose-this-pepper"
	cfg := PasswordHasherConfig{Pepper: pepper, MaxConcurrency: 2}
	passwordService := mustNewPasswordService(t, pepper, cfg.MaxConcurrency)

	for name, value := range map[string]any{
		"configuration": cfg,
		"service":       passwordService,
	} {
		for _, format := range []string{"%s", "%v", "%+v", "%#v"} {
			formatted := fmt.Sprintf(format, value)
			if strings.Contains(formatted, pepper) {
				t.Fatalf("%s format %q exposed the password pepper", name, format)
			}
		}
	}

	_, err := valueobject.NewPlainPassword("short")
	if err == nil {
		t.Fatal("NewPlainPassword() error = nil for a password that violates policy")
	}
	if strings.Contains(err.Error(), pepper) {
		t.Fatal("Hash() error exposed the password pepper")
	}

	for _, invalid := range []PasswordHasherConfig{
		{Pepper: pepper, MaxConcurrency: 0},
		{Pepper: pepper, MaxConcurrency: maximumPasswordHashConcurrency + 1},
	} {
		_, err := NewPasswordService(invalid)
		if err == nil {
			t.Fatalf("NewPasswordService(%v) error = nil", invalid)
		}
		if strings.Contains(err.Error(), pepper) {
			t.Fatal("constructor error exposed the password pepper")
		}
	}
}

func mustNewPasswordService(t testing.TB, pepper string, maxConcurrency int) *PasswordService {
	t.Helper()
	passwordService, err := NewPasswordService(PasswordHasherConfig{
		Pepper:         pepper,
		MaxConcurrency: maxConcurrency,
	})
	if err != nil {
		t.Fatalf("NewPasswordService() error = %v", err)
	}
	return passwordService
}

func testPlainPassword(raw string) valueobject.PlainPassword {
	password, err := valueobject.NewPlainPassword(raw)
	if err != nil {
		panic("test plaintext password is invalid")
	}
	return password
}

func testPasswordDigest(encoded string) valueobject.PasswordDigest {
	if encoded == "" {
		return valueobject.PasswordDigest{}
	}
	digest, err := valueobject.NewPasswordDigest(encoded)
	if err != nil {
		panic("test password digest is invalid")
	}
	return digest
}
