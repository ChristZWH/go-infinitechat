package script

// 抢红包 Lua 脚本
const ReceiveRedPacketLua = `
local poolKey   = KEYS[1]
local recordKey = KEYS[2]
local streamKey = KEYS[3]
local userId    = ARGV[1]
local nowArg 		= ARGV[2]  -- 毫秒
local redPacketId  = ARGV[3]
local receivedTime = ARGV[4]

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

-- 判断是否领完（在 XADD 之前算好，把 completed 一起写进事件）
local left = redis.call('LLEN', poolKey)
local completed = 0
if left == 0 then
		completed = 1
end

-- 领取事件写入 Stream（发件箱）：与 LPOP/HSET 同一脚本 = 原子。
-- 这样即使进程在 LPOP 之后、落库之前崩溃，事件也已在 Redis 持久化，
-- 由 stream worker 消费入账（at-least-once），不存在「抢到但钱没到账」的窗口。
-- MAXLEN ~ 10000 近似裁剪：防止 worker 长时间不可用时 Stream 无界膨胀吃内存；
-- 入账是幂等的，但上限别设太小，否则会裁掉还没入账的旧消息。'*' 表示 ID 自动生成。
redis.call('XADD', streamKey, 'MAXLEN', '~', 10000, '*',
		'userId', userId,
		'redPacketId', redPacketId,
		'amount', amount,
		'receivedTime', receivedTime,
		'completed', completed)

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
