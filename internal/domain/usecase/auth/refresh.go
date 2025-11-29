package auth

import (
	"context"
	"time"

	"github.com/mot0x0/gopi/internal/domain/errors"
	"github.com/mot0x0/gopi/internal/domain/usecase/session"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
	IP           string `json:"-"`
	Device       string `json:"-"`
}

type RefreshOutput struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

func (a *AuthUseCase) Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error) {

	claims, err := valueobject.ParseAndValidate(input.RefreshToken, a.jwtSecret)
	if err != nil {
		return RefreshOutput{}, errors.ErrUnauthorized
	}

	if claims.TokenType != valueobject.TokenTypeRefresh {
		return RefreshOutput{}, errors.ErrUnauthorized
	}

	refreshJTI := a.ulidGen.New()
	refresh, refreshExp, err := valueobject.NewRefreshToken(claims.UserID, a.jwtSecret, refreshJTI)
	if err != nil {
		return RefreshOutput{}, err
	}

	now := time.Now().UTC()
	rotateInput := session.RotateInput{
		OldJTI:       claims.JTI,
		CurrentJTI:   refreshJTI,
		Device:       input.Device,
		IP:           input.IP,
		ExpiresAt:    now.Add(365 * 24 * time.Hour),
		JTIExpiresAt: refreshExp,
	}

	if err := a.sessionUC.RotateSessionJTI(ctx, rotateInput); err != nil {
		return RefreshOutput{}, err
	}

	access, accessExp, err := valueobject.NewAccessToken(claims.UserID, a.jwtSecret, claims.SessionID, refreshJTI)
	if err != nil {
		return RefreshOutput{}, err
	}

	return RefreshOutput{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessExp,
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshExp,
	}, nil
}
