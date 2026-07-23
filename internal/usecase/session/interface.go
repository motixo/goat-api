package session

import (
	"context"
)

type UseCase interface {
	CreateSession(ctx context.Context, input CreateInput) error
	GetSessionsByUser(ctx context.Context, userID, sessionID string, offset, limit int) ([]SessionOutput, int64, error)
	DeleteSessions(ctx context.Context, input DeleteSessionsInput) error
	RotateSessionJTI(ctx context.Context, input RotateInput) (string, error)
	ValidateSession(ctx context.Context, input ValidateInput) (bool, error)
}

type CredentialVersionReader interface {
	GetCredentialVersion(ctx context.Context, userID string) (int64, error)
}
