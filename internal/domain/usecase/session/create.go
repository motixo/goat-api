package session

import (
	"context"
	"time"

	"github.com/mot0x0/goth-api/internal/domain/entity"
)

type CreateInput struct {
	UserID       string
	Device       string
	IP           string
	CurrentJTI   string
	JTIExpiresAt time.Time
}

func (s *SessionUseCase) CreateSession(ctx context.Context, input CreateInput) (string, error) {
	s.logger.Debug("creating session", "userID", input.UserID, "device", input.Device, "ip", input.IP, "currentJTI", input.CurrentJTI)

	now := time.Now().UTC()
	expiresAt := now.Add(365 * 24 * time.Hour)

	session := &entity.Session{
		ID:                s.ulidGen.New(),
		UserID:            input.UserID,
		CurrentJTI:        input.CurrentJTI,
		IP:                input.IP,
		Device:            input.Device,
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         expiresAt,
		JTITTLSeconds:     int(time.Until(input.JTIExpiresAt).Seconds()),
		SessionTTLSeconds: int(time.Until(expiresAt).Seconds()),
	}
	err := s.sessionRepo.Create(ctx, session)
	if err != nil {
		s.logger.Error("failed to create session", "userID", input.UserID, "currentJTI", input.CurrentJTI, "error", err)
		return "", err
	}

	s.logger.Info("session created successfully", "userID", input.UserID, "sessionID", session.ID, "currentJTI", input.CurrentJTI)
	return session.ID, nil

}
