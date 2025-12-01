package valueobject_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mot0x0/goth-api/internal/domain/valueobject"
)

func TestJWT_NewAndParseAccessToken(t *testing.T) {
	secret := "supersecret"
	userID := "user-123"
	sessionID := "session-456"
	jti := "jti-789"

	tokenStr, expiresAt, err := valueobject.NewAccessToken(userID, secret, sessionID, jti)
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	if tokenStr == "" {
		t.Fatal("token string should not be empty")
	}

	if time.Until(expiresAt) <= 0 {
		t.Fatal("expiresAt should be in the future")
	}

	claims, err := valueobject.ParseAndValidate(tokenStr, secret)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected UserID %s, got %s", userID, claims.UserID)
	}

	if claims.TokenType != valueobject.TokenTypeAccess {
		t.Errorf("expected TokenType access, got %s", claims.TokenType)
	}

	if claims.SessionID != sessionID {
		t.Errorf("expected SessionID %s, got %s", sessionID, claims.SessionID)
	}

	if claims.JTI != jti {
		t.Errorf("expected JTI %s, got %s", jti, claims.JTI)
	}
}

func TestJWT_NewAndParseRefreshToken(t *testing.T) {
	secret := "supersecret"
	userID := "user-123"
	jti := "jti-789"

	tokenStr, expiresAt, err := valueobject.NewRefreshToken(userID, secret, jti)
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	if tokenStr == "" {
		t.Fatal("token string should not be empty")
	}

	if time.Until(expiresAt) <= 0 {
		t.Fatal("expiresAt should be in the future")
	}

	claims, err := valueobject.ParseAndValidate(tokenStr, secret)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected UserID %s, got %s", userID, claims.UserID)
	}

	if claims.TokenType != valueobject.TokenTypeRefresh {
		t.Errorf("expected TokenType refresh, got %s", claims.TokenType)
	}

	if claims.JTI != jti {
		t.Errorf("expected JTI %s, got %s", jti, claims.JTI)
	}
}

func TestJWT_ParseInvalidToken(t *testing.T) {
	secret := "supersecret"
	badSecret := "wrongsecret"
	userID := "user-123"
	jti := "jti-789"

	tokenStr, _, err := valueobject.NewRefreshToken(userID, secret, jti)
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	_, err = valueobject.ParseAndValidate(tokenStr, badSecret)
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature error, got %v", err)
	}

	_, err = valueobject.ParseAndValidate("invalid.token.string", secret)
	if err == nil {
		t.Fatal("expected error for invalid token string")
	}
}
