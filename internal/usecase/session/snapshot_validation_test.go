package session

import (
	"context"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
)

func TestValidateSessionUsesRedisSnapshotWithoutAuthoritativeLookup(t *testing.T) {
	repository := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "jti-1",
			CredentialVersion: 9,
			SessionGeneration: 1,
		},
	}
	usecase := NewUsecase(repository, discardSessionLogger{})

	valid, err := usecase.ValidateSession(context.Background(), ValidateInput{
		UserID:            "user-1",
		SessionID:         "session-1",
		JTI:               "jti-1",
		CredentialVersion: 9,
	})
	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if !valid {
		t.Fatal("ValidateSession() = false, want true")
	}
	if repository.findCalls != 1 {
		t.Fatalf("Redis session validation calls = %d, want 1", repository.findCalls)
	}
}

func TestValidateSessionRejectsCredentialSnapshotMismatch(t *testing.T) {
	repository := &credentialVersionSessionRepository{
		session: &entity.Session{
			ID:                "session-1",
			UserID:            "user-1",
			CurrentJTI:        "jti-1",
			CredentialVersion: 10,
			SessionGeneration: 1,
		},
	}
	usecase := NewUsecase(repository, discardSessionLogger{})

	valid, err := usecase.ValidateSession(context.Background(), ValidateInput{
		UserID:            "user-1",
		SessionID:         "session-1",
		JTI:               "jti-1",
		CredentialVersion: 9,
	})
	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if valid {
		t.Fatal("ValidateSession() = true for mismatched signed/session credential versions")
	}
}
