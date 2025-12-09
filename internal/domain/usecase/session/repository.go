package session

import (
	"context"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/pagination"
)

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

func (us *SessionUseCase) GetSessionsByUser(ctx context.Context, userID, sessionID string, p pagination.Input) (*SessionListResponse, error) {
	us.logger.Debug("retrieving user sessions", "userID", userID, "currentSessionID", sessionID)
	sessions, total, err := us.sessionRepo.ListByUser(ctx, userID, p.Page, p.PageSize)
	if err != nil {
		us.logger.Error("failed to list sessions by user", "userID", userID, "error", err)
		return nil, err
	}

	response := make([]*SessionResponse, 0, len(sessions))
	for _, se := range sessions {
		r := &SessionResponse{
			ID:        se.ID,
			Device:    se.Device,
			IP:        se.IP,
			Current:   se.ID == sessionID,
			CreatedAt: se.CreatedAt,
			UpdatedAt: se.UpdatedAt,
		}

		response = append(response, r)
	}

	// sort.Slice(response, func(i, j int) bool {
	// 	return response[i].UpdatedAt.After(response[j].UpdatedAt)
	// })

	us.logger.Info("user sessions retrieved", "userID", userID, "sessionCount", len(sessions))
	return &SessionListResponse{
		Sessions: response,
		Meta:     pagination.NewMeta(total, p),
	}, nil
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
	var target []string
	if input.RemoveOthers {
		sessions, err := us.GetSessionsByUser(ctx, input.UserID, input.CurrentSession, pagination.Input{Page: 0, PageSize: 0})
		if err != nil {
			us.logger.Error("failed to get user sessions for deletion", "userID", input.UserID, "error", err)
			return err
		}

		for _, s := range sessions.Sessions {
			if !s.Current {
				target = append(target, s.ID)
			}
		}
	} else {
		target = input.TargetSessions
	}

	if len(target) == 0 {
		us.logger.Debug("no sessions to delete", "userID", input.UserID)
		return nil
	}

	err := us.sessionRepo.Delete(ctx, target)
	if err != nil {
		us.logger.Error("failed to delete sessions", "userID", input.UserID, "targetCount", len(target), "error", err)
		return err
	}
	us.logger.Info("sessions deleted successfully", "userID", input.UserID, "removeOthers", input.RemoveOthers, "targetCount", len(input.TargetSessions))
	return nil
}
