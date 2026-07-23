local userKey = KEYS[1]
local expectedUserID = ARGV[1]
local mode = ARGV[2]
local sessionKeyPrefix = "session:id:"

if not expectedUserID or expectedUserID == "" then
    return redis.error_reply("User ID is required")
end
if mode ~= "all" and mode ~= "others" then
    return redis.error_reply("Unsupported user session deletion mode")
end

local currentSessionKey = false
if mode == "others" then
    currentSessionKey = KEYS[2]
    if not currentSessionKey then
        return redis.error_reply("Current session key is required")
    end

    -- Validate the authenticated session before inspecting or mutating the
    -- index. Missing and foreign-owned current sessions share the same result.
    local currentOwner = redis.call("HGET", currentSessionKey, "user_id")
    if not currentOwner or currentOwner ~= expectedUserID then
        return -1
    end
end

local indexedSessionKeys = redis.call("ZRANGE", userKey, 0, -1)
local candidates = {}

-- Read every candidate before the first mutation. Redis scripts are atomic but
-- do not roll back writes after a runtime error, so this keeps validation
-- failures mutation-free.
for _, sessionKey in ipairs(indexedSessionKeys) do
    if sessionKey ~= currentSessionKey then
        local fields = redis.pcall("HMGET", sessionKey, "user_id", "current_jti")
        local owner = false
        local jti = false
        if not fields.err then
            owner = fields[1]
            if owner == expectedUserID then
                jti = fields[2]
            end
        end
        candidates[#candidates + 1] = {
            sessionKey = sessionKey,
            owner = owner,
            jti = jti,
            hasSessionPrefix = string.sub(
                sessionKey,
                1,
                string.len(sessionKeyPrefix)
            ) == sessionKeyPrefix,
        }
    end
end

local deletedCount = 0
for _, candidate in ipairs(candidates) do
    redis.call("ZREM", userKey, candidate.sessionKey)
    if candidate.owner == expectedUserID and candidate.hasSessionPrefix then
        if candidate.jti then
            local jtiKey = "session:jti:" .. candidate.jti
            local mappedSessionKey = redis.pcall("GET", jtiKey)
            if not mappedSessionKey.err and mappedSessionKey == candidate.sessionKey then
                redis.call("DEL", jtiKey)
            end
        end
        redis.call("DEL", candidate.sessionKey)
        deletedCount = deletedCount + 1
    end
end

return deletedCount
