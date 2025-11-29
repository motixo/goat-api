package session

import (
	"context"
	"time"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type UseCase interface {
	CreateSession(ctx context.Context, input CreateInput) (string, error)
	GetSessionsByUser(ctx context.Context, userID, sessionID string) ([]*SessionResponse, error)
	DeleteSessions(ctx context.Context, input DeleteSessionsInput) error
	RotateSessionJTI(ctx context.Context, input RotateInput) (string, error)
	IsJTIValid(ctx context.Context, jti string) (bool, error)
}

type Repository interface {
	Create(ctx context.Context, s *entity.Session) error
	ListByUser(ctx context.Context, userID string) ([]*entity.Session, error)
	Delete(ctx context.Context, sessionIDs []string) error
	RotateJTI(ctx context.Context, oldJTI, newJTI, ip, device string, expiresAt time.Time, jtiTTLSeconds, sessionTTLSeconds int) (string, error)
	ExistsJTI(ctx context.Context, jti string) (bool, error)
}
