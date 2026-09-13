package constants

import "fmt"

const (
	PoolTemplate           = "redpacket:%d:pool"
	RecordsTemplate        = "redpacket:%d:records"
	ExpireZSet             = "redpacket-expire-zset"
	PreventDuplicatePrefix = "prevent:duplicate:"
)

func GetPoolKey(redPacketId int64) string {
	return fmt.Sprintf(PoolTemplate, redPacketId)
}

func GetRecordsKey(redPacketId int64) string {
	return fmt.Sprintf(RecordsTemplate, redPacketId)
}
