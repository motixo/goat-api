	local oldJTIKey = KEYS[1]
	local newJTIKey = KEYS[2]

	local newJTI = ARGV[1]
	local ip = ARGV[2]
	local device = ARGV[3]
	local updatedAt = ARGV[4]
	local expiresAt = ARGV[5]
	local jtiTTL = tonumber(ARGV[6])
	local sessionTTL = tonumber(ARGV[7])

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

	if redis.call("DEL", oldJTIKey) == 0 then
		return redis.error_reply("jti_already_used")
	end

	redis.call("SET", newJTIKey, sessionKey, "EX", jtiTTL)

	redis.call("HSET", sessionKey,
		"current_jti", newJTI,
		"updated_at", updatedAt,
		"expires_at", expiresAt,
		"ip", ip,
		"device", device
	)

	redis.call("EXPIRE", sessionKey, sessionTTL)

	return sessionKey