package session

import (
	"context"
)

type UseCase interface {
	CreateSession(ctx context.Context, input CreateInput) error
	GetSessionsByUser(ctx context.Context, userID, sessionID string, offset, limit int) ([]*SessionResponse, int64, error)
	DeleteSessions(ctx context.Context, input DeleteSessionsInput) error
	RotateSessionJTI(ctx context.Context, input RotateInput) (string, error)
	IsJTIValid(ctx context.Context, jti string) (bool, error)
}
