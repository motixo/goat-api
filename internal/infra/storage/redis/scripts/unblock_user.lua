local accessKey = KEYS[1]
local maxSafeGeneration = 9007199254740991

local accessType = redis.call("TYPE", accessKey).ok
if accessType == "none" then
    return redis.error_reply("missing_user_access_state")
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
local generation = access[2]
if (blocked ~= "0" and blocked ~= "1")
    or not generation
    or not string.match(generation, "^[1-9]%d*$") then
    return redis.error_reply("invalid_user_access_state")
end
local generationNumber = tonumber(generation)
if not generationNumber or generationNumber > maxSafeGeneration then
    return redis.error_reply("invalid_user_session_generation")
end

redis.call(
    "HSET",
    accessKey,
    "blocked", "0",
    "session_generation", generation
)

return tonumber(generation)
