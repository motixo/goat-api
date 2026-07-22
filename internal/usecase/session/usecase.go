package session

import (
	"context"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/oklog/ulid/v2"
)

type SessionUseCase struct {
	sessionRepo repository.SessionRepository
	logger      pkg.Logger
}

func NewUsecase(
	r repository.SessionRepository,
	logger pkg.Logger,
) UseCase {
	return &SessionUseCase{
		sessionRepo: r,
		logger:      logger,
	}
}

func (us *SessionUseCase) CreateSession(ctx context.Context, input CreateInput) error {
	us.logger.Debug("creating session", "userID", input.UserID, "device", input.Device, "ip", input.IP, "currentJTI", input.CurrentJTI)

	now := time.Now().UTC()
	expiresAt := now.Add(input.SessionTTL)

	session := &entity.Session{
		ID:                input.ID,
		UserID:            input.UserID,
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

func (us *SessionUseCase) GetSessionsByUser(ctx context.Context, userID, sessionID string, offset, limit int) ([]SessionResponse, int64, error) {
	us.logger.Debug("retrieving user sessions", "userID", userID, "currentSessionID", sessionID)
	sessions, total, err := us.sessionRepo.ListByUser(ctx, userID, offset, limit)
	if err != nil {
		us.logger.Error("failed to list sessions by user", "userID", userID, "error", err)
		return nil, 0, err
	}

	response := make([]SessionResponse, 0, len(sessions))
	for _, se := range sessions {
		r := SessionResponse{
			ID:        se.ID,
			Device:    se.Device,
			IP:        se.IP,
			Current:   se.ID == sessionID,
			CreatedAt: se.CreatedAt,
			UpdatedAt: se.UpdatedAt,
		}

		response = append(response, r)
	}

	us.logger.Info("user sessions retrieved", "userID", userID, "sessionCount", total)
	return response, total, nil
}

func (us *SessionUseCase) RotateSessionJTI(ctx context.Context, input RotateInput) (string, error) {
	us.logger.Debug("rotating session JTI", "oldJTI", input.OldJTI, "newJTI", input.CurrentJTI, "ip", input.IP, "device", input.Device)
	valid, err := us.sessionRepo.ExistsJTI(ctx, input.OldJTI)
	if err != nil {
		us.logger.Error("failed to check if JTI exists", "oldJTI", input.OldJTI, "error", err)
		return "", err
	}
	if !valid {
		us.logger.Warn("attempt to rotate non-existent or expired JTI", "oldJTI", input.OldJTI, "ip", input.IP, "device", input.Device)
		return "", errors.ErrUnauthorized
	}

	now := time.Now().UTC()
	expiresAt := now.Add(input.SessionTTL)

	sessionID, err := us.sessionRepo.RotateJTI(
		ctx,
		input.OldJTI,
		input.CurrentJTI,
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

func (us *SessionUseCase) IsJTIValid(ctx context.Context, jti string) (bool, error) {
	valid, err := us.sessionRepo.ExistsJTI(ctx, jti)
	if err != nil {
		us.logger.Error("failed to check JTI validity", "jti", jti, "error", err)
		return false, err
	}
	us.logger.Debug("JTI validation result", "jti", jti, "valid", valid)
	return valid, nil
}

func (us *SessionUseCase) DeleteSessions(ctx context.Context, input DeleteSessionsInput) error {
	us.logger.Info("delete sessions requested", "userID", input.UserID, "removeOthers", input.RemoveOthers, "targetCount", len(input.TargetSessions))
	if input.UserID == "" {
		return errors.ErrUnauthorized
	}

	if input.RemoveOthers {
		if !isValidSessionID(input.CurrentSession) {
			return errors.ErrInvalidInput
		}

		currentOwned, err := us.sessionRepo.DeleteOthersByUser(ctx, input.UserID, input.CurrentSession)
		if err != nil {
			us.logger.Error("failed to delete other user sessions", "userID", input.UserID, "error", err)
			return err
		}
		if !currentOwned {
			us.logger.Warn("current session was missing or not owned by user", "userID", input.UserID)
			return errors.ErrNotFound
		}
		us.logger.Info("other sessions deleted successfully", "userID", input.UserID)
		return nil
	}

	if len(input.TargetSessions) == 0 {
		return errors.ErrInvalidInput
	}

	for _, sessionID := range input.TargetSessions {
		if !isValidSessionID(sessionID) {
			return errors.ErrInvalidInput
		}
	}

	deleted, err := us.sessionRepo.DeleteByUser(ctx, input.UserID, input.TargetSessions)
	if err != nil {
		us.logger.Error("failed to delete sessions", "userID", input.UserID, "targetCount", len(input.TargetSessions), "error", err)
		return err
	}
	if !deleted {
		us.logger.Warn("session deletion target was missing or not owned by user", "userID", input.UserID, "targetCount", len(input.TargetSessions))
		return errors.ErrNotFound
	}
	us.logger.Info("sessions deleted successfully", "userID", input.UserID, "removeOthers", input.RemoveOthers, "targetCount", len(input.TargetSessions))
	return nil
}

func isValidSessionID(sessionID string) bool {
	_, err := ulid.ParseStrict(sessionID)
	return err == nil
}
