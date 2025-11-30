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

	valid, err := s.sessionRepo.ExistsJTI(ctx, input.OldJTI)
	if err != nil {
		return "", err
	}
	if !valid {
		return "", errors.ErrUnauthorized
	}

	now := time.Now().UTC()
	ExpiresAt := now.Add(365 * 24 * time.Hour)

	sessionID, err := s.sessionRepo.RotateJTI(
		ctx,
		input.OldJTI,
		input.CurrentJTI,
		input.IP,
		input.Device,
		ExpiresAt,
		int(time.Until(input.JTIExpiresAt).Seconds()),
		int(time.Until(ExpiresAt).Seconds()),
	)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}
