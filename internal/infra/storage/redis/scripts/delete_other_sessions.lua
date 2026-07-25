local userKey = KEYS[1]
local expectedUserID = ARGV[1]
local mode = ARGV[2]
local sessionKeyPrefix = "session:id:"
local maxSafeGeneration = 9007199254740991

if not expectedUserID or expectedUserID == "" then
    return redis.error_reply("User ID is required")
end
if mode ~= "all" and mode ~= "others" and mode ~= "block_all" then
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
    local currentOwner = redis.pcall("HGET", currentSessionKey, "user_id")
    if (type(currentOwner) == "table" and currentOwner.err)
        or not currentOwner
        or currentOwner ~= expectedUserID then
        return -1
    end
end

local accessKey = false
local nextGeneration = false
if mode == "block_all" then
    accessKey = KEYS[2]
    if not accessKey then
        return redis.error_reply("User access-state key is required")
    end

    local accessType = redis.call("TYPE", accessKey).ok
    local blocked = "0"
    local generation = 0
    if accessType ~= "none" then
        if accessType ~= "hash" then
            return redis.error_reply("invalid_user_access_state_type")
        end
        local access = redis.call(
            "HMGET",
            accessKey,
            "blocked",
            "session_generation"
        )
        blocked = access[1]
        local generationValue = access[2]
        if (blocked ~= "0" and blocked ~= "1")
            or not generationValue
            or not string.match(generationValue, "^[1-9]%d*$") then
            return redis.error_reply("invalid_user_access_state")
        end
        generation = tonumber(generationValue)
        if not generation or generation > maxSafeGeneration then
            return redis.error_reply("invalid_user_session_generation")
        end
    end

    if blocked == "1" then
        nextGeneration = generation
    else
        if generation >= maxSafeGeneration then
            return redis.error_reply("user_session_generation_exhausted")
        end
        nextGeneration = generation + 1
    end
end

local userIndexType = redis.call("TYPE", userKey).ok
local indexedSessionKeys = {}
if userIndexType == "zset" then
    indexedSessionKeys = redis.call("ZRANGE", userKey, 0, -1)
elseif userIndexType ~= "none" and mode == "others" then
    return redis.error_reply("invalid_user_session_index_type")
end

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

if mode == "block_all" then
    redis.call(
        "HSET",
        accessKey,
        "blocked", "1",
        "session_generation", string.format("%.0f", nextGeneration)
    )
end

if mode == "all" or mode == "block_all" then
    -- The index is server-owned. Removing it also safely prunes a wrong-type
    -- value without touching any unproven foreign session reference.
    redis.call("DEL", userKey)
end

local deletedCount = 0
for _, candidate in ipairs(candidates) do
    if mode == "others" then
        redis.call("ZREM", userKey, candidate.sessionKey)
    end
    if candidate.owner == expectedUserID and candidate.hasSessionPrefix then
        if candidate.jti then
            local jtiKey = "session:jti:" .. candidate.jti
            local mappedSessionKey = redis.pcall("GET", jtiKey)
            local mappedSessionKeyFailed = type(mappedSessionKey) == "table"
                and mappedSessionKey.err
            if not mappedSessionKeyFailed
                and mappedSessionKey == candidate.sessionKey then
                redis.call("DEL", jtiKey)
            end
        end
        redis.call("DEL", candidate.sessionKey)
        deletedCount = deletedCount + 1
    end
end

return deletedCount
