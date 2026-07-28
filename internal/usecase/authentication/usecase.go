package authentication

import (
	"context"
	stdErrors "errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/authorization"
	"github.com/motixo/goat-api/internal/usecase/session"
)

type AuthenticationUseCase struct {
	userRepo       repository.UserRepository
	securityStates authorization.SecurityStateReader
	sessionUC      session.UseCase
	passwordHasher service.PasswordHasher
	jwtService     service.JWTService
	logger         pkg.Logger
	accessTTL      time.Duration
	refreshTTL     time.Duration
	sessionTTL     time.Duration
}

// Dependencies names each application port and lifetime consumed by the
// authentication workflows. This keeps composition explicit and prevents
// positional wiring mistakes between compatible dependencies.
type Dependencies struct {
	UserRepository      repository.UserRepository
	SecurityStateReader authorization.SecurityStateReader
	SessionUseCase      session.UseCase
	PasswordHasher      service.PasswordHasher
	JWTService          service.JWTService
	Logger              pkg.Logger
	AccessTTL           AccessTTL
	RefreshTTL          RefreshTTL
	SessionTTL          SessionTTL
}

func NewUsecase(dependencies Dependencies) UseCase {
	return &AuthenticationUseCase{
		userRepo:       dependencies.UserRepository,
		securityStates: dependencies.SecurityStateReader,
		sessionUC:      dependencies.SessionUseCase,
		passwordHasher: dependencies.PasswordHasher,
		jwtService:     dependencies.JWTService,
		logger:         dependencies.Logger,
		accessTTL:      time.Duration(dependencies.AccessTTL),
		refreshTTL:     time.Duration(dependencies.RefreshTTL),
		sessionTTL:     time.Duration(dependencies.SessionTTL),
	}
}

func (us *AuthenticationUseCase) Signup(ctx context.Context, input RegisterInput) (UserOutput, error) {
	us.logger.Info("signup attempt", "email", input.Email)
	password, err := valueobject.NewPlainPassword(input.Password)
	if err != nil {
		return UserOutput{}, err
	}
	digest, err := us.passwordHasher.Hash(ctx, password)
	if err != nil {
		us.logger.Error("failed to hash password", "email", input.Email, "error", err)
		return UserOutput{}, err
	}

	usr := &entity.User{
		ID:                uuid.New().String(),
		Email:             input.Email,
		PasswordDigest:    digest,
		Status:            valueobject.StatusInactive,
		Role:              valueobject.RoleClient,
		CredentialVersion: entity.InitialCredentialVersion,
		CreatedAt:         time.Now().UTC(),
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

func (us *AuthenticationUseCase) Login(ctx context.Context, input LoginInput) (LoginOutput, error) {
	us.logger.Info("login attempt", "email", input.Email, "ip", input.IP, "device", input.Device)

	userEntity, err := us.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		us.logger.Error("login failed", "error", err)
		if stdErrors.Is(err, service.ErrInvalidStoredPasswordHash) {
			return LoginOutput{}, fmt.Errorf("%w: %w", domainErrors.ErrInvalidCredentials, err)
		}
		return LoginOutput{}, err
	}
	if userEntity == nil {
		us.logger.Warn("login failed: user not found", "email", input.Email)
		return LoginOutput{}, domainErrors.ErrInvalidCredentials
	}

	if err := authenticationStatusError(userEntity.Status); err != nil {
		us.logger.Warn("login failed: user account unavailable", "email", input.Email)
		return LoginOutput{}, err
	}

	password, err := valueobject.NewPlainPassword(input.Password)
	if err != nil {
		return LoginOutput{}, domainErrors.ErrInvalidCredentials
	}
	passwordMatches, err := us.passwordHasher.Verify(ctx, password, userEntity.PasswordDigest)
	if err != nil {
		if stdErrors.Is(err, service.ErrInvalidStoredPasswordHash) {
			us.logger.Error(
				"login failed: stored password credential is invalid",
				"userID", userEntity.ID,
				"error", err,
			)
			return LoginOutput{}, fmt.Errorf("%w: %w", domainErrors.ErrInvalidCredentials, err)
		}
		return LoginOutput{}, err
	}
	if !passwordMatches {
		us.logger.Warn("login failed: invalid password", "email", input.Email, "ip", input.IP, "device", input.Device)
		return LoginOutput{}, domainErrors.ErrInvalidCredentials
	}

	securityState, err := us.securityStates.GetSecurityState(ctx, userEntity.ID)
	if err != nil {
		if stdErrors.Is(err, domainErrors.ErrUserNotFound) {
			return LoginOutput{}, domainErrors.ErrInvalidCredentials
		}
		return LoginOutput{}, err
	}
	if securityState.UserID != userEntity.ID ||
		securityState.CredentialVersion != userEntity.CredentialVersion {
		return LoginOutput{}, domainErrors.ErrInvalidCredentials
	}
	if err := authenticationStatusError(securityState.Status); err != nil {
		return LoginOutput{}, err
	}

	refreshJTI := pkg.ULIDGenerator()
	sessionID := pkg.ULIDGenerator()
	tokenIdentity := valueobject.TokenIdentity{
		UserID:            securityState.UserID,
		SessionID:         sessionID,
		JTI:               refreshJTI,
		CredentialVersion: securityState.CredentialVersion,
	}
	refresh, refreshClaims, err := us.jwtService.GenerateRefreshToken(
		tokenIdentity,
		us.refreshTTL,
	)
	if err != nil {
		us.logger.Error("failed to create refresh token", "userID", userEntity.ID, "error", err)
		return LoginOutput{}, err
	}

	sessionInput := session.CreateInput{
		ID:                sessionID,
		UserID:            securityState.UserID,
		CurrentJTI:        refreshJTI,
		CredentialVersion: securityState.CredentialVersion,
		IP:                input.IP,
		Device:            input.Device,
		JTITTL:            us.refreshTTL,
		SessionTTL:        us.sessionTTL,
	}

	if err := us.sessionUC.CreateSession(ctx, sessionInput); err != nil {
		if stdErrors.Is(err, domainErrors.ErrUserAccessBlocked) {
			return LoginOutput{}, domainErrors.ErrAccountSuspended
		}
		return LoginOutput{}, err
	}

	access, accessClaims, err := us.jwtService.GenerateAccessToken(
		tokenIdentity,
		valueobject.AuthorizationSnapshot{
			Role:        securityState.Role,
			Permissions: securityState.Permissions,
		},
		us.accessTTL,
	)
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
			Role:      securityState.Role.String(),
			Status:    securityState.Status.String(),
			CreatedAt: userEntity.CreatedAt,
		},
	}, nil
}

func authenticationStatusError(status valueobject.UserStatus) error {
	switch status {
	case valueobject.StatusInactive:
		return authorization.ErrPrincipalInactive
	case valueobject.StatusSuspended:
		return domainErrors.ErrAccountSuspended
	case valueobject.StatusActive:
		return nil
	default:
		return fmt.Errorf("authoritative user status is invalid")
	}
}

func (us *AuthenticationUseCase) Logout(ctx context.Context, sessionID, userID string) error {

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

func (us *AuthenticationUseCase) Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error) {

	claims, err := us.jwtService.ParseAndValidate(input.RefreshToken)
	if err != nil {
		us.logger.Error("invalid refresh token", "error", err)
		return RefreshOutput{}, domainErrors.ErrUnauthorized
	}

	if claims.TokenType != valueobject.TokenTypeRefresh {
		us.logger.Error("refresh token with wrong type", "userID", claims.UserID, "tokenType", claims.TokenType)
		return RefreshOutput{}, domainErrors.ErrUnauthorized
	}

	valid, err := us.sessionUC.ValidateSession(ctx, session.ValidateInput{
		UserID:            claims.UserID,
		SessionID:         claims.SessionID,
		JTI:               claims.JTI,
		CredentialVersion: claims.CredentialVersion,
	})
	if err != nil {
		us.logger.Error("failed to validate refresh session", "userID", claims.UserID, "error", err)
		if stdErrors.Is(err, domainErrors.ErrUserAccessBlocked) {
			return RefreshOutput{}, domainErrors.ErrAccountSuspended
		}
		return RefreshOutput{}, err
	}
	if !valid {
		return RefreshOutput{}, domainErrors.ErrUnauthorized
	}

	securityState, err := us.securityStates.GetSecurityState(ctx, claims.UserID)
	if err != nil {
		return RefreshOutput{}, err
	}
	if securityState.UserID != claims.UserID ||
		securityState.CredentialVersion != claims.CredentialVersion {
		return RefreshOutput{}, domainErrors.ErrUnauthorized
	}
	if err := authenticationStatusError(securityState.Status); err != nil {
		return RefreshOutput{}, err
	}
	us.logger.Debug("refresh token requested", "userID", claims.UserID, "ip", input.IP, "device", input.Device)

	refreshJTI := pkg.ULIDGenerator()
	tokenIdentity := valueobject.TokenIdentity{
		UserID:            claims.UserID,
		SessionID:         claims.SessionID,
		JTI:               refreshJTI,
		CredentialVersion: claims.CredentialVersion,
	}
	refresh, refreshClaims, err := us.jwtService.GenerateRefreshToken(
		tokenIdentity,
		us.refreshTTL,
	)
	if err != nil {
		us.logger.Error("failed to create refresh token", "userID", claims.UserID, "error", err)
		return RefreshOutput{}, err
	}

	rotateInput := session.RotateInput{
		UserID:            claims.UserID,
		SessionID:         claims.SessionID,
		OldJTI:            claims.JTI,
		CurrentJTI:        refreshJTI,
		CredentialVersion: claims.CredentialVersion,
		Device:            input.Device,
		IP:                input.IP,
		JTITTL:            us.refreshTTL,
		SessionTTL:        us.sessionTTL,
	}

	access, accessClaims, err := us.jwtService.GenerateAccessToken(
		tokenIdentity,
		valueobject.AuthorizationSnapshot{
			Role:        securityState.Role,
			Permissions: securityState.Permissions,
		},
		us.accessTTL,
	)
	if err != nil {
		us.logger.Error("failed to create access token", "userID", claims.UserID, "error", err)
		return RefreshOutput{}, err
	}

	sessionID, err := us.sessionUC.RotateSessionJTI(ctx, rotateInput)
	if err != nil {
		if stdErrors.Is(err, domainErrors.ErrUserAccessBlocked) {
			return RefreshOutput{}, domainErrors.ErrAccountSuspended
		}
		return RefreshOutput{}, err
	}
	if sessionID != claims.SessionID {
		return RefreshOutput{}, domainErrors.ErrUnauthorized
	}

	us.logger.Info("user refresh token successful", "userID", claims.UserID, "oldJTI", claims.JTI, "newJTI", refreshJTI)

	return RefreshOutput{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessClaims.GetExpiresAt(),
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshClaims.GetExpiresAt(),
	}, nil
}
