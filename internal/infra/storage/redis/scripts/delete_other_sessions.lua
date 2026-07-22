local userKey = KEYS[1]
local currentSessionKey = KEYS[2]
local expectedUserID = ARGV[1]

-- Validate the authenticated session before inspecting or mutating the index.
-- Missing and foreign-owned current sessions share the same result.
local currentOwner = redis.call("HGET", currentSessionKey, "user_id")
if not currentOwner or currentOwner ~= expectedUserID then
    return -1
end

local indexedSessionKeys = redis.call("ZRANGE", userKey, 0, -1)
local candidates = {}

-- Read every candidate before the first mutation. Redis scripts are atomic but
-- do not roll back writes after a runtime error, so this keeps validation
-- failures mutation-free.
for _, sessionKey in ipairs(indexedSessionKeys) do
    if sessionKey ~= currentSessionKey then
        local owner = redis.call("HGET", sessionKey, "user_id")
        local jti = false
        if owner == expectedUserID then
            jti = redis.call("HGET", sessionKey, "current_jti")
        end
        candidates[#candidates + 1] = {
            sessionKey = sessionKey,
            owner = owner,
            jti = jti,
        }
    end
end

local deletedCount = 0
for _, candidate in ipairs(candidates) do
    redis.call("ZREM", userKey, candidate.sessionKey)
    if candidate.owner == expectedUserID then
        if candidate.jti then
            redis.call("DEL", "session:jti:" .. candidate.jti)
        end
        redis.call("DEL", candidate.sessionKey)
        deletedCount = deletedCount + 1
    end
end

return deletedCount
