package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
)

func TestValidateSessionRejectsOlderCredentialVersion(t *testing.T) {
	sessions := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "jti-1",
			CredentialVersion: 4,
		},
	}
	users := &credentialVersionUserRepository{version: 5}
	usecase := NewUsecase(sessions, users, discardSessionLogger{})

	valid, err := usecase.ValidateSession(context.Background(), ValidateInput{
		UserID:    "user-1",
		SessionID: "session-1",
		JTI:       "jti-1",
	})

	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if valid {
		t.Fatal("ValidateSession() = true for an older credential version")
	}
	if users.calls != 1 {
		t.Fatalf("authoritative credential-version reads = %d, want 1", users.calls)
	}
}

func TestValidateSessionAllowsMatchingCredentialVersion(t *testing.T) {
	sessions := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "jti-1",
			CredentialVersion: 5,
		},
	}
	users := &credentialVersionUserRepository{version: 5}
	usecase := NewUsecase(sessions, users, discardSessionLogger{})

	valid, err := usecase.ValidateSession(context.Background(), ValidateInput{
		UserID:    "user-1",
		SessionID: "session-1",
		JTI:       "jti-1",
	})

	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if !valid {
		t.Fatal("ValidateSession() = false for matching credential versions")
	}
}

func TestValidateSessionFailsClosedOnAuthoritativeLookupFailure(t *testing.T) {
	lookupErr := errors.New("postgres unavailable")
	sessions := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "jti-1",
			CredentialVersion: 5,
		},
	}
	users := &credentialVersionUserRepository{err: lookupErr}
	usecase := NewUsecase(sessions, users, discardSessionLogger{})

	valid, err := usecase.ValidateSession(context.Background(), ValidateInput{
		UserID:    "user-1",
		SessionID: "session-1",
		JTI:       "jti-1",
	})

	if valid {
		t.Fatal("ValidateSession() = true after authoritative lookup failure")
	}
	if !errors.Is(err, lookupErr) {
		t.Fatalf("ValidateSession() error = %v, want PostgreSQL failure", err)
	}
}

func TestValidateSessionTreatsMissingAuthoritativeUserAsInvalid(t *testing.T) {
	sessions := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "jti-1",
			CredentialVersion: 5,
		},
	}
	users := &credentialVersionUserRepository{err: domainErrors.ErrUserNotFound}
	usecase := NewUsecase(sessions, users, discardSessionLogger{})

	valid, err := usecase.ValidateSession(context.Background(), ValidateInput{
		UserID:    "user-1",
		SessionID: "session-1",
		JTI:       "jti-1",
	})

	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if valid {
		t.Fatal("ValidateSession() = true for a deleted authoritative user")
	}
}

func TestValidateSessionRejectsClaimIdentityMismatchBeforePostgreSQL(t *testing.T) {
	tests := []struct {
		name  string
		input ValidateInput
	}{
		{
			name: "foreign user",
			input: ValidateInput{
				UserID:    "user-2",
				SessionID: "session-1",
				JTI:       "jti-1",
			},
		},
		{
			name: "foreign session",
			input: ValidateInput{
				UserID:    "user-1",
				SessionID: "session-2",
				JTI:       "jti-1",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sessions := &credentialVersionSessionRepository{
				session: &entity.Session{
					ID:                "session-1",
					UserID:            "user-1",
					CurrentJTI:        "jti-1",
					CredentialVersion: 5,
				},
			}
			users := &credentialVersionUserRepository{version: 5}
			usecase := NewUsecase(sessions, users, discardSessionLogger{})

			valid, err := usecase.ValidateSession(context.Background(), test.input)

			if err != nil {
				t.Fatalf("ValidateSession() error = %v", err)
			}
			if valid {
				t.Fatal("ValidateSession() = true for mismatched signed identity")
			}
			if users.calls != 0 {
				t.Fatalf("authoritative version reads = %d, want 0 for mismatched Redis identity", users.calls)
			}
		})
	}
}

func TestCreateSessionSnapshotsCredentialVersion(t *testing.T) {
	sessions := &credentialVersionSessionRepository{}
	usecase := NewUsecase(sessions, &credentialVersionUserRepository{}, discardSessionLogger{})

	err := usecase.CreateSession(context.Background(), CreateInput{
		ID:                "session-1",
		UserID:            "user-1",
		CurrentJTI:        "jti-1",
		CredentialVersion: 7,
		SessionTTL:        time.Hour,
		JTITTL:            time.Minute,
	})

	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if sessions.created == nil || sessions.created.CredentialVersion != 7 {
		t.Fatalf("stored credential version = %#v, want 7", sessions.created)
	}
}

func TestCreateSessionRejectsMissingCredentialVersion(t *testing.T) {
	sessions := &credentialVersionSessionRepository{}
	usecase := NewUsecase(sessions, &credentialVersionUserRepository{}, discardSessionLogger{})

	err := usecase.CreateSession(context.Background(), CreateInput{
		ID:         "session-1",
		UserID:     "user-1",
		CurrentJTI: "jti-1",
		SessionTTL: time.Hour,
		JTITTL:     time.Minute,
	})

	if err == nil {
		t.Fatal("CreateSession() error = nil for missing credential version")
	}
	if sessions.created != nil {
		t.Fatalf("session repository received invalid session: %#v", sessions.created)
	}
}

func TestRotateSessionJTIPreservesValidatedCredentialVersion(t *testing.T) {
	sessions := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "old-jti",
			CredentialVersion: 8,
		},
	}
	users := &credentialVersionUserRepository{version: 8}
	usecase := NewUsecase(sessions, users, discardSessionLogger{})

	sessionID, err := usecase.RotateSessionJTI(context.Background(), RotateInput{
		UserID:     "user-1",
		OldJTI:     "old-jti",
		CurrentJTI: "new-jti",
		SessionTTL: time.Hour,
		JTITTL:     time.Minute,
	})

	if err != nil {
		t.Fatalf("RotateSessionJTI() error = %v", err)
	}
	if sessionID != "session-1" {
		t.Fatalf("rotated session ID = %q, want session-1", sessionID)
	}
	if sessions.rotatedCredentialVersion != 8 {
		t.Fatalf("rotation credential version = %d, want 8", sessions.rotatedCredentialVersion)
	}
	if sessions.rotatedUserID != "user-1" {
		t.Fatalf("rotation user ID = %q, want user-1", sessions.rotatedUserID)
	}
}

func TestRotateSessionJTIRejectsOldCredentialVersionWithoutRedisMutation(t *testing.T) {
	sessions := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "old-jti",
			CredentialVersion: 8,
		},
	}
	users := &credentialVersionUserRepository{version: 9}
	usecase := NewUsecase(sessions, users, discardSessionLogger{})

	_, err := usecase.RotateSessionJTI(context.Background(), RotateInput{
		UserID:     "user-1",
		OldJTI:     "old-jti",
		CurrentJTI: "new-jti",
		SessionTTL: time.Hour,
		JTITTL:     time.Minute,
	})

	if !errors.Is(err, domainErrors.ErrUnauthorized) {
		t.Fatalf("RotateSessionJTI() error = %v, want ErrUnauthorized", err)
	}
	if sessions.rotateCalls != 0 {
		t.Fatalf("RotateJTI calls = %d, want 0", sessions.rotateCalls)
	}
}

type credentialVersionSessionRepository struct {
	repository.SessionRepository
	session                  *entity.Session
	findErr                  error
	created                  *entity.Session
	rotateCalls              int
	rotatedUserID            string
	rotatedCredentialVersion int64
}

func (r *credentialVersionSessionRepository) Create(_ context.Context, session *entity.Session) error {
	copy := *session
	r.created = &copy
	return nil
}

func (r *credentialVersionSessionRepository) FindByJTI(context.Context, string) (*entity.Session, error) {
	return r.session, r.findErr
}

func (r *credentialVersionSessionRepository) RotateJTI(
	_ context.Context,
	_, _ string,
	expectedUserID string,
	expectedCredentialVersion int64,
	_, _ string,
	_ time.Time,
	_, _ int64,
) (string, error) {
	r.rotateCalls++
	r.rotatedUserID = expectedUserID
	r.rotatedCredentialVersion = expectedCredentialVersion
	return r.session.ID, nil
}

type credentialVersionUserRepository struct {
	repository.UserRepository
	version int64
	err     error
	calls   int
}

func (r *credentialVersionUserRepository) GetCredentialVersion(context.Context, string) (int64, error) {
	r.calls++
	return r.version, r.err
}
