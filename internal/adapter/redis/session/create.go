package session

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
	"github.com/redis/go-redis/v9"
)

var createSessionLua = redis.NewScript(`
	local sessionKey = KEYS[1]
	local jtiKey = KEYS[2]
	local userKey = KEYS[3]

	local sessionTTL = tonumber(ARGV[#ARGV - 1])
	local jtiTTL = tonumber(ARGV[#ARGV])

	if not sessionTTL or sessionTTL <= 0 then
		return redis.error_reply("Session TTL must be positive integer")
	end
	if not jtiTTL or jtiTTL <= 0 then
		return redis.error_reply("JTI TTL must be positive integer")
	end

	local hsetArgs = {}
	for i = 1, #ARGV - 2 do
		hsetArgs[i] = ARGV[i]
	end

	redis.call("HSET", sessionKey, unpack(hsetArgs))
	redis.call("EXPIRE", sessionKey, sessionTTL)

	redis.call("SET", jtiKey, sessionKey, "EX", jtiTTL)
	redis.call("SADD", userKey, sessionKey)
	redis.call("EXPIRE", userKey, sessionTTL)

	return 1
`)

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

	_, err := createSessionLua.Run(ctx, r.client, []string{sessionkey, jtiKey, userkey}, argv...).Result()
	return err
}
