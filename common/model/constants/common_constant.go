package constants

import "fmt"

var (
	KafkaMessageTopicStore = "store-topic"
	KafkaMessageTopicPush  = "message-topic"

	// 系统通知消息 Kafka
	KafkaSystemNotificationTopic = "system-notification-topic"

	// 系统通知持久化 Kafka 主题
	KafkaStoreNotificationTopic = "store-notification-topic"

	RedisWsServerUri    = "wsServerUri"
	DiscoveryClientName = "RealTimeService"
	AiRoleOpen          = "AI:ROLE:OPEN"
	SevenDaysMillis     = int64(7 * 24 * 3600 * 1000)
	LimitMessageCount   = 30
	WsServiceUri        = "/ws/chat"

	// 手机验证码（暂时固定）
	PhoneCode = "123456"
)

// FriendStatusCachePrefix 好友状态缓存前缀。
// user-api（写后清理）与 user-rpc（读校验）必须用同一份拼接规则，
// 否则清理端删不到读取端的 key，会出现永久脏缓存。
const FriendStatusCachePrefix = "msg:validate:friend:status:"

// FriendStatusCacheKey 统一的好友状态缓存 key：prefix + userId + ":" + friendId。
// 中间分隔符不能省：(1,23) 与 (12,3) 拼出来的 key 必须不同。
func FriendStatusCacheKey(userId, friendId int64) string {
	return fmt.Sprintf("%s%d:%d", FriendStatusCachePrefix, userId, friendId)
}

