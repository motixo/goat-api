local oldJTIKey = KEYS[1]
local newJTIKey = KEYS[2]
local accessKey = KEYS[3]

local newJTI = ARGV[1]
local expectedUserID = ARGV[2]
local expectedSessionID = ARGV[3]
local expectedCredentialVersion = ARGV[4]
local ip = ARGV[5]
local device = ARGV[6]
local updatedAt = ARGV[7]
local expiresAt = ARGV[8]
local jtiTTL = tonumber(ARGV[9])
local sessionTTL = tonumber(ARGV[10])
local maxSafeGeneration = 9007199254740991

if not jtiTTL or jtiTTL <= 0 then
    return redis.error_reply("JTI TTL must be positive")
end
if not sessionTTL or sessionTTL <= 0 then
    return redis.error_reply("Session TTL must be positive")
end
if not expectedUserID or expectedUserID == "" then
    return redis.error_reply("User ID is required")
end
if not expectedSessionID or expectedSessionID == "" then
    return redis.error_reply("Session ID is required")
end
if not expectedCredentialVersion or not string.match(expectedCredentialVersion, "^[1-9]%d*$") then
    return redis.error_reply("Credential version must be a positive integer")
end

local accessType = redis.call("TYPE", accessKey).ok
if accessType == "none" then
    return { 0 }
end
if accessType ~= "hash" then
    return redis.error_reply("invalid_user_access_state_type")
end
local access = redis.call(
    "HMGET",
    accessKey,
    "blocked",
    "session_generation"
)
local blocked = access[1]
local currentGeneration = access[2]
if (blocked ~= "0" and blocked ~= "1")
    or not currentGeneration
    or not string.match(currentGeneration, "^[1-9]%d*$") then
    return redis.error_reply("invalid_user_access_state")
end
local currentGenerationNumber = tonumber(currentGeneration)
if not currentGenerationNumber or currentGenerationNumber > maxSafeGeneration then
    return redis.error_reply("invalid_user_session_generation")
end
if blocked == "1" then
    return { -1 }
end

local sessionKey = redis.call("GET", oldJTIKey)
if not sessionKey then
    return { 0 }
end

if redis.call("EXISTS", sessionKey) == 0 then
    redis.call("DEL", oldJTIKey)
    return { 0 }
end

local sessionIndexFields = redis.pcall(
    "HMGET",
    sessionKey,
    "id",
    "user_id",
    "created_at",
    "credential_version",
    "current_jti",
    "session_generation"
)
if sessionIndexFields.err then
    return redis.error_reply("read_session_failed")
end
local sessionID = sessionIndexFields[1]
local userID = sessionIndexFields[2]
local createdAt = tonumber(sessionIndexFields[3])
local credentialVersion = sessionIndexFields[4]
local currentJTI = sessionIndexFields[5]
local sessionGeneration = sessionIndexFields[6]
if not sessionID or sessionID == ""
    or sessionID ~= expectedSessionID
    or sessionKey ~= "session:id:" .. expectedSessionID
    or not userID or userID == ""
    or userID ~= expectedUserID
    or not createdAt
    or not credentialVersion
    or credentialVersion ~= expectedCredentialVersion
    or currentJTI ~= string.match(oldJTIKey, "session:jti:(.+)")
    or not sessionGeneration
    or not string.match(sessionGeneration, "^[1-9]%d*$")
    or not tonumber(sessionGeneration)
    or tonumber(sessionGeneration) > maxSafeGeneration
    or sessionGeneration ~= currentGeneration then
    return { 0 }
end

local userKey = "session:user:" .. userID
local userIndexType = redis.call("TYPE", userKey).ok
local newJTIType = redis.call("TYPE", newJTIKey).ok
if userIndexType ~= "none" and userIndexType ~= "zset" then
    return redis.error_reply("invalid_user_session_index_type")
end
if newJTIType ~= "none" and newJTIType ~= "string" then
    return redis.error_reply("invalid_jti_key_type")
end

if redis.call("DEL", oldJTIKey) == 0 then
    return { 0 }
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

-- Refresh extends the session, so it must extend (or repair) the index too.
local currentUserTTL = redis.call("TTL", userKey)
redis.call("ZADD", userKey, "NX", createdAt, sessionKey)
if currentUserTTL == -2 or (currentUserTTL >= 0 and currentUserTTL < sessionTTL) then
    redis.call("EXPIRE", userKey, sessionTTL)
end

local id = string.match(sessionKey, "session:id:(.+)")
if id then
    return { 1, id }
else
    return { 1, sessionKey }
end
