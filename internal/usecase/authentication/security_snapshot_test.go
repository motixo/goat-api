package authentication

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/authorization"
	"github.com/motixo/goat-api/internal/usecase/session"
)

func TestLoginBuildsAccessSnapshotFromAuthoritativeSecurityState(t *testing.T) {
	permissions := authPermissionSet(t, valueobject.PermUserWrite, valueobject.PermUserRead)
	user := &entity.User{
		ID:                "user-1",
		Email:             "user@example.com",
		PasswordDigest:    testPasswordDigest("$hash"),
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: 4,
		CreatedAt:         time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC),
	}
	users := &securitySnapshotUserRepository{user: user}
	states := &securitySnapshotStateReader{state: authorization.SecurityState{
		UserID:            user.ID,
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleOperator,
		CredentialVersion: user.CredentialVersion,
		Permissions:       permissions,
	}}
	sessions := &securitySnapshotSessionUseCase{}
	tokens := &securitySnapshotJWTService{}
	usecase := NewUsecase(Dependencies{
		UserRepository:      users,
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		PasswordHasher:      securitySnapshotPasswordHasher{valid: true},
		JWTService:          tokens,
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(24 * time.Hour),
		SessionTTL:          SessionTTL(30 * 24 * time.Hour),
	})

	output, err := usecase.Login(context.Background(), LoginInput{
		Email: user.Email, Password: "Password1!", IP: "127.0.0.1", Device: "test",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if states.calls != 1 {
		t.Fatalf("authoritative security-state reads = %d, want 1", states.calls)
	}
	if sessions.createCalls != 1 {
		t.Fatalf("session creates = %d, want 1", sessions.createCalls)
	}
	if tokens.accessCalls != 1 || tokens.refreshCalls != 1 {
		t.Fatalf("token generation calls: access=%d refresh=%d, want 1 each", tokens.accessCalls, tokens.refreshCalls)
	}
	if tokens.accessIdentity.CredentialVersion != user.CredentialVersion ||
		tokens.refreshIdentity.CredentialVersion != user.CredentialVersion {
		t.Fatalf("credential-version snapshot was not copied to both tokens")
	}
	if tokens.accessIdentity != tokens.refreshIdentity {
		t.Fatalf("access and refresh identities differ: access=%#v refresh=%#v", tokens.accessIdentity, tokens.refreshIdentity)
	}
	if tokens.accessSnapshot.Role != valueobject.RoleOperator {
		t.Fatalf("access role = %s, want operator", tokens.accessSnapshot.Role)
	}
	if got := tokens.accessSnapshot.Permissions.Values(); len(got) != 2 ||
		got[0] != valueobject.PermUserRead || got[1] != valueobject.PermUserWrite {
		t.Fatalf("access permissions = %v, want stable sorted snapshot", got)
	}
	if output.User.Role != valueobject.RoleOperator.String() {
		t.Fatalf("login user role = %q, want authoritative role %q", output.User.Role, valueobject.RoleOperator)
	}
	if sessions.createInput.CredentialVersion != user.CredentialVersion {
		t.Fatalf("session credential version = %d, want %d", sessions.createInput.CredentialVersion, user.CredentialVersion)
	}
}

func TestLoginPropagatesPasswordVerificationAdmissionFailure(t *testing.T) {
	verificationErr := errors.New("password verification admission canceled")
	user := &entity.User{
		ID:                "user-1",
		Email:             "user@example.com",
		PasswordDigest:    testPasswordDigest("$hash"),
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: 1,
	}
	states := &securitySnapshotStateReader{}
	sessions := &securitySnapshotSessionUseCase{}
	tokens := &securitySnapshotJWTService{}
	usecase := NewUsecase(Dependencies{
		UserRepository:      &securitySnapshotUserRepository{user: user},
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		PasswordHasher:      securitySnapshotPasswordHasher{err: verificationErr},
		JWTService:          tokens,
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(time.Hour),
		SessionTTL:          SessionTTL(24 * time.Hour),
	})

	_, err := usecase.Login(context.Background(), LoginInput{
		Email: user.Email, Password: "Password1!",
	})
	if !errors.Is(err, verificationErr) {
		t.Fatalf("Login() error = %v, want password verification failure", err)
	}
	if states.calls != 0 || sessions.createCalls != 0 ||
		tokens.accessCalls != 0 || tokens.refreshCalls != 0 {
		t.Fatalf(
			"failed password admission performed security work: state=%d session=%d access=%d refresh=%d",
			states.calls,
			sessions.createCalls,
			tokens.accessCalls,
			tokens.refreshCalls,
		)
	}
}

func TestLoginTreatsInvalidHashLookingInputAsPlaintextWithoutLeakingPolicy(t *testing.T) {
	verifyCalls := 0
	user := &entity.User{
		ID:                "user-1",
		Email:             "user@example.com",
		PasswordDigest:    testPasswordDigest("$stored-digest"),
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: 1,
	}
	usecase := NewUsecase(Dependencies{
		UserRepository: &securitySnapshotUserRepository{user: user},
		PasswordHasher: securitySnapshotPasswordHasher{valid: true, calls: &verifyCalls},
		Logger:         discardAuthLogger{},
	})

	_, err := usecase.Login(context.Background(), LoginInput{
		Email:    user.Email,
		Password: "$argon2id$",
	})
	if !errors.Is(err, domainErrors.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want privacy-preserving ErrInvalidCredentials", err)
	}
	if errors.Is(err, domainErrors.ErrPasswordPolicyViolation) {
		t.Fatalf("Login() exposed plaintext policy details: %v", err)
	}
	if verifyCalls != 0 {
		t.Fatalf("password verifications = %d, want 0 for invalid plaintext", verifyCalls)
	}
}

func TestLoginKeepsInvalidStoredPasswordHashPrivateAndObservable(t *testing.T) {
	const (
		plaintext  = "Password1!"
		storedHash = "$argon2id$corrupted-credential-material"
	)
	verificationErr := fmt.Errorf(
		"validate persisted credential: %w",
		service.ErrInvalidStoredPasswordHash,
	)
	user := &entity.User{
		ID:                "user-1",
		Email:             "user@example.com",
		PasswordDigest:    testPasswordDigest(storedHash),
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: 1,
	}
	states := &securitySnapshotStateReader{}
	sessions := &securitySnapshotSessionUseCase{}
	tokens := &securitySnapshotJWTService{}
	logger := &passwordVerificationLogRecorder{}
	usecase := NewUsecase(Dependencies{
		UserRepository:      &securitySnapshotUserRepository{user: user},
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		PasswordHasher:      securitySnapshotPasswordHasher{err: verificationErr},
		JWTService:          tokens,
		Logger:              logger,
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(time.Hour),
		SessionTTL:          SessionTTL(24 * time.Hour),
	})

	_, err := usecase.Login(context.Background(), LoginInput{
		Email: user.Email, Password: plaintext,
	})
	if !errors.Is(err, domainErrors.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want privacy-preserving ErrInvalidCredentials", err)
	}
	if !errors.Is(err, service.ErrInvalidStoredPasswordHash) {
		t.Fatalf("Login() error = %v, want retained invalid stored-hash identity", err)
	}
	if states.calls != 0 || sessions.createCalls != 0 ||
		tokens.accessCalls != 0 || tokens.refreshCalls != 0 {
		t.Fatalf(
			"invalid stored hash performed security work: state=%d session=%d access=%d refresh=%d",
			states.calls,
			sessions.createCalls,
			tokens.accessCalls,
			tokens.refreshCalls,
		)
	}
	if len(logger.errors) != 1 {
		t.Fatalf("password credential error logs = %d, want 1", len(logger.errors))
	}
	logged := logger.errors[0]
	if !strings.Contains(logged, "stored password credential is invalid") {
		t.Fatalf("password credential error log = %q, want observable safe message", logged)
	}
	for _, secret := range []string{plaintext, storedHash} {
		if strings.Contains(logged, secret) {
			t.Fatal("password credential error log exposed plaintext or encoded hash")
		}
	}
}

func TestLoginRejectsStateChangedAfterPasswordVerification(t *testing.T) {
	user := &entity.User{
		ID:                "user-1",
		Email:             "user@example.com",
		PasswordDigest:    testPasswordDigest("$old-hash"),
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: 4,
	}
	states := &securitySnapshotStateReader{state: authorization.SecurityState{
		UserID:            user.ID,
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: 5,
		Permissions:       authPermissionSet(t, valueobject.PermUserRead),
	}}
	sessions := &securitySnapshotSessionUseCase{}
	tokens := &securitySnapshotJWTService{}
	usecase := NewUsecase(Dependencies{
		UserRepository:      &securitySnapshotUserRepository{user: user},
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		PasswordHasher:      securitySnapshotPasswordHasher{valid: true},
		JWTService:          tokens,
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(time.Hour),
		SessionTTL:          SessionTTL(24 * time.Hour),
	})

	_, err := usecase.Login(context.Background(), LoginInput{
		Email: user.Email, Password: "Password1!",
	})
	if !errors.Is(err, domainErrors.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if sessions.createCalls != 0 || tokens.accessCalls != 0 || tokens.refreshCalls != 0 {
		t.Fatalf(
			"state-changing login created security artifacts: sessions=%d access=%d refresh=%d",
			sessions.createCalls,
			tokens.accessCalls,
			tokens.refreshCalls,
		)
	}
}

func TestLoginRefusesCurrentInactiveOrSuspendedStateWithoutIssuingTokens(t *testing.T) {
	for _, test := range []struct {
		status    valueobject.UserStatus
		wantError error
	}{
		{
			status:    valueobject.StatusInactive,
			wantError: authorization.ErrPrincipalInactive,
		},
		{
			status:    valueobject.StatusSuspended,
			wantError: domainErrors.ErrAccountSuspended,
		},
	} {
		t.Run(test.status.String(), func(t *testing.T) {
			user := &entity.User{
				ID: "user-1", Email: "user@example.com",
				PasswordDigest: testPasswordDigest("$hash"),
				Status:         valueobject.StatusActive, Role: valueobject.RoleClient,
				CredentialVersion: 4,
			}
			states := &securitySnapshotStateReader{state: authorization.SecurityState{
				UserID: user.ID, Status: test.status, Role: valueobject.RoleClient,
				CredentialVersion: user.CredentialVersion,
				Permissions:       authPermissionSet(t, valueobject.PermUserRead),
			}}
			sessions := &securitySnapshotSessionUseCase{}
			tokens := &securitySnapshotJWTService{}
			usecase := NewUsecase(Dependencies{
				UserRepository:      &securitySnapshotUserRepository{user: user},
				SecurityStateReader: states,
				SessionUseCase:      sessions,
				PasswordHasher:      securitySnapshotPasswordHasher{valid: true},
				JWTService:          tokens,
				Logger:              discardAuthLogger{},
				AccessTTL:           AccessTTL(5 * time.Minute),
				RefreshTTL:          RefreshTTL(time.Hour),
				SessionTTL:          SessionTTL(24 * time.Hour),
			})

			_, err := usecase.Login(context.Background(), LoginInput{
				Email: user.Email, Password: "Password1!",
			})
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Login() error = %v, want %v", err, test.wantError)
			}
			if sessions.createCalls != 0 || tokens.accessCalls != 0 || tokens.refreshCalls != 0 {
				t.Fatalf(
					"login issued security artifacts for %s user: sessions=%d access=%d refresh=%d",
					test.status,
					sessions.createCalls,
					tokens.accessCalls,
					tokens.refreshCalls,
				)
			}
		})
	}
}

func TestLoginRejectsPersistedInactiveUserBeforePasswordOrSessionWork(t *testing.T) {
	user := &entity.User{
		ID:                "user-1",
		Email:             "inactive@example.com",
		PasswordDigest:    testPasswordDigest("$hash"),
		Status:            valueobject.StatusInactive,
		Role:              valueobject.RoleClient,
		CredentialVersion: 1,
	}
	states := &securitySnapshotStateReader{}
	sessions := &securitySnapshotSessionUseCase{}
	tokens := &securitySnapshotJWTService{}
	usecase := NewUsecase(Dependencies{
		UserRepository:      &securitySnapshotUserRepository{user: user},
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		PasswordHasher:      securitySnapshotPasswordHasher{valid: true},
		JWTService:          tokens,
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(time.Hour),
		SessionTTL:          SessionTTL(24 * time.Hour),
	})

	_, err := usecase.Login(context.Background(), LoginInput{
		Email: user.Email, Password: "Password1!",
	})

	if !errors.Is(err, authorization.ErrPrincipalInactive) {
		t.Fatalf("Login() error = %v, want ErrPrincipalInactive", err)
	}
	if states.calls != 0 || sessions.createCalls != 0 ||
		tokens.accessCalls != 0 || tokens.refreshCalls != 0 {
		t.Fatalf(
			"inactive login performed security work: state=%d session=%d access=%d refresh=%d",
			states.calls,
			sessions.createCalls,
			tokens.accessCalls,
			tokens.refreshCalls,
		)
	}
}

func TestLoginUnknownStatusFailsClosedWithoutIssuingSecurityArtifacts(t *testing.T) {
	user := &entity.User{
		ID:                "user-1",
		Email:             "unknown-status@example.com",
		PasswordDigest:    testPasswordDigest("$hash"),
		Status:            valueobject.StatusUnknown,
		Role:              valueobject.RoleClient,
		CredentialVersion: 1,
	}
	states := &securitySnapshotStateReader{}
	sessions := &securitySnapshotSessionUseCase{}
	tokens := &securitySnapshotJWTService{}
	usecase := NewUsecase(Dependencies{
		UserRepository:      &securitySnapshotUserRepository{user: user},
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		PasswordHasher:      securitySnapshotPasswordHasher{valid: true},
		JWTService:          tokens,
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(time.Hour),
		SessionTTL:          SessionTTL(24 * time.Hour),
	})

	_, err := usecase.Login(context.Background(), LoginInput{
		Email: user.Email, Password: "Password1!",
	})

	if err == nil {
		t.Fatal("Login() error = nil for unknown status")
	}
	if errors.Is(err, authorization.ErrPrincipalInactive) ||
		errors.Is(err, domainErrors.ErrAccountSuspended) {
		t.Fatalf("Login() misclassified unknown status as a known account state: %v", err)
	}
	if states.calls != 0 || sessions.createCalls != 0 ||
		tokens.accessCalls != 0 || tokens.refreshCalls != 0 {
		t.Fatalf(
			"unknown-status login performed security work: state=%d session=%d access=%d refresh=%d",
			states.calls,
			sessions.createCalls,
			tokens.accessCalls,
			tokens.refreshCalls,
		)
	}
}

func TestLoginRejectsRedisBlockedUserAfterAuthoritativeStateWasLoaded(t *testing.T) {
	user := &entity.User{
		ID:                "user-1",
		Email:             "user@example.com",
		PasswordDigest:    testPasswordDigest("$hash"),
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: 4,
	}
	states := &securitySnapshotStateReader{state: authorization.SecurityState{
		UserID:            user.ID,
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: user.CredentialVersion,
		Permissions:       authPermissionSet(t, valueobject.PermUserRead),
	}}
	sessions := &securitySnapshotSessionUseCase{createErr: domainErrors.ErrUserAccessBlocked}
	tokens := &securitySnapshotJWTService{}
	usecase := NewUsecase(Dependencies{
		UserRepository:      &securitySnapshotUserRepository{user: user},
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		PasswordHasher:      securitySnapshotPasswordHasher{valid: true},
		JWTService:          tokens,
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(time.Hour),
		SessionTTL:          SessionTTL(24 * time.Hour),
	})

	output, err := usecase.Login(context.Background(), LoginInput{
		Email: user.Email, Password: "Password1!",
	})
	if !errors.Is(err, domainErrors.ErrAccountSuspended) {
		t.Fatalf("Login() error = %v, want ErrAccountSuspended", err)
	}
	if output.AccessToken != "" || output.RefreshToken != "" {
		t.Fatalf("Login() returned tokens after Redis access block: %#v", output)
	}
	if states.calls != 1 || sessions.createCalls != 1 {
		t.Fatalf("state reads = %d, session creates = %d, want 1 each", states.calls, sessions.createCalls)
	}
	if tokens.accessCalls != 0 {
		t.Fatalf("access-token generations = %d, want 0 after blocked session creation", tokens.accessCalls)
	}
}

func TestRefreshUsesOneAuthoritativeStateReadAndRebuildsSnapshot(t *testing.T) {
	permissions := authPermissionSet(t, valueobject.PermUserDelete, valueobject.PermUserRead)
	states := &securitySnapshotStateReader{state: authorization.SecurityState{
		UserID:            "user-1",
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleAdmin,
		CredentialVersion: 9,
		Permissions:       permissions,
	}}
	sessions := &securitySnapshotSessionUseCase{valid: true, rotatedSessionID: "session-1"}
	tokens := &securitySnapshotJWTService{parsed: &valueobject.JWTClaims{
		UserID:            "user-1",
		SessionID:         "session-1",
		CredentialVersion: 9,
		TokenType:         valueobject.TokenTypeRefresh,
		JTI:               "old-jti",
	}}
	usecase := NewUsecase(Dependencies{
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		JWTService:          tokens,
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(24 * time.Hour),
		SessionTTL:          SessionTTL(30 * 24 * time.Hour),
	})

	output, err := usecase.Refresh(context.Background(), RefreshInput{
		RefreshToken: "refresh-token", IP: "127.0.0.1", Device: "test",
	})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if output.AccessToken == "" || output.RefreshToken == "" {
		t.Fatalf("Refresh() returned empty tokens: %#v", output)
	}
	if sessions.validateCalls != 1 {
		t.Fatalf("Redis validations = %d, want 1", sessions.validateCalls)
	}
	if states.calls != 1 {
		t.Fatalf("authoritative PostgreSQL security-state reads = %d, want 1", states.calls)
	}
	if sessions.rotateCalls != 1 {
		t.Fatalf("atomic JTI rotations = %d, want 1", sessions.rotateCalls)
	}
	if tokens.accessSnapshot.Role != valueobject.RoleAdmin ||
		!tokens.accessSnapshot.Permissions.Has(valueobject.PermUserDelete) {
		t.Fatalf(
			"refreshed access snapshot = role %s permissions %v",
			tokens.accessSnapshot.Role,
			tokens.accessSnapshot.Permissions.Values(),
		)
	}
	if tokens.accessIdentity != tokens.refreshIdentity {
		t.Fatalf("refreshed token identities differ: access=%#v refresh=%#v", tokens.accessIdentity, tokens.refreshIdentity)
	}
	if sessions.rotateInput.CredentialVersion != 9 ||
		sessions.rotateInput.OldJTI != "old-jti" ||
		sessions.rotateInput.CurrentJTI != tokens.accessIdentity.JTI {
		t.Fatalf("rotation input = %#v, want validated identity and new JTI", sessions.rotateInput)
	}
}

func TestRefreshFailsClosedBeforeRotationForCurrentSecurityState(t *testing.T) {
	postgresErr := errors.New("postgres unavailable")
	tests := []struct {
		name      string
		state     authorization.SecurityState
		stateErr  error
		wantError error
	}{
		{
			name:      "deleted",
			stateErr:  domainErrors.ErrUserNotFound,
			wantError: domainErrors.ErrUserNotFound,
		},
		{
			name: "inactive",
			state: authorization.SecurityState{
				UserID: "user-1", Status: valueobject.StatusInactive,
				Role: valueobject.RoleClient, CredentialVersion: 9,
			},
			wantError: authorization.ErrPrincipalInactive,
		},
		{
			name: "suspended",
			state: authorization.SecurityState{
				UserID: "user-1", Status: valueobject.StatusSuspended,
				Role: valueobject.RoleClient, CredentialVersion: 9,
			},
			wantError: domainErrors.ErrAccountSuspended,
		},
		{
			name: "credential version changed",
			state: authorization.SecurityState{
				UserID: "user-1", Status: valueobject.StatusActive,
				Role: valueobject.RoleClient, CredentialVersion: 10,
			},
			wantError: domainErrors.ErrUnauthorized,
		},
		{
			name:      "unknown postgres failure",
			stateErr:  postgresErr,
			wantError: postgresErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			states := &securitySnapshotStateReader{state: test.state, err: test.stateErr}
			sessions := &securitySnapshotSessionUseCase{valid: true, rotatedSessionID: "session-1"}
			tokens := &securitySnapshotJWTService{parsed: &valueobject.JWTClaims{
				UserID: "user-1", SessionID: "session-1", CredentialVersion: 9,
				TokenType: valueobject.TokenTypeRefresh, JTI: "old-jti",
			}}
			usecase := NewUsecase(Dependencies{
				SecurityStateReader: states,
				SessionUseCase:      sessions,
				JWTService:          tokens,
				Logger:              discardAuthLogger{},
				AccessTTL:           AccessTTL(5 * time.Minute),
				RefreshTTL:          RefreshTTL(time.Hour),
				SessionTTL:          SessionTTL(24 * time.Hour),
			})

			_, err := usecase.Refresh(context.Background(), RefreshInput{RefreshToken: "refresh-token"})
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Refresh() error = %v, want %v", err, test.wantError)
			}
			if states.calls != 1 {
				t.Fatalf("authoritative state reads = %d, want 1", states.calls)
			}
			if sessions.rotateCalls != 0 {
				t.Fatalf("JTI rotations = %d, want 0 after failed current-state check", sessions.rotateCalls)
			}
			if tokens.accessCalls != 0 || tokens.refreshCalls != 0 {
				t.Fatalf("tokens generated after failed current-state check: access=%d refresh=%d", tokens.accessCalls, tokens.refreshCalls)
			}
		})
	}
}

func TestRefreshUnknownStatusFailsClosedWithoutRotationOrTokenIssuance(t *testing.T) {
	states := &securitySnapshotStateReader{state: authorization.SecurityState{
		UserID:            "user-1",
		Status:            valueobject.StatusUnknown,
		Role:              valueobject.RoleClient,
		CredentialVersion: 9,
	}}
	sessions := &securitySnapshotSessionUseCase{
		valid:            true,
		rotatedSessionID: "session-1",
	}
	tokens := &securitySnapshotJWTService{parsed: &valueobject.JWTClaims{
		UserID:            "user-1",
		SessionID:         "session-1",
		CredentialVersion: 9,
		TokenType:         valueobject.TokenTypeRefresh,
		JTI:               "old-jti",
	}}
	usecase := NewUsecase(Dependencies{
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		JWTService:          tokens,
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(time.Hour),
		SessionTTL:          SessionTTL(24 * time.Hour),
	})

	_, err := usecase.Refresh(
		context.Background(),
		RefreshInput{RefreshToken: "refresh-token"},
	)

	if err == nil {
		t.Fatal("Refresh() error = nil for unknown status")
	}
	if sessions.rotateCalls != 0 || tokens.accessCalls != 0 || tokens.refreshCalls != 0 {
		t.Fatalf(
			"unknown-status refresh mutated security state: rotate=%d access=%d refresh=%d",
			sessions.rotateCalls,
			tokens.accessCalls,
			tokens.refreshCalls,
		)
	}
}

func TestRefreshDoesNotQueryPostgreSQLAfterRedisValidationFailure(t *testing.T) {
	redisErr := errors.New("redis unavailable")
	tests := []struct {
		name      string
		valid     bool
		redisErr  error
		wantError error
	}{
		{name: "revoked session", wantError: domainErrors.ErrUnauthorized},
		{
			name:      "blocked user",
			redisErr:  domainErrors.ErrUserAccessBlocked,
			wantError: domainErrors.ErrAccountSuspended,
		},
		{name: "Redis failure", redisErr: redisErr, wantError: redisErr},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			states := &securitySnapshotStateReader{}
			sessions := &securitySnapshotSessionUseCase{
				valid: test.valid, validateErr: test.redisErr,
			}
			tokens := &securitySnapshotJWTService{parsed: &valueobject.JWTClaims{
				UserID: "user-1", SessionID: "session-1", CredentialVersion: 9,
				TokenType: valueobject.TokenTypeRefresh, JTI: "old-jti",
			}}
			usecase := NewUsecase(Dependencies{
				SecurityStateReader: states,
				SessionUseCase:      sessions,
				JWTService:          tokens,
				Logger:              discardAuthLogger{},
				AccessTTL:           AccessTTL(5 * time.Minute),
				RefreshTTL:          RefreshTTL(time.Hour),
				SessionTTL:          SessionTTL(24 * time.Hour),
			})

			_, err := usecase.Refresh(context.Background(), RefreshInput{RefreshToken: "refresh-token"})
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Refresh() error = %v, want %v", err, test.wantError)
			}
			if states.calls != 0 {
				t.Fatalf("PostgreSQL security-state reads = %d, want 0 after Redis rejection", states.calls)
			}
			if sessions.rotateCalls != 0 {
				t.Fatalf("JTI rotations = %d, want 0 after Redis rejection", sessions.rotateCalls)
			}
		})
	}
}

func TestRefreshRejectsRedisBlockDuringAtomicRotation(t *testing.T) {
	states := &securitySnapshotStateReader{state: authorization.SecurityState{
		UserID:            "user-1",
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: 9,
		Permissions:       authPermissionSet(t, valueobject.PermUserRead),
	}}
	sessions := &securitySnapshotSessionUseCase{
		valid:     true,
		rotateErr: domainErrors.ErrUserAccessBlocked,
	}
	tokens := &securitySnapshotJWTService{parsed: &valueobject.JWTClaims{
		UserID:            "user-1",
		SessionID:         "session-1",
		CredentialVersion: 9,
		TokenType:         valueobject.TokenTypeRefresh,
		JTI:               "old-jti",
	}}
	usecase := NewUsecase(Dependencies{
		SecurityStateReader: states,
		SessionUseCase:      sessions,
		JWTService:          tokens,
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(5 * time.Minute),
		RefreshTTL:          RefreshTTL(time.Hour),
		SessionTTL:          SessionTTL(24 * time.Hour),
	})

	output, err := usecase.Refresh(context.Background(), RefreshInput{RefreshToken: "refresh-token"})
	if !errors.Is(err, domainErrors.ErrAccountSuspended) {
		t.Fatalf("Refresh() error = %v, want ErrAccountSuspended", err)
	}
	if output.AccessToken != "" || output.RefreshToken != "" {
		t.Fatalf("Refresh() returned tokens after Redis access block: %#v", output)
	}
	if states.calls != 1 || sessions.rotateCalls != 1 {
		t.Fatalf("state reads = %d, rotations = %d, want 1 each", states.calls, sessions.rotateCalls)
	}
}

type securitySnapshotUserRepository struct {
	repository.UserRepository
	user *entity.User
	err  error
}

func (r *securitySnapshotUserRepository) FindByEmail(context.Context, string) (*entity.User, error) {
	return r.user, r.err
}

type securitySnapshotStateReader struct {
	state authorization.SecurityState
	err   error
	calls int
}

func (r *securitySnapshotStateReader) GetSecurityState(
	context.Context,
	string,
) (authorization.SecurityState, error) {
	r.calls++
	return r.state, r.err
}

type securitySnapshotSessionUseCase struct {
	session.UseCase
	createInput      session.CreateInput
	createCalls      int
	createErr        error
	validateInput    session.ValidateInput
	validateCalls    int
	valid            bool
	validateErr      error
	rotateInput      session.RotateInput
	rotateCalls      int
	rotatedSessionID string
	rotateErr        error
}

func (s *securitySnapshotSessionUseCase) CreateSession(
	_ context.Context,
	input session.CreateInput,
) error {
	s.createCalls++
	s.createInput = input
	return s.createErr
}

func (s *securitySnapshotSessionUseCase) ValidateSession(
	_ context.Context,
	input session.ValidateInput,
) (bool, error) {
	s.validateCalls++
	s.validateInput = input
	return s.valid, s.validateErr
}

func (s *securitySnapshotSessionUseCase) RotateSessionJTI(
	_ context.Context,
	input session.RotateInput,
) (string, error) {
	s.rotateCalls++
	s.rotateInput = input
	return s.rotatedSessionID, s.rotateErr
}

type securitySnapshotPasswordHasher struct {
	service.PasswordHasher
	valid bool
	err   error
	calls *int
}

type passwordVerificationLogRecorder struct {
	errors []string
}

func (l *passwordVerificationLogRecorder) Info(string, ...any) {}

func (l *passwordVerificationLogRecorder) Error(message string, fields ...any) {
	l.errors = append(l.errors, message+fmt.Sprint(fields...))
}

func (l *passwordVerificationLogRecorder) Warn(string, ...any)  {}
func (l *passwordVerificationLogRecorder) Debug(string, ...any) {}
func (l *passwordVerificationLogRecorder) Panic(string, ...any) {}

func (h securitySnapshotPasswordHasher) Verify(
	context.Context,
	valueobject.PlainPassword,
	valueobject.PasswordDigest,
) (bool, error) {
	if h.calls != nil {
		(*h.calls)++
	}
	return h.valid, h.err
}

type securitySnapshotJWTService struct {
	service.JWTService
	parsed          *valueobject.JWTClaims
	parseErr        error
	accessIdentity  valueobject.TokenIdentity
	accessSnapshot  valueobject.AuthorizationSnapshot
	accessDuration  time.Duration
	accessCalls     int
	refreshIdentity valueobject.TokenIdentity
	refreshDuration time.Duration
	refreshCalls    int
}

func (s *securitySnapshotJWTService) ParseAndValidate(string) (*valueobject.JWTClaims, error) {
	return s.parsed, s.parseErr
}

func (s *securitySnapshotJWTService) GenerateAccessToken(
	identity valueobject.TokenIdentity,
	snapshot valueobject.AuthorizationSnapshot,
	duration time.Duration,
) (string, *valueobject.JWTClaims, error) {
	s.accessCalls++
	s.accessIdentity = identity
	s.accessSnapshot = snapshot
	s.accessDuration = duration
	return "access-token", &valueobject.JWTClaims{
		ExpiresAt: time.Date(2026, 7, 23, 12, 5, 0, 0, time.UTC),
	}, nil
}

func (s *securitySnapshotJWTService) GenerateRefreshToken(
	identity valueobject.TokenIdentity,
	duration time.Duration,
) (string, *valueobject.JWTClaims, error) {
	s.refreshCalls++
	s.refreshIdentity = identity
	s.refreshDuration = duration
	return "refresh-token", &valueobject.JWTClaims{
		ExpiresAt: time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
	}, nil
}

func authPermissionSet(
	t testing.TB,
	permissions ...valueobject.Permission,
) valueobject.PermissionSet {
	t.Helper()
	set, err := valueobject.NewPermissionSet(permissions)
	if err != nil {
		t.Fatalf("build permission set: %v", err)
	}
	return set
}

func testPasswordDigest(encoded string) valueobject.PasswordDigest {
	digest, err := valueobject.NewPasswordDigest(encoded)
	if err != nil {
		panic("test password digest is invalid")
	}
	return digest
}
