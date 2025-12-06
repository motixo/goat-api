package auth

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

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

	access, accessClaims, err := us.jwtService.GenerateAccessToken(claims.UserRole, claims.UserID, sessionID, refreshJTI, us.accessTTL)
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
