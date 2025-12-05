package session

import (
	"context"
	"time"

	"github.com/motixo/goat-api/internal/domain/errors"
)

func (us *SessionUseCase) RotateSessionJTI(ctx context.Context, input RotateInput) (string, error) {
	us.logger.Debug("rotating session JTI", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "ip", input.IP, "device", input.Device)
	valid, err := us.sessionRepo.ExistsJTI(ctx, input.OldJTI)
	if err != nil {
		us.logger.Error("failed to check if JTI exists", "oldJTI", input.OldJTI, "error", err)
		return "", err
	}
	if !valid {
		us.logger.Warn("attempt to rotate non-existent or expired JTI", "oldJTI", input.OldJTI, "ip", input.IP, "device", input.Device)
		return "", errors.ErrUnauthorized
	}

	now := time.Now().UTC()
	expiresAt := now.Add(input.SessionTTL)

	sessionID, err := us.sessionRepo.RotateJTI(
		ctx,
		input.OldJTI,
		input.CurrentJTI,
		input.IP,
		input.Device,
		expiresAt,
		int64(input.JTITTL.Seconds()),
		int64(input.SessionTTL.Seconds()),
	)
	if err != nil {
		us.logger.Error("failed to rotate JTI", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "ip", input.IP, "device", input.Device, "error", err)
		return "", err
	}
	us.logger.Info("session JTI rotated successfully", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "sessionID", sessionID)
	return sessionID, nil
}
