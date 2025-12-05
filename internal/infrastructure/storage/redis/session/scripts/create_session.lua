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