package session

import (
	"context"
	"time"

	"github.com/motixo/goth-api/internal/domain/entity"
)

func (us *SessionUseCase) CreateSession(ctx context.Context, input CreateInput) (string, error) {
	us.logger.Debug("creating session", "userID", input.UserID, "device", input.Device, "ip", input.IP, "currentJTI", input.CurrentJTI)

	now := time.Now().UTC()
	expiresAt := now.Add(input.SessionTTL)

	session := &entity.Session{
		ID:                us.ulidGen.New(),
		UserID:            input.UserID,
		CurrentJTI:        input.CurrentJTI,
		IP:                input.IP,
		Device:            input.Device,
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         expiresAt,
		JTITTLSeconds:     int64(input.JTITTL.Seconds()),
		SessionTTLSeconds: int64(input.SessionTTL.Seconds()),
	}
	err := us.sessionRepo.Create(ctx, session)
	if err != nil {
		us.logger.Error("failed to create session", "userID", input.UserID, "currentJTI", input.CurrentJTI, "error", err)
		return "", err
	}

	us.logger.Info("session created successfully", "userID", input.UserID, "sessionID", session.ID, "currentJTI", input.CurrentJTI)
	return session.ID, nil

}
