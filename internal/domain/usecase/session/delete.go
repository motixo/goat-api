package session

import (
	"context"
)

type DeleteSessionsInput struct {
	UserID         string
	CurrentSession string
	TargetSessions []string `json:"session_ids"`
	RemoveOthers   bool     `json:"others"`
}

func (u *SessionUseCase) DeleteSessions(ctx context.Context, input DeleteSessionsInput) error {
	var target []string
	if input.RemoveOthers {
		sessions, err := u.GetSessionsByUser(ctx, input.UserID, input.CurrentSession)
		if err != nil {
			return err
		}

		for _, s := range sessions {
			if !s.Current {
				target = append(target, s.ID)
			}
		}
	} else {
		target = input.TargetSessions
	}

	if len(target) == 0 {
		return nil
	}

	return u.sessionRepo.Delete(ctx, target)
}
