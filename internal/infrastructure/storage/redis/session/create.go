package session

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
)

func (r *Repository) Create(ctx context.Context, s *entity.Session) error {
	sessionkey := r.key("session", s.ID)
	jtiKey := r.key("jti", s.CurrentJTI)
	userkey := r.key("user", s.UserID)

	argv := []interface{}{
		"id", s.ID,
		"user_id", s.UserID,
		"device", s.Device,
		"ip", s.IP,
		"created_at", s.CreatedAt.Unix(),
		"updated_at", s.UpdatedAt.Unix(),
		"expires_at", s.ExpiresAt.Unix(),
		"current_jti", s.CurrentJTI,
		s.SessionTTLSeconds,
		s.JTITTLSeconds,
	}

	script := getScript("create_session")
	_, err := script.Run(ctx, r.client, []string{sessionkey, jtiKey, userkey}, argv...).Result()
	return err
}
