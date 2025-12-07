package auth

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
	"github.com/motixo/goat-api/internal/domain/usecase/user"
)

func (us *AuthUseCase) Login(ctx context.Context, input LoginInput) (LoginOutput, error) {
	us.logger.Info("login attempt", "email", input.Email, "ip", input.IP, "device", input.Device)

	userEntity, err := us.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		us.logger.Error("login failed", "error", err)
		return LoginOutput{}, err
	}
	if userEntity == nil {
		us.logger.Warn("login failed: user not found", "email", input.Email)
		return LoginOutput{}, errors.ErrNotFound
	}

	if !us.passwordHasher.Verify(ctx, input.Password, userEntity.Password) {
		us.logger.Warn("login failed: invalid password", "email", input.Email, "ip", input.IP, "device", input.Device)
		return LoginOutput{}, errors.ErrInvalidCredentials
	}

	refreshJTI := us.ulidGen.Generate()
	refresh, refreshClaims, err := us.jwtService.GenerateRefreshToken(userEntity.ID, refreshJTI, us.refreshTTL)
	if err != nil {
		us.logger.Error("failed to create refresh token", "userID", userEntity.ID, "error", err)
		return LoginOutput{}, err
	}

	sessionID := us.ulidGen.Generate()
	sessionInput := session.CreateInput{
		ID:         sessionID,
		UserID:     userEntity.ID,
		CurrentJTI: refreshJTI,
		IP:         input.IP,
		Device:     input.Device,
		JTITTL:     us.refreshTTL,
		SessionTTL: us.sessionTTL,
	}

	if err := us.sessionUC.CreateSession(ctx, sessionInput); err != nil {
		return LoginOutput{}, err
	}

	access, accessClaims, err := us.jwtService.GenerateAccessToken(int8(userEntity.Role), userEntity.ID, sessionID, refreshJTI, us.accessTTL)
	if err != nil {
		us.logger.Error("failed to create access token", "userID", userEntity.ID, "error", err)
		us.sessionUC.DeleteSessions(ctx, session.DeleteSessionsInput{
			TargetSessions: []string{sessionID},
		})
		return LoginOutput{}, err
	}

	us.logger.Info("user logged in successfully", "userID", userEntity.ID, "refreshJTI", refreshJTI, "sessionID", sessionID)

	return LoginOutput{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessClaims.GetExpiresAt(),
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshClaims.GetExpiresAt(),
		User: user.UserResponse{
			ID:        userEntity.ID,
			Email:     userEntity.Email,
			Role:      userEntity.Role.String(),
			CreatedAt: userEntity.CreatedAt,
		},
	}, nil
}
