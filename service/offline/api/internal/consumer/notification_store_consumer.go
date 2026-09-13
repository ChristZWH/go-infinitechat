package consumer

import (
	"context"
	"encoding/json"
	"time"

	"go-infinitechat/common/common"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/offline/model/system_notification"
)

type NotificationStoreConsumer struct {
	Model system_notification.SystemNotificationModel
}

func NewNotificationStoreConsumer(model system_notification.SystemNotificationModel) *NotificationStoreConsumer {
	return &NotificationStoreConsumer{Model: model}
}

type notificationMsg struct {
	MessageId  string `json:"messageId"`
	ReceiverId int64  `json:"receiverId"`
	Type       int64  `json:"type"`
}

func (c *NotificationStoreConsumer) Consume(ctx context.Context, key, val string) error {
	common.Debugf("[NotificationStoreConsumer] 收到通知: %s", val)

	var msg notificationMsg
	if err := json.Unmarshal([]byte(val), &msg); err != nil {
		common.Errorf("[NotificationStoreConsumer] 解析失败: %s", err.Error())
		return nil
	}

	now := time.Now()

	notification := &system_notification.SystemNotification{
		Id:          utils.NextInt(),
		MessageId:   msg.MessageId,
		ReceiverId:  msg.ReceiverId,
		Type:        msg.Type,
		Content:     val, // 存完整 JSON
		IsRead:      0,
		CreatedTime: now,
		UpdatedTime: now,
	}

	// 保存到数据库中
	if _, err := c.Model.Insert(ctx, notification); err != nil {
		// messageId 唯一索引冲突（1062）→ 重复消费，视为成功并提交 offset，
		// 否则返回 err 会导致重启后无限重投
		if isDuplicateKey(err) {
			common.Warnf("[NotificationStoreConsumer] 重复通知，跳过: messageId=%s", msg.MessageId)
			return nil
		}
		common.Errorf("[NotificationStoreConsumer] 持久化失败: messageId=%s, err=%s", msg.MessageId, err.Error())
		return err
	}

	common.Infof("[NotificationStoreConsumer] 通知持久化成功: messageId=%s, receiverId=%d, type=%d", msg.MessageId, msg.ReceiverId, msg.Type)
	return nil
}
