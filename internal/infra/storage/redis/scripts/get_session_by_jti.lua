local jtiKey = KEYS[1]
local accessKey = KEYS[2]
local expectedJTI = ARGV[1]
local expectedUserID = ARGV[2]
local expectedSessionID = ARGV[3]
local expectedCredentialVersion = ARGV[4]
local maxSafeGeneration = 9007199254740991

if not expectedJTI or expectedJTI == "" then
    return redis.error_reply("JTI is required")
end
if not expectedUserID or expectedUserID == ""
    or not expectedSessionID or expectedSessionID == ""
    or not expectedCredentialVersion
    or not string.match(expectedCredentialVersion, "^[1-9]%d*$") then
    return redis.error_reply("Expected session identity is incomplete")
end

local accessType = redis.call("TYPE", accessKey).ok
if accessType == "none" then
    return nil
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

local sessionKey = redis.call("GET", jtiKey)
if not sessionKey then
    return nil
end

if redis.call("EXISTS", sessionKey) == 0 then
    redis.call("DEL", jtiKey)
    return nil
end

local fields = redis.pcall(
    "HMGET",
    sessionKey,
    "id",
    "user_id",
    "current_jti",
    "credential_version",
    "session_generation"
)
if fields.err then
    return redis.error_reply("read_session_failed")
end

local id = fields[1]
local userID = fields[2]
local currentJTI = fields[3]
local credentialVersion = fields[4]
local sessionGeneration = fields[5]
local expectedSessionKey = id and "session:id:" .. id or ""

if not id or id == ""
    or not userID or userID == ""
    or currentJTI ~= expectedJTI
    or sessionKey ~= expectedSessionKey
    or not credentialVersion
    or not string.match(credentialVersion, "^[1-9]%d*$")
    or not sessionGeneration
    or not string.match(sessionGeneration, "^[1-9]%d*$")
    or not tonumber(sessionGeneration)
    or tonumber(sessionGeneration) > maxSafeGeneration then
    redis.call("DEL", jtiKey)
    return nil
end

if id ~= expectedSessionID
    or userID ~= expectedUserID
    or credentialVersion ~= expectedCredentialVersion
    or sessionGeneration ~= currentGeneration then
    return nil
end

return {
    id,
    userID,
    currentJTI,
    credentialVersion,
    sessionGeneration
}
