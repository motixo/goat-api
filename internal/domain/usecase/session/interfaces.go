package session

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type SessionUseCase interface {
	Create(ctx context.Context, s *entity.Session) error
	Get(ctx context.Context, sessionID string) (*entity.Session, error)
	ListUserSessions(ctx context.Context, userID string) ([]*entity.Session, error)
	Delete(ctx context.Context, sessionID string) error
	UpdateJTI(ctx context.Context, sessionID, jti string, ttlSeconds int) error
}
