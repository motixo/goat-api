package session

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

func (r *Repository) Get(ctx context.Context, sessionID string) (*entity.Session, error) {
	key := r.key("session", sessionID)

	res, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("session not found")
	}

	createdAtUnix, _ := strconv.ParseInt(res["created_at"], 10, 64)
	expiresAtUnix, _ := strconv.ParseInt(res["expires_at"], 10, 64)

	s := &entity.Session{
		ID:         sessionID,
		UserID:     res["user_id"],
		Device:     res["device"],
		IP:         res["ip"],
		CreatedAt:  time.Unix(createdAtUnix, 0),
		ExpiresAt:  time.Unix(expiresAtUnix, 0),
		CurrentJTI: res["current_jti"],
	}

	return s, nil
}
