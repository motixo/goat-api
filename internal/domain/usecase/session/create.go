package session

import (
	"context"
	"time"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type CreateInput struct {
	UserID     string
	Device     string
	IP         string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	CurrentJTI string
}

func (s *SessionUseCase) Create(ctx context.Context, input CreateInput) error {

	session := &entity.Session{
		ID:         s.ulidGen.New(),
		UserID:     input.UserID,
		CurrentJTI: input.CurrentJTI,
		IP:         input.IP,
		Device:     input.Device,
		ExpiresAt:  input.ExpiresAt,
		CreatedAt:  time.Now().UTC(),
	}
	return s.sessionRepo.CreateSession(ctx, session)
}
