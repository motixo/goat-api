package repository

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type SessionRepository interface {
	CreateSession(ctx context.Context, s *entity.Session) error
	GetSession(ctx context.Context, sessionID string) (*entity.Session, error)
	ListSessionsByUser(ctx context.Context, userID string) ([]*entity.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	UpdateSessionJTI(ctx context.Context, sessionID, jti string, ttlSeconds int) error
}
