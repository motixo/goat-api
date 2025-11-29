package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var rotateJtiLua = redis.NewScript(`
	local oldJTIKey = KEYS[1]
	local newJTIKey = KEYS[2]

	local newJTI = ARGV[1]
	local ip = ARGV[2]
	local device = ARGV[3]
	local updatedAt = ARGV[4]
	local jtiTTL = tonumber(ARGV[5])
	local sessionTTL = tonumber(ARGV[6])

	if jtiTTL <= 0 then
		return redis.error_reply("JTI TTL must be positive")
	end
	if sessionTTL <= 0 then
		return redis.error_reply("Session TTL must be positive")
	end

	local sessionKey = redis.call("GET", oldJTIKey)
	if not sessionKey then
		return redis.error_reply("invalid_or_expired_jti")
	end

	redis.call("DEL", oldJTIKey)

	redis.call("SET", newJTIKey, sessionKey, "EX", jtiTTL)

	redis.call("HSET", sessionKey,
		"current_jti", newJTI,
		"updated_at", updatedAt,
		"ip", ip,
		"device", device
	)

	redis.call("EXPIRE", sessionKey, sessionTTL)

	return sessionKey
`)

func (r *Repository) RotateJTI(
	ctx context.Context,
	oldJTI, newJTI, ip, device string,
	jtiTTLSeconds, sessionTTLSeconds int,
) (string, error) {

	oldJTIKey := r.key("jti", oldJTI)
	newJTIKey := r.key("jti", newJTI)
	updatedAt := fmt.Sprintf("%d", time.Now().UTC().Unix())

	argv := []interface{}{
		newJTI,
		ip,
		device,
		updatedAt,
		jtiTTLSeconds,
		sessionTTLSeconds,
	}

	res, err := rotateJtiLua.Run(ctx, r.client, []string{oldJTIKey, newJTIKey}, argv...).Result()
	if err != nil {
		return "", fmt.Errorf("failed to rotate JTI: %w", err)
	}

	sessionID, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type returned from Redis: %T", res)
	}

	parts := strings.Split(sessionID, ":")
	if len(parts) == 2 {
		sessionID = parts[1]
	}
	return sessionID, nil
}
