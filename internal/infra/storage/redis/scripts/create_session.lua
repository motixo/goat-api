local sessionKey = KEYS[1]
local jtiKey = KEYS[2]
local userKey = KEYS[3]
local accessKey = KEYS[4]

local sessionTTL = tonumber(ARGV[#ARGV - 1])
local jtiTTL = tonumber(ARGV[#ARGV])
local maxSafeGeneration = 9007199254740991

if not sessionTTL or sessionTTL <= 0 then
    return redis.error_reply("Session TTL must be positive integer")
end
if not jtiTTL or jtiTTL <= 0 then
    return redis.error_reply("JTI TTL must be positive integer")
end
if (#ARGV - 2) % 2 ~= 0 then
    return redis.error_reply("Session fields must be field-value pairs")
end

local accessType = redis.call("TYPE", accessKey).ok
local blocked = "0"
local generation = "1"
local initializeAccess = accessType == "none"
if not initializeAccess then
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
    generation = access[2]
    if (blocked ~= "0" and blocked ~= "1")
        or not generation
        or not string.match(generation, "^[1-9]%d*$") then
        return redis.error_reply("invalid_user_access_state")
    end
    local generationNumber = tonumber(generation)
    if not generationNumber or generationNumber > maxSafeGeneration then
        return redis.error_reply("invalid_user_session_generation")
    end
end

if blocked == "1" then
    return -1
end

local sessionType = redis.call("TYPE", sessionKey).ok
local jtiType = redis.call("TYPE", jtiKey).ok
local userIndexType = redis.call("TYPE", userKey).ok
if sessionType ~= "none" and sessionType ~= "hash" then
    return redis.error_reply("invalid_session_key_type")
end
if jtiType ~= "none" and jtiType ~= "string" then
    return redis.error_reply("invalid_jti_key_type")
end
if userIndexType ~= "none" and userIndexType ~= "zset" then
    return redis.error_reply("invalid_user_session_index_type")
end

local hsetArgs = {}
for i = 1, #ARGV - 2 do
    hsetArgs[i] = ARGV[i]
end
hsetArgs[#hsetArgs + 1] = "session_generation"
hsetArgs[#hsetArgs + 1] = generation

if initializeAccess then
    redis.call(
        "HSET",
        accessKey,
        "blocked", "0",
        "session_generation", generation
    )
end

redis.call("HSET", sessionKey, unpack(hsetArgs))
redis.call("EXPIRE", sessionKey, sessionTTL)

redis.call("SET", jtiKey, sessionKey, "EX", jtiTTL)
local now = redis.call("TIME")[1]
-- The shared index must live at least as long as its longest session.
local currentUserTTL = redis.call("TTL", userKey)
redis.call("ZADD", userKey, now, sessionKey)
if currentUserTTL == -2 or (currentUserTTL >= 0 and currentUserTTL < sessionTTL) then
    redis.call("EXPIRE", userKey, sessionTTL)
end

return tonumber(generation)
