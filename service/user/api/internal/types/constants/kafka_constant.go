package constants

import "go-infinitechat/common/model/constants"

var (
	// 好友申请创建事件Topic
	KafkaFriendRequestCreationTopic = "friend-request-creation-topic"

	// 好友申请过期事件Topic
	KafkaFriendRequestExpirationTopic = "friend-request-expiration-topic"
)

/*
 * 消息类型常量
 */
const (
	TypeSystemNewApply        = 101 // 收到好友申请通知
	TypeSystemNewSession      = 102 // 新会话创建通知
	TypeSystemNewGroupSession = 103 // 新群聊会话创建通知
	TypeSystemGroupKick       = 104 // 群聊踢出通知
)

/*
 * 会话类型常量
 */
const (
	SessionTypeSignal = 0 // 单聊
	SessionTypeGroup  = 1 // 群聊
	SessionTypeRobot  = 2 // 机器人
)

// 好友申请过期时间（24小时，毫秒）
const FriendRequestExpirationMillis = 24 * 60 * 60 * 1000

// 返回 UserService 需要初始化的所有 Kafka Topic 列表
func AllKafkaTopics() []string {
	return []string{
		constants.KafkaMessageTopicStore,
		constants.KafkaMessageTopicPush,
		constants.KafkaSystemNotificationTopic,
		constants.KafkaStoreNotificationTopic,
		KafkaFriendRequestCreationTopic,
		KafkaFriendRequestExpirationTopic,
	}
}
