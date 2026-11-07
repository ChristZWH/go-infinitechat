package constants

import "fmt"

const (
	PoolTemplate           = "redpacket:%d:pool"
	RecordsTemplate        = "redpacket:%d:records"
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
