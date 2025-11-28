package session

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type UseCase interface {
	Create(ctx context.Context, input CreateInput) error
	Get(ctx context.Context, sessionID string) (*entity.Session, error)
	ListUserSessions(ctx context.Context, userID string) ([]*entity.Session, error)
	Delete(ctx context.Context, sessionID string) error
	UpdateJTI(ctx context.Context, sessionID, jti string, ttlSeconds int) error
}

type Repository interface {
	CreateSession(ctx context.Context, s *entity.Session) error
	GetSession(ctx context.Context, sessionID string) (*entity.Session, error)
	ListSessionsByUser(ctx context.Context, userID string) ([]*entity.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	UpdateSessionJTI(ctx context.Context, sessionID, jti string, ttlSeconds int) error
}
