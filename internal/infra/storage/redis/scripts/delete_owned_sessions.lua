local expectedUserID = ARGV[1]

-- Validate the complete target set before mutating any Redis key. Returning 0
-- for both missing and foreign-owned sessions avoids disclosing ownership.
for _, sessionKey in ipairs(KEYS) do
	local userID = redis.call("HGET", sessionKey, "user_id")
	if not userID or userID ~= expectedUserID then
		return 0
	end
end

for _, sessionKey in ipairs(KEYS) do
	local jti = redis.call("HGET", sessionKey, "current_jti")
	redis.call("ZREM", "session:user:" .. expectedUserID, sessionKey)
	if jti then
		redis.call("DEL", "session:jti:" .. jti)
	end
	redis.call("DEL", sessionKey)
end

return 1
