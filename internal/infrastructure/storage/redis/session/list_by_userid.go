package session

import (
	"context"
	"strconv"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
)

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]*entity.Session, error) {
	userKey := r.key("user", userID)

	sessionKeys, err := r.client.SMembers(ctx, userKey).Result()
	if err != nil {
		return nil, err
	}

	sessions := make([]*entity.Session, 0, len(sessionKeys))

	for _, sessionKey := range sessionKeys {
		fields, err := r.client.HGetAll(ctx, sessionKey).Result()
		if err != nil {
			return nil, err
		}

		if len(fields) == 0 {
			continue
		}

		s := &entity.Session{
			ID:         fields["id"],
			UserID:     fields["user_id"],
			Device:     fields["device"],
			IP:         fields["ip"],
			CurrentJTI: fields["current_jti"],
		}

		if createdAt, err := strconv.ParseInt(fields["created_at"], 10, 64); err == nil {
			s.CreatedAt = time.Unix(createdAt, 0).UTC()
		}
		if updatedAt, err := strconv.ParseInt(fields["updated_at"], 10, 64); err == nil {
			s.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		}
		if expiresAt, err := strconv.ParseInt(fields["expires_at"], 10, 64); err == nil {
			s.ExpiresAt = time.Unix(expiresAt, 0).UTC()
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}
