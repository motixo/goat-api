package session

import (
	"context"

	"github.com/redis/go-redis/v9"
)

var deleteSessionLua = redis.NewScript(`
    local sessionKey = KEYS[1]
    local jtiKey  = KEYS[2]
    redis.call("DEL", jtiKey)
    redis.call("DEL", sessionKey)
    return 1
`)

func (r *Repository) Delete(ctx context.Context, sessionID, JTI string) error {
	sessionKey := r.key("session", sessionID)
	jtiPrefix := r.key("jti", JTI)

	_, err := deleteSessionLua.Run(ctx, r.client, []string{sessionKey, jtiPrefix}).Result()
	return err
}
