package session

import (
	"context"
	"time"

	"github.com/mot0x0/goth-api/internal/domain/errors"
)

type RotateInput struct {
	OldJTI       string
	CurrentJTI   string
	Device       string
	IP           string
	JTIExpiresAt time.Time
}

func (s *SessionUseCase) RotateSessionJTI(ctx context.Context, input RotateInput) (string, error) {
	s.logger.Debug("rotating session JTI", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "ip", input.IP, "device", input.Device)
	valid, err := s.sessionRepo.ExistsJTI(ctx, input.OldJTI)
	if err != nil {
		s.logger.Error("failed to check if JTI exists", "oldJTI", input.OldJTI, "error", err)
		return "", err
	}
	if !valid {
		s.logger.Warn("attempt to rotate non-existent or expired JTI", "oldJTI", input.OldJTI, "ip", input.IP, "device", input.Device)
		return "", errors.ErrUnauthorized
	}

	now := time.Now().UTC()
	expiresAt := now.Add(365 * 24 * time.Hour)

	sessionID, err := s.sessionRepo.RotateJTI(
		ctx,
		input.OldJTI,
		input.CurrentJTI,
		input.IP,
		input.Device,
		expiresAt,
		int(time.Until(input.JTIExpiresAt).Seconds()),
		int(time.Until(expiresAt).Seconds()),
	)
	if err != nil {
		s.logger.Error("failed to rotate JTI", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "ip", input.IP, "device", input.Device, "error", err)
		return "", err
	}
	s.logger.Info("session JTI rotated successfully", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "sessionID", sessionID)
	return sessionID, nil
}
