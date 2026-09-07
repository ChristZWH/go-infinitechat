package constants

var (
	KafkaMessageTopicStore = "stroe-topic"
	KafkaMessageTopicPush  = "message-topic"

	// 系统通知消息 Kafka
	KafkaSystemNotificationTopic = "system-notifaction-topic"

	// 系统通知持久化 Kafka 主题
	KafkaStoreNotificationTopic = "store-notifaction-topic"

	RedisWsServerUri    = "wsServerUri"
	DiscoveryClientName = "RealTimeService"
	AiRoleOpen          = "AI:ROLE:OPEN"
	SevenDaysMillis     = int64(7 * 24 * 3600 * 1000)
	LimitMessageCount   = 30
	WsServiceUri        = "/ws/chat"

	// 手机验证码（暂时固定）
	PhoneCode = "123456"
)
