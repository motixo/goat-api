package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
	"github.com/motixo/goat-api/internal/domain/usecase/user"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func (us *AuthUseCase) Signup(ctx context.Context, input RegisterInput) (RegisterOutput, error) {
	us.logger.Info("signup attempt", "email", input.Email)
	hashedPassword, err := us.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		us.logger.Error("failed to hash password", "email", input.Email, "error", err)
		return RegisterOutput{}, err
	}

	rq := &entity.User{
		ID:        uuid.New().String(),
		Email:     input.Email,
		Password:  hashedPassword,
		Status:    valueobject.StatusActive,
		Role:      valueobject.RoleClient,
		CreatedAt: time.Now().UTC(),
	}

	err = us.userRepo.Create(ctx, rq)
	if err != nil {
		us.logger.Error("failed to create user", "email", input.Email, "error", err)
		return RegisterOutput{}, err
	}

	us.logger.Info("user registered successfully", "userID", rq.ID, "email", rq.Email)
	return RegisterOutput{
		User: user.UserResponse{
			ID:        rq.ID,
			Email:     rq.Email,
			Role:      rq.Role.String(),
			CreatedAt: rq.CreatedAt,
		},
	}, nil
}

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

	access, accessClaims, err := us.jwtService.GenerateAccessToken(userEntity.ID, sessionID, refreshJTI, us.accessTTL)
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
			Status:    userEntity.Status.String(),
			CreatedAt: userEntity.CreatedAt,
		},
	}, nil
}

func (us *AuthUseCase) Logout(ctx context.Context, sessionID, userID string) error {

	us.logger.Info("user logout requested", "userID", userID)

	input := session.DeleteSessionsInput{
		TargetSessions: []string{sessionID},
		UserID:         userID,
	}

	err := us.sessionUC.DeleteSessions(ctx, input)
	if err != nil {
		return err
	}
	us.logger.Info("user logged out", "userID", userID)
	return nil
}

func (us *AuthUseCase) Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error) {

	claims, err := us.jwtService.ParseAndValidate(input.RefreshToken)
	if err != nil {
		us.logger.Warn("invalid refresh token", "error", err)
		return RefreshOutput{}, errors.ErrUnauthorized
	}

	if claims.TokenType != valueobject.TokenTypeRefresh {
		us.logger.Warn("refresh token with wrong type", "userID", claims.UserID, "tokenType", claims.TokenType)
		return RefreshOutput{}, errors.ErrUnauthorized
	}

	us.logger.Debug("refresh token requested", "userID", claims.UserID, "ip", input.IP, "device", input.Device)

	refreshJTI := us.ulidGen.Generate()
	refresh, refreshClaims, err := us.jwtService.GenerateRefreshToken(claims.UserID, refreshJTI, us.refreshTTL)
	if err != nil {
		us.logger.Error("failed to create refresh token", "userID", claims.UserID, "error", err)
		return RefreshOutput{}, err
	}

	rotateInput := session.RotateInput{
		OldJTI:     claims.JTI,
		CurrentJTI: refreshJTI,
		Device:     input.Device,
		IP:         input.IP,
		JTITTL:     us.refreshTTL,
		SessionTTL: us.sessionTTL,
	}

	sessionID, err := us.sessionUC.RotateSessionJTI(ctx, rotateInput)
	if err != nil {
		return RefreshOutput{}, err
	}

	access, accessClaims, err := us.jwtService.GenerateAccessToken(claims.UserID, sessionID, refreshJTI, us.accessTTL)
	if err != nil {
		us.logger.Error("failed to create access token", "userID", claims.UserID, "error", err)
		return RefreshOutput{}, err
	}

	us.logger.Info("user refresh token successful", "userID", claims.UserID, "oldJTI", claims.JTI, "newJTI", refreshJTI)

	return RefreshOutput{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessClaims.GetExpiresAt(),
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshClaims.GetExpiresAt(),
	}, nil
}
