package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestAccessTokenRoundTripCarriesBoundedAuthorizationSnapshot(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	manager := newJWTManagerWithClock("test-secret", func() time.Time { return now })
	permissions, err := valueobject.NewPermissionSet([]valueobject.Permission{
		valueobject.PermUserUpdate,
		valueobject.PermUserRead,
		valueobject.PermUserRead,
	})
	if err != nil {
		t.Fatalf("build permission set: %v", err)
	}
	identity := valueobject.TokenIdentity{
		UserID:            "user-1",
		SessionID:         "session-1",
		JTI:               "jti-1",
		CredentialVersion: 4,
	}

	token, generated, err := manager.GenerateAccessToken(
		identity,
		valueobject.AuthorizationSnapshot{
			Role:        valueobject.RoleOperator,
			Permissions: permissions,
		},
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	parsed, err := manager.ParseAndValidate(token)
	if err != nil {
		t.Fatalf("ParseAndValidate() error = %v", err)
	}
	if generated.CredentialVersion != identity.CredentialVersion ||
		parsed.CredentialVersion != identity.CredentialVersion {
		t.Fatalf(
			"credential versions = generated %d parsed %d, want %d",
			generated.CredentialVersion,
			parsed.CredentialVersion,
			identity.CredentialVersion,
		)
	}
	if parsed.Role != valueobject.RoleOperator {
		t.Fatalf("parsed role = %s, want operator", parsed.Role)
	}
	wantPermissions := []valueobject.Permission{
		valueobject.PermUserRead,
		valueobject.PermUserUpdate,
	}
	if got := parsed.Permissions.Values(); !reflect.DeepEqual(got, wantPermissions) {
		t.Fatalf("parsed permissions = %v, want %v", got, wantPermissions)
	}
	if got := parsed.ExpiresAt.Sub(parsed.IssuedAt); got != 5*time.Minute {
		t.Fatalf("access lifetime = %v, want 5m", got)
	}
}

func TestRefreshTokenCarriesIdentityVersionWithoutAuthorizationSnapshot(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	manager := newJWTManagerWithClock("test-secret", func() time.Time { return now })
	identity := valueobject.TokenIdentity{
		UserID:            "user-1",
		SessionID:         "session-1",
		JTI:               "jti-1",
		CredentialVersion: 4,
	}

	token, _, err := manager.GenerateRefreshToken(identity, time.Hour)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	parsed, err := manager.ParseAndValidate(token)
	if err != nil {
		t.Fatalf("ParseAndValidate() error = %v", err)
	}
	if !parsed.IsRefresh() || parsed.SessionID != identity.SessionID ||
		parsed.CredentialVersion != identity.CredentialVersion {
		t.Fatalf("parsed refresh claims = %#v, want identity %#v", parsed, identity)
	}
	if parsed.Role != valueobject.RoleUnknown || len(parsed.Permissions.Values()) != 0 {
		t.Fatalf(
			"refresh token contains authorization snapshot: role=%s permissions=%v",
			parsed.Role,
			parsed.Permissions.Values(),
		)
	}
}

func TestMaximumAccessTokenStaysBelowFourKiB(t *testing.T) {
	manager := NewJWTManager("test-secret")
	permissions, err := valueobject.NewPermissionSet(valueobject.AllPermissions())
	if err != nil {
		t.Fatalf("build maximum permission set: %v", err)
	}
	token, _, err := manager.GenerateAccessToken(
		valueobject.TokenIdentity{
			UserID:            "11111111-1111-4111-8111-111111111111",
			SessionID:         "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			JTI:               "01ARZ3NDEKTSV4RRFFQ69G5FAW",
			CredentialVersion: 9223372036854775807,
		},
		valueobject.AuthorizationSnapshot{
			Role:        valueobject.RoleAdmin,
			Permissions: permissions,
		},
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	got := len("Bearer " + token)
	t.Logf("maximum bearer token length = %d bytes", got)
	if limit := 4 * 1024; got >= limit {
		t.Fatalf("maximum bearer token length = %d bytes, want < %d", got, limit)
	}
}

func TestAccessTokenEncodingIsDeterministicAndContainsOnlySecurityClaims(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	manager := newJWTManagerWithClock("test-secret", func() time.Time { return now })
	permissions, err := valueobject.NewPermissionSet([]valueobject.Permission{
		valueobject.PermUserWrite,
		valueobject.PermUserRead,
		valueobject.PermUserWrite,
	})
	if err != nil {
		t.Fatalf("build permission set: %v", err)
	}
	identity := valueobject.TokenIdentity{
		UserID: "user-1", SessionID: "session-1", JTI: "jti-1", CredentialVersion: 4,
	}
	snapshot := valueobject.AuthorizationSnapshot{
		Role: valueobject.RoleOperator, Permissions: permissions,
	}

	first, _, err := manager.GenerateAccessToken(identity, snapshot, 5*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken(first) error = %v", err)
	}
	second, _, err := manager.GenerateAccessToken(identity, snapshot, 5*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken(second) error = %v", err)
	}
	if first != second {
		t.Fatal("identical access-token inputs produced non-deterministic encoding")
	}

	parts := strings.Split(first, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT segment count = %d, want 3", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode JWT payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("decode JWT claims: %v", err)
	}
	allowed := map[string]bool{
		"user_id": true, "session_id": true, "credential_version": true,
		"token_type": true, "role": true, "permissions": true,
		"iss": true, "sub": true, "aud": true, "exp": true, "nbf": true,
		"iat": true, "jti": true,
	}
	for claim := range claims {
		if !allowed[claim] {
			t.Errorf("access token contains non-security claim %q", claim)
		}
	}
	for _, forbidden := range []string{
		"name", "email", "phone", "avatar", "preferences", "created_at", "updated_at",
	} {
		if _, exists := claims[forbidden]; exists {
			t.Errorf("access token contains mutable profile claim %q", forbidden)
		}
	}
}

func TestAccessTokenExpiresAtConfiguredBoundaryWithoutSleeping(t *testing.T) {
	current := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	manager := newJWTManagerWithClock("test-secret", func() time.Time { return current })
	permissions, err := valueobject.NewPermissionSet(nil)
	if err != nil {
		t.Fatalf("build permission set: %v", err)
	}
	token, _, err := manager.GenerateAccessToken(
		valueobject.TokenIdentity{
			UserID: "user-1", SessionID: "session-1", JTI: "jti-1", CredentialVersion: 4,
		},
		valueobject.AuthorizationSnapshot{
			Role: valueobject.RoleClient, Permissions: permissions,
		},
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	current = current.Add(5*time.Minute - time.Second)
	if _, err := manager.ParseAndValidate(token); err != nil {
		t.Fatalf("ParseAndValidate(before expiry) error = %v", err)
	}
	current = current.Add(time.Second)
	if _, err := manager.ParseAndValidate(token); !errors.Is(err, domainErrors.ErrTokenExpired) {
		t.Fatalf("ParseAndValidate(at expiry) error = %v, want ErrTokenExpired", err)
	}
}

func TestTamperedAuthorizationSnapshotFailsSignatureValidation(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	manager := newJWTManagerWithClock("test-secret", func() time.Time { return now })
	permissions, err := valueobject.NewPermissionSet([]valueobject.Permission{
		valueobject.PermUserRead,
	})
	if err != nil {
		t.Fatalf("build permission set: %v", err)
	}
	token, _, err := manager.GenerateAccessToken(
		valueobject.TokenIdentity{
			UserID: "user-1", SessionID: "session-1", JTI: "jti-1", CredentialVersion: 4,
		},
		valueobject.AuthorizationSnapshot{
			Role: valueobject.RoleClient, Permissions: permissions,
		},
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	parts := strings.Split(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	claims["permissions"] = []string{valueobject.PermFullAccess.String()}
	payload, err = json.Marshal(claims)
	if err != nil {
		t.Fatalf("encode tampered claims: %v", err)
	}
	parts[1] = base64.RawURLEncoding.EncodeToString(payload)

	if _, err := manager.ParseAndValidate(strings.Join(parts, ".")); !errors.Is(err, domainErrors.ErrTokenInvalid) {
		t.Fatalf("ParseAndValidate(tampered snapshot) error = %v, want ErrTokenInvalid", err)
	}
}

func TestSignedAccessTokenRejectsUnknownPermissionClaim(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	manager := newJWTManagerWithClock("test-secret", func() time.Time { return now })
	claims := tokenClaims{
		UserID:            "user-1",
		SessionID:         "session-1",
		CredentialVersion: 4,
		TokenType:         string(valueobject.TokenTypeAccess),
		Role:              valueobject.RoleClient.String(),
		Permissions:       []string{"unknown:permission"},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: valueobject.TokenIssuer, Subject: string(valueobject.TokenTypeAccess),
			Audience:  jwt.ClaimStrings{valueobject.TokenAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now), IssuedAt: jwt.NewNumericDate(now), ID: "jti-1",
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(manager.secret)
	if err != nil {
		t.Fatalf("sign malformed snapshot: %v", err)
	}
	if _, err := manager.ParseAndValidate(token); !errors.Is(err, domainErrors.ErrTokenInvalid) {
		t.Fatalf("ParseAndValidate(unknown permission) error = %v, want ErrTokenInvalid", err)
	}
}
