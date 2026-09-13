package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"go-infinitechat/common/common"
	"go-infinitechat/common/model/dto"
	"go-infinitechat/service/offline/model/message"

	"github.com/go-sql-driver/mysql"
)

// 消息持久化消费者（store-topic）
//
// 职责：只负责将消息写入 MySQL，不操作 Redis
// Redis 缓存更新由 CanalClient 监听 binlog 变更后异步处理
type MessageStoreConsumer struct {
	MessageModel message.MessageModel
}

func NewMessageStoreConsumer(msgModel message.MessageModel) *MessageStoreConsumer {
	return &MessageStoreConsumer{MessageModel: msgModel}
}

func (c *MessageStoreConsumer) Consume(ctx context.Context, key, val string) error {
	common.Infof("[offlineService-MessageStoreConsumer] 收到消息: %s", val)

	var msgReq dto.MessageRequest
	if err := json.Unmarshal([]byte(val), &msgReq); err != nil {
		common.Errorf("[offlineService-MessageStoreConsumer] 消息解析失败: %s", err.Error())
		return nil // 解析失败的消息重试也没用，跳过并提交 offset，避免毒丸消息卡死队列
	}

	// 只做 MySQL 持久化（Redis 缓存更新由 CanalClient 处理）
	if err := c.saveToMySQL(ctx, &msgReq); err != nil {
		// 返回 error → go-queue 不提交 offset，服务重启/重平衡后会重新投递，
		// 避免数据库故障期间的消息永久丢失
		return err
	}
	return nil
}

func (c *MessageStoreConsumer) saveToMySQL(ctx context.Context, msgReq *dto.MessageRequest) error {
	now := time.Now()

	msg := &message.Message{
		MessageId:   msgReq.MessageId,
		SenderId:    msgReq.SenderId,
		SessionId:   msgReq.SessionId,
		Type:        int64(msgReq.Type),
		SessionType: int64(msgReq.SessionType),
		UpdatedTime: now,
	}

	if msgReq.CreatedTime != nil {
		msg.CreatedTime = *msgReq.CreatedTime
	} else {
		msg.CreatedTime = now
	}

	// 红包消息：将整个 body 序列化为 JSON 存 content（与读侧 m.Type == 3 的解析逻辑对应）
	if msgReq.Body != nil {
		if msgReq.Type == dto.MessageTypeRedPacket {
			bodyJSON, err := json.Marshal(msgReq.Body)
			if err != nil {
				// body 刚从 JSON 反序列化出来，再序列化失败是编程 bug，重试无意义
				common.Errorf("[offlineService-MessageStoreConsumer] body 序列化失败: messageId=%d, err=%s", msgReq.MessageId, err.Error())
				return nil
			}
			msg.Content = string(bodyJSON)
		} else {
			msg.Content = msgReq.Body.Content
		}
		if msgReq.Body.ReplyId != nil {
			msg.ReplyId = sql.NullInt64{Int64: *msgReq.Body.ReplyId, Valid: true}
		}
	}

	if _, err := c.MessageModel.Insert(ctx, msg); err != nil {
		if isDuplicateKey(err) {
			// Kafka 重投导致的重复消息：message_id 主键冲突，视为已处理成功
			common.Infof("[offlineService-MessageStoreConsumer] 消息已存在，跳过重复投递: messageId=%d", msgReq.MessageId)
			return nil
		}
		return err
	}
	return nil
}

// isDuplicateKey 判断是否为 MySQL 主键/唯一键冲突（错误码 1062）
func isDuplicateKey(err error) bool {
	var myErr *mysql.MySQLError
	return errors.As(err, &myErr) && myErr.Number == 1062
}
