local oldJTIKey = KEYS[1]
local newJTIKey = KEYS[2]

local newJTI = ARGV[1]
local expectedUserID = ARGV[2]
local expectedCredentialVersion = ARGV[3]
local ip = ARGV[4]
local device = ARGV[5]
local updatedAt = ARGV[6]
local expiresAt = ARGV[7]
local jtiTTL = tonumber(ARGV[8])
local sessionTTL = tonumber(ARGV[9])

if not jtiTTL or jtiTTL <= 0 then
    return redis.error_reply("JTI TTL must be positive")
end
if not sessionTTL or sessionTTL <= 0 then
    return redis.error_reply("Session TTL must be positive")
end
if not expectedUserID or expectedUserID == "" then
    return redis.error_reply("User ID is required")
end
if not expectedCredentialVersion or not string.match(expectedCredentialVersion, "^[1-9]%d*$") then
    return redis.error_reply("Credential version must be a positive integer")
end

local sessionKey = redis.call("GET", oldJTIKey)
if not sessionKey then
    return redis.error_reply("invalid_or_expired_jti")
end

if redis.call("EXISTS", sessionKey) == 0 then
    return redis.error_reply("session_expired_or_deleted")
end

local sessionIndexFields = redis.call(
    "HMGET",
    sessionKey,
    "user_id",
    "created_at",
    "credential_version",
    "current_jti"
)
local userID = sessionIndexFields[1]
local createdAt = tonumber(sessionIndexFields[2])
local credentialVersion = sessionIndexFields[3]
local currentJTI = sessionIndexFields[4]
if not userID or userID == ""
    or userID ~= expectedUserID
    or not createdAt
    or not credentialVersion
    or credentialVersion ~= expectedCredentialVersion
    or currentJTI ~= string.match(oldJTIKey, "session:jti:(.+)") then
    return redis.error_reply("invalid_session_index_fields")
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

local userKey = "session:user:" .. userID
-- Refresh extends the session, so it must extend (or repair) the index too.
local currentUserTTL = redis.call("TTL", userKey)
redis.call("ZADD", userKey, "NX", createdAt, sessionKey)
if currentUserTTL == -2 or (currentUserTTL >= 0 and currentUserTTL < sessionTTL) then
    redis.call("EXPIRE", userKey, sessionTTL)
end

local id = string.match(sessionKey, "session:id:(.+)")
if id then
    return id
else
    return sessionKey
end
