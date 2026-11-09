package script

// ReceiveRedPacketLua 抢红包（原子）。
//
// 参数：
//
//	KEYS[1] poolKey    红包金额池 List，redpacket:{redPacketId}:pool，元素=每份金额(分)
//	KEYS[2] recordKey  领取记录 Hash，redpacket:{redPacketId}:records，field=userId value=金额，兼防重复领取
//	KEYS[3] streamKey  领取事件发件箱 Stream，redpacket:receive:stream（全局共享，非每包一个）
//	KEYS[4] remainKey  剩余金额 String，redpacket:{redPacketId}:remain，LPOP 后 DECRBY，O(1) 读剩余
//	KEYS[5] pendingKey 已 LPOP 未入账计数 String，redpacket:{redPacketId}:pending，LPOP 后 INCR
//	ARGV[1] userId        领取用户 ID
//	ARGV[2] nowArg        红包过期时间戳(毫秒)，与 Redis TIME 比较判断是否过期
//	ARGV[3] redPacketId   红包ID（red_packet 主键，key 里的 {redPacketId} 就是它）
//	ARGV[4] receivedTime  领取时间(毫秒)，写入 Stream 供入账使用
//
// 返回：{金额(分), completed}；-1 已领过，-2 已过期，-3 金额池空。
const ReceiveRedPacketLua = `
local poolKey    = KEYS[1]
local recordKey  = KEYS[2]
local streamKey  = KEYS[3]
local remainKey  = KEYS[4]
local pendingKey = KEYS[5]
local userId     = ARGV[1]
local nowArg     = ARGV[2]  -- 毫秒
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

-- 与 LPOP 同脚本原子维护两个计数：
-- 1) remainKey DECRBY：剩余金额 O(1) 读（替代 LRANGE 0 -1 全量求和）
--    负数说明 remainKey 是旧数据缺失（上线前创建的红包无 remainKey），DEL 掉走 DB 差额兜底
-- 2) pendingKey INCR：已 LPOP 未入账计数，DB 差额兜底前检查为 0，防止双花
local remainAfter = redis.call('DECRBY', remainKey, amount)
if remainAfter < 0 then
		redis.call('DEL', remainKey)
end
redis.call('INCR', pendingKey)

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

// ScanExpiredRedPacketsLua 扫描过期红包（只读不摘）。
//
// 参数：
//
//	KEYS[1] zsetKey  过期 ZSet，redpacket-expire-zset，member=红包ID score=过期毫秒时间戳
//	ARGV[1] nowArg   当前时间(毫秒)，<=0 时用 Redis TIME
//	ARGV[2] maxCount 单批最多返回条数，<=0 时默认 500
//
// 返回：到期红包 ID 列表。只 ZRANGEBYSCORE 不 ZREM——摘除由 Go 侧在「发件箱幂等写入成功」后执行，
// 避免「先摘 ZSet 再投递」在投递失败时丢 ID。
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

return redis.call('ZRANGEBYSCORE', zsetKey, '-inf', nowMs, 'LIMIT', 0, maxCount)
`

// InitRedisPoolLua 初始化红包金额池（幂等）。
//
// 参数：
//
//	KEYS[1] poolKey    红包金额池 List，redpacket:{redPacketId}:pool
//	KEYS[2] expireZSet 过期 ZSet，redpacket-expire-zset
//	KEYS[3] remainKey  剩余金额 String，redpacket:{redPacketId}:remain
//	ARGV[1] redPacketId      红包ID（red_packet 主键，key 里的 {redPacketId} 就是它）
//	ARGV[2] expireTimestamp  过期时间戳(毫秒，发送时刻定死，重试不重算，避免漂移)
//	ARGV[3] ttlSeconds       pool / remain 的 TTL(秒)
//	ARGV[4..] amounts        每份红包金额(分)，依次 RPUSH 进 pool，并求和写 remainKey
//
// 幂等语义：pool 不存在才写金额（重试不会 RPUSH 翻倍）；ZADD 加 NX 只在 member 不存在时登记，
// 防止 pool-init 事件重复处理时漂移 score（不会给已领完/已过期红包"续命"）。
const InitRedisPoolLua = `
local poolKey    = KEYS[1]
local expireZSet = KEYS[2]
local remainKey  = KEYS[3]
local redPacketId     = ARGV[1]
local expireTimestamp = tonumber(ARGV[2])
local ttlSeconds      = tonumber(ARGV[3])

-- 幂等：只有 pool 不存在时才写金额（Lua 原子，不会出现半截写入）
if redis.call('EXISTS', poolKey) == 0 then
		local total = 0
		for i = 4, #ARGV do
				redis.call('RPUSH', poolKey, ARGV[i])
				total = total + tonumber(ARGV[i])
		end
		redis.call('SET', remainKey, total)
		if ttlSeconds > 0 then
				redis.call('EXPIRE', poolKey, ttlSeconds)
				redis.call('EXPIRE', remainKey, ttlSeconds)
		end
end

-- 登记到过期 ZSet（NX：member 已存在则不覆盖 score）
redis.call('ZADD', expireZSet, 'NX', expireTimestamp, redPacketId)
return 1
`

// DecrPendingOnceLua 恰好一次 DECR pending 计数。
//
// 参数：
//
//	KEYS[1] settledKey  每事件一次性标志 String，redpacket:settled:{streamMsgID}
//	KEYS[2] pendingKey  已 LPOP 未入账计数 String，redpacket:{redPacketId}:pending
//	ARGV[1] ttlSeconds  settledKey 的 TTL(秒)
//
// SETNX 成功（=1，第一次处理该事件）才 DECR；worker 重投时 SETNX 已存在（=0）跳过。
// SETNX + DECR + EXPIRE 放同一脚本原子执行，避免「SETNX 成功但 DECR 失败」留下的中间态，
// 从而消除「事务提交后 DECR 失败 → pending 永远 >0 → 退款永久冻结」的窗口。
const DecrPendingOnceLua = `
local settledKey  = KEYS[1]
local pendingKey  = KEYS[2]
local ttlSeconds  = tonumber(ARGV[1])

if redis.call('SETNX', settledKey, '1') == 1 then
		redis.call('DECR', pendingKey)
		if ttlSeconds > 0 then
				redis.call('EXPIRE', settledKey, ttlSeconds)
		end
end
return 1
`
