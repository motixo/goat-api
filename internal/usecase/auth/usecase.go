package auth

import (
	"context"
	stdErrors "errors"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/session"
)

type AuthUseCase struct {
	userRepo       repository.UserRepository
	sessionUC      session.UseCase
	passwordHasher service.PasswordHasher
	jwtService     service.JWTService
	userCache      service.UserCacheService
	logger         pkg.Logger
	accessTTL      time.Duration
	refreshTTL     time.Duration
	sessionTTL     time.Duration
}

func NewUsecase(
	userRepo repository.UserRepository,
	sessionUC session.UseCase,
	passwordHasher service.PasswordHasher,
	jwtService service.JWTService,
	userCache service.UserCacheService,
	logger pkg.Logger,
	accessTTL AccessTTL,
	refreshTTL RefreshTTL,
	sessionTTL SessionTTL,

) UseCase {
	return &AuthUseCase{
		userRepo:       userRepo,
		sessionUC:      sessionUC,
		passwordHasher: passwordHasher,
		jwtService:     jwtService,
		userCache:      userCache,
		logger:         logger,
		accessTTL:      time.Duration(accessTTL),
		refreshTTL:     time.Duration(refreshTTL),
		sessionTTL:     time.Duration(sessionTTL),
	}
}

func (us *AuthUseCase) Signup(ctx context.Context, input RegisterInput) (UserOutput, error) {
	us.logger.Info("signup attempt", "email", input.Email)
	hashedPassword, err := us.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		us.logger.Error("failed to hash password", "email", input.Email, "error", err)
		return UserOutput{}, err
	}

	usr := &entity.User{
		ID:        uuid.New().String(),
		Email:     input.Email,
		Password:  hashedPassword,
		Status:    valueobject.StatusActive,
		Role:      valueobject.RoleClient,
		CreatedAt: time.Now().UTC(),
	}

	err = us.userRepo.Create(ctx, usr)
	if err != nil {
		us.logger.Error("failed to create user", "email", input.Email, "error", err)
		return UserOutput{}, err
	}

	us.logger.Info("user registered successfully", "userID", usr.ID, "email", usr.Email)
	return UserOutput{
		ID:        usr.ID,
		Email:     usr.Email,
		Role:      usr.Role.String(),
		Status:    usr.Status.String(),
		CreatedAt: usr.CreatedAt,
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
		return LoginOutput{}, domainErrors.ErrNotFound
	}

	if userEntity.Status != valueobject.StatusActive {
		us.logger.Warn("login failed: user account suspended", "email", input.Email)
		return LoginOutput{}, domainErrors.ErrAccountSuspended
	}

	if !us.passwordHasher.Verify(ctx, input.Password, userEntity.Password) {
		us.logger.Warn("login failed: invalid password", "email", input.Email, "ip", input.IP, "device", input.Device)
		return LoginOutput{}, domainErrors.ErrInvalidCredentials
	}

	refreshJTI := pkg.ULIDGenerator()
	refresh, refreshClaims, err := us.jwtService.GenerateRefreshToken(userEntity.ID, refreshJTI, us.refreshTTL)
	if err != nil {
		us.logger.Error("failed to create refresh token", "userID", userEntity.ID, "error", err)
		return LoginOutput{}, err
	}

	sessionID := pkg.ULIDGenerator()
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
		if cleanupErr := us.sessionUC.DeleteSessions(ctx, session.DeleteSessionsInput{
			UserID:         userEntity.ID,
			TargetSessions: []string{sessionID},
		}); cleanupErr != nil {
			us.logger.Error("failed to clean up session after access token generation failure", "userID", userEntity.ID, "error", cleanupErr)
		}
		return LoginOutput{}, err
	}

	us.logger.Info("user logged in successfully", "userID", userEntity.ID, "refreshJTI", refreshJTI, "sessionID", sessionID)

	return LoginOutput{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessClaims.GetExpiresAt(),
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshClaims.GetExpiresAt(),
		User: UserOutput{
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
		if stdErrors.Is(err, domainErrors.ErrNotFound) {
			return NewCurrentSessionInvalidError(err)
		}
		return err
	}
	us.logger.Info("user logged out", "userID", userID)
	return nil
}

func (us *AuthUseCase) Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error) {

	claims, err := us.jwtService.ParseAndValidate(input.RefreshToken)
	if err != nil {
		us.logger.Error("invalid refresh token", "error", err)
		return RefreshOutput{}, domainErrors.ErrUnauthorized
	}

	if claims.TokenType != valueobject.TokenTypeRefresh {
		us.logger.Error("refresh token with wrong type", "userID", claims.UserID, "tokenType", claims.TokenType)
		return RefreshOutput{}, domainErrors.ErrUnauthorized
	}

	role, err := us.userCache.GetUserStatus(ctx, claims.UserID)
	if err != nil {
		us.logger.Error("failed to fetch user status", "userID", claims.UserID)
		return RefreshOutput{}, err
	}
	if role == valueobject.StatusSuspended {
		return RefreshOutput{}, domainErrors.ErrAccountSuspended
	}
	us.logger.Debug("refresh token requested", "userID", claims.UserID, "ip", input.IP, "device", input.Device)

	refreshJTI := pkg.ULIDGenerator()
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
