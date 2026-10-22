local stockKey = KEYS[1]
local purchaseKey = KEYS[2]

local userID = ARGV[1]
local ttl = tonumber(ARGV[2])

if redis.call("EXISTS", purchaseKey) == 1 then
	return -1
end

local stock = tonumber(redis.call("GET", stockKey) or "0")
if stock <= 0 then
	return 0
end

redis.call("DECR", stockKey)        -- 库存 -1
redis.call("SET", purchaseKey, userID, "EX", ttl)  -- 标记用户已购买，带过期时间

return 1