package consumer

import (
	"context"
	"encoding/json"
	"go-infinitechat/common/common"
	rtc "go-infinitechat/service/realtime/internal/constants"
	"go-infinitechat/service/realtime/internal/svc"
	"go-infinitechat/service/realtime/internal/websocket"
	"strconv"
)

// 系统通知消费者
//
//	消费 system-notification-topic 中的系统通知消息，处理流程：
//	  1. 解析消息，提取 receiverId
//	  2. 检查接收者是否在当前节点在线
//	  3. 在线 → 通过 WebSocket 直接推送
//	  4. 不在线 → 检查 Redis 判断是否在其他节点在线
//	  5. 确认离线 → 转发到 store-notification-topic 进行持久化
//	通知消息类型（type 字段）：
//	  101: 收到好友申请
//	  102: 新单聊会话创建
//	  103: 新群聊会话创建（群组邀请）
//	  104: 被踢出群聊
type Notification struct {
	svcCtx *svc.ServiceContext
}

func NewNotificationConsumer(svc *svc.ServiceContext) *Notification {
	return &Notification{svcCtx: svc}
}

// 通知消息的最小解析结构
//
// 只提取推送路由需要的字段（receiverId, messageId, type），
// 不需要解析完整的 body 内容 —— 直接把原始 JSON 原样推给客户端
type notificationMessage struct {
	MessageId  string `json:"messageId"`
	ReceiverId int64  `json:"receiverId"`
	Type       int    `json:"type"`
}

// Consume 实现 go-queue 的 ConsumeHandler 接口

// 参数:
//
//	key - Kafka 消息的 key（即 receiverId，保证同一用户的通知有序）
//	val - Kafka 消息的 value（JSON 格式的 SystemNotificationMessage）
func (c *Notification) Consume(ctx context.Context, key, val string) error {
	common.Debugf("[NotificationConsumer] 收到系统通知: key=%s", key)

	// 1. 解析消息，只取路由需要的字段
	var msg notificationMessage
	if err := json.Unmarshal([]byte(val), &msg); err != nil {
		common.Errorf("[NotificationConsumer] 通知消息解析失败: err=%s, raw=%s", err.Error(), val)
		return nil // 格式错误不重试
	}

	receiverId := strconv.FormatInt(msg.ReceiverId, 10)

	// 2. 尝试通过本节点的 ChannelManager 推送
	if ok := websocket.SendMessageToUser(c.svcCtx.ChannelManager, receiverId, []byte(msg.MessageId)); !ok {
		// 用户在当前节点在线，推送成功
		common.Infof("[NotificationConsumer] 系统通知推送成功: messageId=%s, receiverId=%s, type=%d",
			msg.MessageId, receiverId, msg.Type)
		return nil
	}

	// 3. 本节点没找到连接，检查 Redis 判断用户是否在其他节点在线
	isOnline, _ := c.svcCtx.Redis.Hexists(rtc.RedisWsServerUri, receiverId)
	if isOnline {
		// 用户在其他节点在线，由其他节点的 Consumer 负责推送，本节点忽略
		common.Debugf("[NotificationConsumer] 用户在其他节点在线，跳过: receiverId=%s", receiverId)
		return nil
	}

	// 4. 用户确实离线，转发到 store-notification-topic 进行持久化
	// 用户下次上线时，客户端会拉取离线通知
	if c.svcCtx.NotificationStorePusher != nil {
		if err := c.svcCtx.NotificationStorePusher.Push(ctx, val); err != nil {
			common.Errorf("[NotificationConsumer] 离线通知持久化发送失败: messageId=%s, receiverId=%s, err=%s", msg.MessageId, receiverId, err.Error())
		} else {
			common.Infof("[NotificationConsumer] 用户离线，通知已转发持久化: messageId=%s, receiverId=%s, type=%d", msg.MessageId, receiverId, msg.Type)
		}
	} else {
		common.Warnf("[NotificationConsumer] NotificationStorePusher 未初始化，离线通知丢失: messageId=%s, receiverId=%s", msg.MessageId, receiverId)
	}

	return nil
}
