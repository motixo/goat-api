package session

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/pagination"
)

type UseCase interface {
	CreateSession(ctx context.Context, input CreateInput) error
	GetSessionsByUser(ctx context.Context, userID, sessionID string, p pagination.Input) (*SessionListResponse, error)
	DeleteSessions(ctx context.Context, input DeleteSessionsInput) error
	RotateSessionJTI(ctx context.Context, input RotateInput) (string, error)
	IsJTIValid(ctx context.Context, jti string) (bool, error)
}
