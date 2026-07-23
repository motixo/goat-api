local jtiKey = KEYS[1]
local expectedJTI = ARGV[1]

if not expectedJTI or expectedJTI == "" then
    return redis.error_reply("JTI is required")
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
    "credential_version"
)
if fields.err then
    return redis.error_reply("read_session_failed")
end

local id = fields[1]
local userID = fields[2]
local currentJTI = fields[3]
local credentialVersion = fields[4]
local expectedSessionKey = id and "session:id:" .. id or ""

if not id or id == ""
    or not userID or userID == ""
    or currentJTI ~= expectedJTI
    or sessionKey ~= expectedSessionKey
    or not credentialVersion
    or not string.match(credentialVersion, "^[1-9]%d*$") then
    redis.call("DEL", jtiKey)
    return nil
end

return { id, userID, currentJTI, credentialVersion }
