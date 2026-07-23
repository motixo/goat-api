local userKey = KEYS[1]
local expectedUserID = ARGV[1]
local offset = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

if not expectedUserID or expectedUserID == "" then
    return redis.error_reply("User ID is required")
end
if not offset or offset < 0 or offset % 1 ~= 0 then
    return redis.error_reply("Offset must be a non-negative integer")
end
if not limit or limit < 0 or limit % 1 ~= 0 then
    return redis.error_reply("Limit must be a non-negative integer")
end

local sessionKeyPrefix = "session:id:"
local indexedSessionKeys = redis.call("ZREVRANGE", userKey, 0, -1)
local cleanup = {}
local result = { 0 }
local validTotal = 0
local pageCount = 0

local function isUnixTimestamp(value)
    if not value or not string.match(value, "^%d+$") then
        return false
    end

    local timestamp = tonumber(value)
    return timestamp and timestamp <= 253402300799
end

-- Read and validate the entire index before the first mutation. Pagination is
-- applied to valid owned sessions, not raw index positions.
for _, sessionKey in ipairs(indexedSessionKeys) do
    local fields = redis.pcall(
        "HMGET",
        sessionKey,
        "id",
        "user_id",
        "device",
        "ip",
        "current_jti",
        "credential_version",
        "created_at",
        "updated_at",
        "expires_at"
    )

    if fields.err then
        cleanup[#cleanup + 1] = { sessionKey = sessionKey, revoke = false }
    else
        local id = fields[1]
        local owner = fields[2]
        local device = fields[3]
        local ip = fields[4]
        local currentJTI = fields[5]
        local credentialVersion = fields[6]
        local createdAt = fields[7]
        local updatedAt = fields[8]
        local expiresAt = fields[9]
        local hasSessionPrefix = string.sub(sessionKey, 1, string.len(sessionKeyPrefix)) == sessionKeyPrefix
        local complete = owner == expectedUserID
            and hasSessionPrefix
            and id
            and id ~= ""
            and sessionKey == sessionKeyPrefix .. id
            and currentJTI
            and currentJTI ~= ""
            and credentialVersion
            and string.match(credentialVersion, "^[1-9]%d*$")
            and isUnixTimestamp(createdAt)
            and isUnixTimestamp(updatedAt)
            and isUnixTimestamp(expiresAt)

        if complete then
            if validTotal >= offset and (limit == 0 or pageCount < limit) then
                result[#result + 1] = id
                result[#result + 1] = owner
                result[#result + 1] = device or ""
                result[#result + 1] = ip or ""
                result[#result + 1] = currentJTI
                result[#result + 1] = credentialVersion
                result[#result + 1] = createdAt
                result[#result + 1] = updatedAt
                result[#result + 1] = expiresAt
                pageCount = pageCount + 1
            end
            validTotal = validTotal + 1
        else
            cleanup[#cleanup + 1] = {
                sessionKey = sessionKey,
                revoke = owner == expectedUserID and hasSessionPrefix,
                jti = currentJTI and currentJTI ~= "" and currentJTI or nil,
            }
        end
    end
end

-- Foreign, stale, expired, wrong-type, and incomplete references are removed
-- from this user's index. Only incomplete records proven to belong to this user
-- are revoked; foreign hashes and JTIs are never changed.
for _, candidate in ipairs(cleanup) do
    redis.call("ZREM", userKey, candidate.sessionKey)
    if candidate.revoke then
        if candidate.jti then
            local jtiKey = "session:jti:" .. candidate.jti
            local mappedSessionKey = redis.pcall("GET", jtiKey)
            if not mappedSessionKey.err and mappedSessionKey == candidate.sessionKey then
                redis.call("DEL", jtiKey)
            end
        end
        redis.call("DEL", candidate.sessionKey)
    end
end

result[1] = validTotal
return result
