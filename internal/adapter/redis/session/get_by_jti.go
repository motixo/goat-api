package session

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mot0x0/gopi/internal/domain/entity"
	"github.com/redis/go-redis/v9"
)

func (r *Repository) GetSessionByJTI(ctx context.Context, jti string) (*entity.Session, error) {
	jtiKey := "jti:" + jti

	sessionID, err := r.client.Get(ctx, jtiKey).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, err
	}

	sessionKey := "session:" + sessionID
	data, err := r.client.HGetAll(ctx, sessionKey).Result()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("session hash missing")
	}

	s := &entity.Session{
		ID:         data["id"],
		UserID:     data["user_id"],
		Device:     data["device"],
		IP:         data["ip"],
		CurrentJTI: data["current_jti"],
	}

	createdAtUnix, _ := strconv.ParseInt(data["created_at"], 10, 64)
	expiresAtUnix, _ := strconv.ParseInt(data["expires_at"], 10, 64)
	s.CreatedAt = time.Unix(createdAtUnix, 0)
	s.ExpiresAt = time.Unix(expiresAtUnix, 0)

	return s, nil
}
