package session

import (
	"context"
	"time"

	"github.com/mot0x0/gopi/internal/domain/errors"
)

type RotateInput struct {
	OldJTI       string
	CurrentJTI   string
	Device       string
	IP           string
	ExpiresAt    time.Time
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

	sessionID, err := s.sessionRepo.RotateJTI(
		ctx,
		input.OldJTI,
		input.CurrentJTI,
		input.Device,
		input.IP,
		int(time.Until(input.JTIExpiresAt).Seconds()),
		int(time.Until(input.ExpiresAt).Seconds()),
	)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}
