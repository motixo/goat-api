	for i, sessionKey in ipairs(KEYS) do
		local userId = redis.call("HGET", sessionKey, "user_id")
		local jti = redis.call("HGET", sessionKey, "current_jti")

		if userId then
			redis.call("ZREM", "session:user:" .. userId, sessionKey)
		end
		if jti then
			redis.call("DEL", "session:jti:" .. jti)
		end
		redis.call("DEL", sessionKey)
	end
	return 1