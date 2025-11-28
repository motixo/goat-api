package auth

import (
	"context"
	"time"

	"github.com/mot0x0/gopi/internal/domain/errors"
	"github.com/mot0x0/gopi/internal/domain/usecase/jti"
	"github.com/mot0x0/gopi/internal/domain/usecase/session"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	IP       string `json:"-"`
	Device   string `json:"-"`
}

type LoginOutput struct {
	AccessToken           string            `json:"access_token"`
	AccessTokenExpiresAt  time.Time         `json:"access_token_expires_at"`
	RefreshToken          string            `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time         `json:"refresh_token_expires_at"`
	User                  user.UserResponse `json:"user"`
}

func (a *AuthUseCase) Login(ctx context.Context, input LoginInput) (LoginOutput, error) {
	u, err := a.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		return LoginOutput{}, err
	}
	if u == nil {
		return LoginOutput{}, errors.ErrNotFound
	}

	if !a.passwordService.Verify(ctx, input.Password, valueobject.PasswordFromHash(u.Password)) {
		return LoginOutput{}, errors.ErrUnauthorized
	}

	access, accessExp, err := valueobject.NewAccessToken(u.ID, u.Email, a.jwtSecret, a.ulidGen.New())
	if err != nil {
		return LoginOutput{}, err
	}

	refreshJTI := a.ulidGen.New()
	refresh, refreshExp, err := valueobject.NewRefreshToken(u.ID, u.Email, a.jwtSecret, refreshJTI)
	if err != nil {
		return LoginOutput{}, err
	}

	jtiInput := jti.StoreInput{
		UserID: u.ID,
		JTI:    refreshJTI,
		Exp:    time.Until(refreshExp),
	}

	if err := a.jtiUC.StoreJTI(ctx, jtiInput); err != nil {
		return LoginOutput{}, err
	}

	sessionInput := session.CreateInput{
		UserID:     u.ID,
		CurrentJTI: refreshJTI,
		IP:         input.IP,
		Device:     input.Device,
		ExpiresAt:  refreshExp,
	}
	if err := a.sessionUC.Create(ctx, sessionInput); err != nil {
		return LoginOutput{}, err
	}

	return LoginOutput{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessExp,
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshExp,
		User: user.UserResponse{
			ID:        u.ID,
			Email:     u.Email,
			CreatedAt: u.CreatedAt,
		},
	}, nil
}
