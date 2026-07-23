package session

import (
	"context"
	stdErrors "errors"
	"fmt"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/oklog/ulid/v2"
)

type SessionUseCase struct {
	sessionRepo repository.SessionRepository
	userRepo    CredentialVersionReader
	logger      pkg.Logger
}

func NewUsecase(
	sessionRepo repository.SessionRepository,
	userRepo CredentialVersionReader,
	logger pkg.Logger,
) UseCase {
	return &SessionUseCase{
		sessionRepo: sessionRepo,
		userRepo:    userRepo,
		logger:      logger,
	}
}

func (us *SessionUseCase) CreateSession(ctx context.Context, input CreateInput) error {
	us.logger.Debug("creating session", "userID", input.UserID, "device", input.Device, "ip", input.IP, "currentJTI", input.CurrentJTI)
	if input.CredentialVersion <= 0 {
		return fmt.Errorf("credential version must be positive")
	}

	now := time.Now().UTC()
	expiresAt := now.Add(input.SessionTTL)

	session := &entity.Session{
		ID:                input.ID,
		UserID:            input.UserID,
		CredentialVersion: input.CredentialVersion,
		CurrentJTI:        input.CurrentJTI,
		IP:                input.IP,
		Device:            input.Device,
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         expiresAt,
		JTITTLSeconds:     int64(input.JTITTL.Seconds()),
		SessionTTLSeconds: int64(input.SessionTTL.Seconds()),
	}
	if err := us.sessionRepo.Create(ctx, session); err != nil {
		us.logger.Error("failed to create session", "userID", input.UserID, "currentJTI", input.CurrentJTI, "error", err)
		return err
	}
	us.logger.Info("session created successfully", "userID", input.UserID, "sessionID", session.ID, "currentJTI", input.CurrentJTI)
	return nil

}

func (us *SessionUseCase) GetSessionsByUser(ctx context.Context, userID, sessionID string, offset, limit int) ([]SessionOutput, int64, error) {
	us.logger.Debug("retrieving user sessions", "userID", userID, "currentSessionID", sessionID)
	sessions, total, err := us.sessionRepo.ListByUser(ctx, userID, offset, limit)
	if err != nil {
		us.logger.Error("failed to list sessions by user", "userID", userID, "error", err)
		return nil, 0, err
	}

	output := make([]SessionOutput, 0, len(sessions))
	for _, se := range sessions {
		sessionOutput := SessionOutput{
			ID:        se.ID,
			Device:    se.Device,
			IP:        se.IP,
			Current:   se.ID == sessionID,
			CreatedAt: se.CreatedAt,
			UpdatedAt: se.UpdatedAt,
		}

		output = append(output, sessionOutput)
	}

	us.logger.Info("user sessions retrieved", "userID", userID, "sessionCount", total)
	return output, total, nil
}

func (us *SessionUseCase) RotateSessionJTI(ctx context.Context, input RotateInput) (string, error) {
	us.logger.Debug("rotating session JTI", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "ip", input.IP, "device", input.Device)
	current, valid, err := us.validateSession(ctx, ValidateInput{
		UserID: input.UserID,
		JTI:    input.OldJTI,
	})
	if err != nil {
		us.logger.Error("failed to validate session before JTI rotation", "oldJTI", input.OldJTI, "error", err)
		return "", err
	}
	if !valid {
		us.logger.Warn("attempt to rotate invalid session JTI", "oldJTI", input.OldJTI, "ip", input.IP, "device", input.Device)
		return "", domainErrors.ErrUnauthorized
	}

	now := time.Now().UTC()
	expiresAt := now.Add(input.SessionTTL)

	sessionID, err := us.sessionRepo.RotateJTI(
		ctx,
		input.OldJTI,
		input.CurrentJTI,
		input.UserID,
		current.CredentialVersion,
		input.IP,
		input.Device,
		expiresAt,
		int64(input.JTITTL.Seconds()),
		int64(input.SessionTTL.Seconds()),
	)
	if err != nil {
		us.logger.Error("failed to rotate JTI", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "ip", input.IP, "device", input.Device, "error", err)
		return "", err
	}
	us.logger.Info("session JTI rotated successfully", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "sessionID", sessionID)
	return sessionID, nil
}

func (us *SessionUseCase) ValidateSession(ctx context.Context, input ValidateInput) (bool, error) {
	if input.UserID == "" || input.SessionID == "" || input.JTI == "" {
		return false, nil
	}
	_, valid, err := us.validateSession(ctx, input)
	if err != nil {
		us.logger.Error("failed to validate session", "userID", input.UserID, "sessionID", input.SessionID, "error", err)
		return false, err
	}
	us.logger.Debug("session validation result", "userID", input.UserID, "sessionID", input.SessionID, "valid", valid)
	return valid, nil
}

func (us *SessionUseCase) validateSession(
	ctx context.Context,
	input ValidateInput,
) (*entity.Session, bool, error) {
	if input.UserID == "" || input.JTI == "" {
		return nil, false, nil
	}

	current, err := us.sessionRepo.FindByJTI(ctx, input.JTI)
	if err != nil {
		return nil, false, err
	}
	if current == nil ||
		current.ID == "" ||
		current.UserID != input.UserID ||
		current.CurrentJTI != input.JTI ||
		current.CredentialVersion <= 0 ||
		(input.SessionID != "" && current.ID != input.SessionID) {
		return nil, false, nil
	}

	authoritativeVersion, err := us.userRepo.GetCredentialVersion(ctx, input.UserID)
	if err != nil {
		if stdErrors.Is(err, domainErrors.ErrUserNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if authoritativeVersion <= 0 {
		return nil, false, fmt.Errorf("authoritative credential version must be positive")
	}
	if current.CredentialVersion != authoritativeVersion {
		return nil, false, nil
	}
	return current, true, nil
}

func (us *SessionUseCase) DeleteSessions(ctx context.Context, input DeleteSessionsInput) error {
	us.logger.Info("delete sessions requested", "userID", input.UserID, "removeOthers", input.RemoveOthers, "targetCount", len(input.TargetSessions))
	if input.UserID == "" {
		return domainErrors.ErrUnauthorized
	}

	if input.RemoveOthers {
		if !isValidSessionID(input.CurrentSession) {
			return ErrInvalidSessionSelection
		}

		currentOwned, err := us.sessionRepo.DeleteOthersByUser(ctx, input.UserID, input.CurrentSession)
		if err != nil {
			us.logger.Error("failed to delete other user sessions", "userID", input.UserID, "error", err)
			return err
		}
		if !currentOwned {
			us.logger.Warn("current session was missing or not owned by user", "userID", input.UserID)
			return domainErrors.ErrNotFound
		}
		us.logger.Info("other sessions deleted successfully", "userID", input.UserID)
		return nil
	}

	if len(input.TargetSessions) == 0 {
		return ErrInvalidSessionSelection
	}

	for _, sessionID := range input.TargetSessions {
		if !isValidSessionID(sessionID) {
			return ErrInvalidSessionSelection
		}
	}

	deleted, err := us.sessionRepo.DeleteByUser(ctx, input.UserID, input.TargetSessions)
	if err != nil {
		us.logger.Error("failed to delete sessions", "userID", input.UserID, "targetCount", len(input.TargetSessions), "error", err)
		return err
	}
	if !deleted {
		us.logger.Warn("session deletion target was missing or not owned by user", "userID", input.UserID, "targetCount", len(input.TargetSessions))
		return domainErrors.ErrNotFound
	}
	us.logger.Info("sessions deleted successfully", "userID", input.UserID, "removeOthers", input.RemoveOthers, "targetCount", len(input.TargetSessions))
	return nil
}

func isValidSessionID(sessionID string) bool {
	_, err := ulid.ParseStrict(sessionID)
	return err == nil
}
