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

func TestValidateSessionMatchesSignedAndRedisSnapshots(t *testing.T) {
	sessions := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "jti-1",
			CredentialVersion: 5,
			SessionGeneration: 1,
		},
	}
	usecase := NewUsecase(sessions, discardSessionLogger{})

	valid, err := usecase.ValidateSession(context.Background(), ValidateInput{
		UserID:            "user-1",
		SessionID:         "session-1",
		JTI:               "jti-1",
		CredentialVersion: 5,
	})

	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if !valid {
		t.Fatal("ValidateSession() = false for matching signed and Redis snapshots")
	}
	if sessions.findCalls != 1 {
		t.Fatalf("FindByJTI calls = %d, want 1", sessions.findCalls)
	}
}

func TestValidateSessionRejectsCredentialVersionMismatch(t *testing.T) {
	sessions := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "jti-1",
			CredentialVersion: 5,
			SessionGeneration: 1,
		},
	}
	usecase := NewUsecase(sessions, discardSessionLogger{})

	valid, err := usecase.ValidateSession(context.Background(), ValidateInput{
		UserID:            "user-1",
		SessionID:         "session-1",
		JTI:               "jti-1",
		CredentialVersion: 4,
	})

	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if valid {
		t.Fatal("ValidateSession() = true for mismatched credential-version snapshots")
	}
}

func TestValidateSessionFailsClosedOnRedisFailure(t *testing.T) {
	lookupErr := errors.New("redis unavailable")
	sessions := &credentialVersionSessionRepository{findErr: lookupErr}
	usecase := NewUsecase(sessions, discardSessionLogger{})

	valid, err := usecase.ValidateSession(context.Background(), ValidateInput{
		UserID:            "user-1",
		SessionID:         "session-1",
		JTI:               "jti-1",
		CredentialVersion: 5,
	})

	if valid {
		t.Fatal("ValidateSession() = true after Redis failure")
	}
	if !errors.Is(err, lookupErr) {
		t.Fatalf("ValidateSession() error = %v, want Redis failure", err)
	}
}

func TestValidateSessionRejectsSignedIdentityMismatch(t *testing.T) {
	tests := []struct {
		name  string
		input ValidateInput
	}{
		{
			name: "foreign user",
			input: ValidateInput{
				UserID:            "user-2",
				SessionID:         "session-1",
				JTI:               "jti-1",
				CredentialVersion: 5,
			},
		},
		{
			name: "foreign session",
			input: ValidateInput{
				UserID:            "user-1",
				SessionID:         "session-2",
				JTI:               "jti-1",
				CredentialVersion: 5,
			},
		},
		{
			name: "foreign JTI",
			input: ValidateInput{
				UserID:            "user-1",
				SessionID:         "session-1",
				JTI:               "jti-2",
				CredentialVersion: 5,
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
					SessionGeneration: 1,
				},
			}
			usecase := NewUsecase(sessions, discardSessionLogger{})

			valid, err := usecase.ValidateSession(context.Background(), test.input)

			if err != nil {
				t.Fatalf("ValidateSession() error = %v", err)
			}
			if valid {
				t.Fatal("ValidateSession() = true for mismatched signed identity")
			}
		})
	}
}

func TestCreateSessionSnapshotsCredentialVersion(t *testing.T) {
	sessions := &credentialVersionSessionRepository{}
	usecase := NewUsecase(sessions, discardSessionLogger{})

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
	usecase := NewUsecase(sessions, discardSessionLogger{})

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
			SessionGeneration: 1,
		},
	}
	usecase := NewUsecase(sessions, discardSessionLogger{})

	sessionID, err := usecase.RotateSessionJTI(context.Background(), RotateInput{
		UserID:            "user-1",
		SessionID:         "session-1",
		CredentialVersion: 8,
		OldJTI:            "old-jti",
		CurrentJTI:        "new-jti",
		SessionTTL:        time.Hour,
		JTITTL:            time.Minute,
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

func TestRotateSessionJTIRejectsMissingCredentialVersionWithoutRedisMutation(t *testing.T) {
	sessions := &credentialVersionSessionRepository{}
	usecase := NewUsecase(sessions, discardSessionLogger{})

	_, err := usecase.RotateSessionJTI(context.Background(), RotateInput{
		UserID:     "user-1",
		SessionID:  "session-1",
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
	findCalls                int
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

func (r *credentialVersionSessionRepository) FindByJTI(
	context.Context,
	string,
	string,
	string,
	int64,
) (*entity.Session, error) {
	r.findCalls++
	return r.session, r.findErr
}

func (r *credentialVersionSessionRepository) RotateJTI(
	_ context.Context,
	_, _ string,
	expectedUserID, _ string,
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
