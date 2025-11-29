package session

import (
	"context"
	"time"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type CreateInput struct {
	UserID       string
	Device       string
	IP           string
	CurrentJTI   string
	JTIExpiresAt time.Time
}

func (s *SessionUseCase) CreateSession(ctx context.Context, input CreateInput) (string, error) {

	now := time.Now().UTC()
	session := &entity.Session{
		ID:                s.ulidGen.New(),
		UserID:            input.UserID,
		CurrentJTI:        input.CurrentJTI,
		IP:                input.IP,
		Device:            input.Device,
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         now.Add(365 * 24 * time.Hour),
		JTITTLSeconds:     int(time.Until(input.JTIExpiresAt).Seconds()),
		SessionTTLSeconds: int(time.Until(now.Add(365 * 24 * time.Hour)).Seconds()),
	}
	err := s.sessionRepo.Create(ctx, session)
	if err != nil {
		return "", err
	}
	return session.ID, nil

}
