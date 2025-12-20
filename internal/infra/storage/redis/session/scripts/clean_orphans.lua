local userKey = KEYS[1]

local storedSessionKeys = redis.call("ZRANGE", userKey, 0, -1)
local removedCount = 0

for _, sessionKey in ipairs(storedSessionKeys) do
    local exists = redis.call("EXISTS", sessionKey)
    
    if exists == 0 then
        redis.call("ZREM", userKey, sessionKey)
        removedCount = removedCount + 1
    end
end

return removedCount