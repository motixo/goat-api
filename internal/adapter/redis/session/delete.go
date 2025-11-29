package session

import (
	"context"

	"github.com/redis/go-redis/v9"
)

var deleteSessionsLua = redis.NewScript(`
	for i, sessionKey in ipairs(KEYS) do
		local userId = redis.call("HGET", sessionKey, "user_id")
		local jti = redis.call("HGET", sessionKey, "current_jti")

		if userId then
			redis.call("SREM", "user:" .. userId, sessionKey)
		end
		if jti then
			redis.call("DEL", "jti:" .. jti)
		end
		redis.call("DEL", sessionKey)
	end
	return 1
`)

func (r *Repository) Delete(ctx context.Context, sessionIDs []string) error {
	if len(sessionIDs) == 0 {
		return nil
	}

	sessionKeys := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		sessionKeys = append(sessionKeys, r.key("session", sessionID))
	}

	_, err := deleteSessionsLua.Run(ctx, r.client, sessionKeys).Result()
	return err
}
