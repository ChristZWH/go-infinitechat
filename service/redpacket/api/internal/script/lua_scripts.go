package script

// 抢红包 Lua 脚本
const ReceiveRedPacketLua = `
local poolKey   = KEYS[1]
local recordKey = KEYS[2]
local userId    = ARGV[1]
local nowArg 		= ARGV[2]  -- 毫秒

-- 防止重复抢红包
if redis.call('HEXISTS', recordKey, userId) == 1 then
		return {-1}
end

-- 业务错误 调用方漏传参数
if not nowArg or nowArg == '' or tonumber(nowArg) <= 0 then
		return redis.error_reply("红包过期时间参数缺失")
end

-- 判断是否过期
local t = redis.call('TIME')
if tonumber(nowArg) <= (tonumber(t[1]) * 1000) + math.floor(tonumber(t[2]) / 1000) then
		return {-2}
end

-- 消费金额
local amount = redis.call('LPOP', poolKey)
if not amount then
		return {-3}
end

redis.call('HSET', recordKey, userId, amount)

local left = redis.call('LLEN', poolKey)
local completed = 0
if left == 0 then
		completed = 1
end
return {tonumber(amount), completed}
`

// 扫描过期红包 Lua 脚本
const ScanExpiredRedPacketsLua = `
local zsetKey  = KEYS[1]
local nowArg   = ARGV[1]
local maxCount = tonumber(ARGV[2])

if not maxCount or maxCount <= 0 then
		maxCount = 500
end

local nowMs
if not nowArg or nowArg == '' or tonumber(nowArg) <= 0 then
		local t = redis.call('TIME')
		nowMs = (tonumber(t[1]) * 1000) + math.floor(tonumber(t[2]) / 1000)
else 
		nowMs = tonumber(nowArg)
end

local expired = redis.call('ZRANGEBYSCORE', zsetKey, '-inf', nowMs, 'LIMIT', 0, maxCount)
if #expired == 0 then
		return {}
end

for i = 1, #expired do
		redis.call('ZREM', zsetKey, expired[i])
end

return expired  -- Go 端删除过期的 poolKey
`

// 计算红包剩余金额 Lua 脚本
const CalculateRemainAmountLua = `
local lkey = KEYS[1]
local items = redis.call('LRANGE', lkey, 0, -1)
local total = 0
for i = 1, #items do
		total = total + tonumber(items[i])
end
return total
`
