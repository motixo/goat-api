package auth

import (
	"context"
	"time"

	"github.com/mot0x0/goth-api/internal/domain/errors"
	"github.com/mot0x0/goth-api/internal/domain/usecase/session"
	"github.com/mot0x0/goth-api/internal/domain/usecase/user"
	"github.com/mot0x0/goth-api/internal/domain/valueobject"
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
	a.logger.Info("login attempt", "email", input.Email, "ip", input.IP, "device", input.Device)
	u, err := a.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		a.logger.Error("login failed", "error", err)
		return LoginOutput{}, err
	}
	if u == nil {
		a.logger.Warn("login failed: user not found", "email", input.Email)
		return LoginOutput{}, errors.ErrNotFound
	}

	if !a.passwordHasher.Verify(ctx, input.Password, valueobject.PasswordFromHash(u.Password)) {
		a.logger.Warn("login failed: invalid password", "email", input.Email, "ip", input.IP, "device", input.Device)
		return LoginOutput{}, errors.ErrUnauthorized
	}

	refreshJTI := a.ulidGen.New()
	refresh, refreshExp, err := valueobject.NewRefreshToken(u.ID, a.jwtSecret, refreshJTI)
	if err != nil {
		a.logger.Error("failed to create refresh token", "userID", u.ID, "error", err)
		return LoginOutput{}, err
	}

	sessionInput := session.CreateInput{
		UserID:       u.ID,
		CurrentJTI:   refreshJTI,
		IP:           input.IP,
		Device:       input.Device,
		JTIExpiresAt: refreshExp,
	}

	sesseionID, err := a.sessionUC.CreateSession(ctx, sessionInput)
	if err != nil {
		return LoginOutput{}, err
	}

	access, accessExp, err := valueobject.NewAccessToken(u.ID, a.jwtSecret, sesseionID, refreshJTI)
	if err != nil {
		a.logger.Error("failed to create access token", "userID", u.ID, "error", err)
		return LoginOutput{}, err
	}

	a.logger.Info("user logged in successfully", "userID", u.ID, "refreshJTI", refreshJTI, "sessionID", sesseionID)

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
