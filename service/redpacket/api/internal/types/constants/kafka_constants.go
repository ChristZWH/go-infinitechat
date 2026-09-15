package constants

const (
	//发送红包成功后，将红包创建事件推送到此 Topic
	TopicRedpacketCreation = "redpacket-creation-topic"

	//用户领取红包后，将领取记录推送到此 Topic
	TopicRedpacketReceive = "topic-redpacket-receive"

	//红包被全部领完后，将领完事件推送到此 Topic
	TopicRedpacketCompleted = "topic-redpacket-completed"

	//过期扫描调度器扫描到过期红包后，将过期事件推送到此 Topic
	TopicRedpacketExpiration = "redpacket-expiration-topic"

	//红包消息发送时，推送到此 Topic
	TopicMessageStore = "message-store-topic"

	//红包消息发送时，推送到此 Topic
	TopicMessagePush = "message-push-topic"
)

func AllRedPacketKafkaTopics() []string {
	return []string{
		TopicRedpacketCreation,
		TopicRedpacketReceive,
		TopicRedpacketCompleted,
		TopicRedpacketExpiration,
		TopicMessageStore,
		TopicMessagePush,
	}
}
