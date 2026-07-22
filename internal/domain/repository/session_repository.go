package repository

import (
	"context"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
)

type SessionRepository interface {
	Create(ctx context.Context, s *entity.Session) error
	ListByUser(ctx context.Context, userID string, page, pageSize int) ([]*entity.Session, int64, error)
	Delete(ctx context.Context, sessionIDs []string) error
	DeleteByUser(ctx context.Context, userID string, sessionIDs []string) (bool, error)
	// DeleteOthersByUser returns false without mutation when currentSessionID is not owned by userID.
	DeleteOthersByUser(ctx context.Context, userID, currentSessionID string) (bool, error)
	RotateJTI(ctx context.Context, oldJTI, newJTI, ip, device string, expiresAt time.Time, jtiTTL, sessionTTL int64) (string, error)
	ExistsJTI(ctx context.Context, jti string) (bool, error)
	CleanOrphanSessions(ctx context.Context) error
}
