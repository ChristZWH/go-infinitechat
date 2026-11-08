package constants

import commonconsts "go-infinitechat/common/model/constants"

const (
	// 红包被全部领完后，将领完事件推送到此 Topic（CompletedConsumer 消费：状态置已领完 + 清缓存）
	TopicRedpacketCompleted = "topic-redpacket-completed"

	// 过期扫描调度器扫描到过期红包后，将过期事件推送到此 Topic（ExpirationConsumer 消费：退款）
	TopicRedpacketExpiration = "redpacket-expiration-topic"
)

// AllRedPacketKafkaTopics 返回 redpacket 服务所有需要创建 Pusher 的 topic。
//
// 红包消息的 store/push 复用公共 topic（store-topic / message-topic，见 common/model/constants），
// 不再使用历史遗留的错误常量 message-store-topic / message-push-topic——
// 那俩名字和 offline/realtime 实际消费的 topic 对不上，导致红包消息一直推不出去。
func AllRedPacketKafkaTopics() []string {
	return []string{
		commonconsts.KafkaMessageTopicStore, // store-topic：红包消息落库（发件箱投递）
		commonconsts.KafkaMessageTopicPush,  // message-topic：红包消息推送（发件箱投递）
		TopicRedpacketCompleted,
		TopicRedpacketExpiration,
	}
}
