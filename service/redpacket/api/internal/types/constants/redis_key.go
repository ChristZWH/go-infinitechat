package constants

import "fmt"

const (
	PoolTemplate           = "redpacket:%d:pool"
	RecordsTemplate        = "redpacket:%d:records"
	RemainTemplate         = "redpacket:%d:remain"  // 剩余金额（分）：发送时 SET 总额，每次 LPOP 后 DECRBY，O(1) 读
	PendingTemplate        = "redpacket:%d:pending" // 已 LPOP 未入账计数：抢包 Lua INCR、SettleReceive 成功 DECR
	SettledTemplate        = "redpacket:settled:%s" // 每事件一次性标志：SETNX 成功才 DECR pending，保证恰好一次
	ExpireZSet             = "redpacket-expire-zset"
	PreventDuplicatePrefix = "prevent:duplicate:"

	// 领取事件发件箱：Redis Stream + 消费组
	// Lua 抢包时把领取事件 XADD 到这里，stream worker 用 XREADGROUP 消费入账
	ReceiveStreamKey   = "redpacket:receive:stream"
	ReceiveStreamGroup = "redpacket-receive-group"
)

// List  [350, 120, 880, ...]   ← 每个元素一个随机金额（单位：分）
func GetPoolKey(redPacketId int64) string {
	return fmt.Sprintf(PoolTemplate, redPacketId)
}

// redpacket:records:123 Hash  {888: 350, 999: 120}   ← field=userId, value=抢到的金额
func GetRecordsKey(redPacketId int64) string {
	return fmt.Sprintf(RecordsTemplate, redPacketId)
}

// 剩余金额（分），String：LPOP 后 DECRBY，过期退款 O(1) 读
func GetRemainKey(redPacketId int64) string {
	return fmt.Sprintf(RemainTemplate, redPacketId)
}

// 已 LPOP 未入账计数，String：抢包 Lua INCR、SettleReceive 成功 DECR
func GetPendingKey(redPacketId int64) string {
	return fmt.Sprintf(PendingTemplate, redPacketId)
}

// 每事件一次性标志（stream msg.ID）：SETNX 成功才 DECR pending，保证「恰好一次」DECR
func GetSettledKey(eventID string) string {
	return fmt.Sprintf(SettledTemplate, eventID)
}
