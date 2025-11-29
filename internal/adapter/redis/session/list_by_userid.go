package session

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]*entity.Session, error) {
	var sessions []*entity.Session

	iter := r.client.Scan(ctx, 0, "session:*", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()

		res, err := r.client.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}

		if res["user_id"] != userID {
			continue
		}

		createdAtUnix, _ := strconv.ParseInt(res["created_at"], 10, 64)
		expiresAtUnix, _ := strconv.ParseInt(res["expires_at"], 10, 64)

		sessions = append(sessions, &entity.Session{
			ID:         strings.TrimPrefix(key, "session:"),
			UserID:     res["user_id"],
			Device:     res["device"],
			IP:         res["ip"],
			CreatedAt:  time.Unix(createdAtUnix, 0),
			ExpiresAt:  time.Unix(expiresAtUnix, 0),
			CurrentJTI: res["current_jti"],
		})
	}

	return sessions, iter.Err()
}
