package session

import (
	"context"
	"fmt"
	"time"

	"github.com/mot0x0/gopi/internal/domain/entity"
	"github.com/redis/go-redis/v9"
)

var createSessionLua = redis.NewScript(`
	local sessionKey = KEYS[1]
	local jtiKey = KEYS[2]

	local ttl = tonumber(ARGV[#ARGV])
	if not ttl or ttl <= 0 then
		return redis.error_reply("TTL must be positive integer")
	end

	-- HSET field/value pairs (all except last arg)
	local hsetArgs = {}
	for i = 1, #ARGV - 1 do
		hsetArgs[i] = ARGV[i]
	end

	-- write session hash
	redis.call("HSET", sessionKey, unpack(hsetArgs))
	redis.call("EXPIRE", sessionKey, ttl)

	-- read sessionID from field "id"
	local sessionID = redis.call("HGET", sessionKey, "id")
	if not sessionID then
		return redis.error_reply("Missing sessionID")
	end

	redis.call("SET", jtiKey, sessionID, "EX", ttl)

	return 1
`)

func (r *Repository) CreateSession(ctx context.Context, s *entity.Session) error {
	key := r.key(s.ID)
	jtiKey := "jti:" + s.CurrentJTI

	ttl := time.Until(s.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("expires_at is in the past")
	}

	ttlSeconds := int64(ttl.Seconds())
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}

	argv := []interface{}{
		"id", s.ID,
		"user_id", s.UserID,
		"device", s.Device,
		"ip", s.IP,
		"created_at", s.CreatedAt.Unix(),
		"expires_at", s.ExpiresAt.Unix(),
		"current_jti", s.CurrentJTI,
		ttlSeconds,
	}

	_, err := createSessionLua.Run(ctx, r.client, []string{key, jtiKey}, argv...).Result()
	return err
}
