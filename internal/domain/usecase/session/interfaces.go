package session

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/dto"
	"github.com/mot0x0/gopi/internal/domain/entity"
)

type UseCase interface {
	CreateSession(ctx context.Context, input CreateInput) (string, error)
	GetSession(ctx context.Context, sessionID string) (*entity.Session, error)
	ListUserSessions(ctx context.Context, userID string) ([]*entity.Session, error)
	DeleteSession(ctx context.Context, sessionID, JTI string) error
	RotateSessionJTI(ctx context.Context, input RotateInput) error
	IsJTIValid(ctx context.Context, jti string) (bool, error)
}

type Repository interface {
	Create(ctx context.Context, s *dto.Session) error
	Get(ctx context.Context, sessionID string) (*entity.Session, error)
	ListByUser(ctx context.Context, userID string) ([]*entity.Session, error)
	Delete(ctx context.Context, sessionID, JTI string) error
	RotateJTI(ctx context.Context, oldJTI, newJTI, ip, device string, jtiTTLSeconds, sessionTTLSeconds int) error
	ExistsJTI(ctx context.Context, jti string) (bool, error)
}
