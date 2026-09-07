package service

import (
	"context"
	"fmt"
	"go-infinitechat/common/common"
	"go-infinitechat/common/kafka"
	"go-infinitechat/common/model/constants"
	"go-infinitechat/common/utils"
	constants2 "go-infinitechat/service/user/api/internal/types/constants"
	"go-infinitechat/service/user/api/internal/types/dto"
	"strconv"
	"time"
)

// 通知推送服务
// 功能说明：
// - 使用Kafka异步发送系统通知消息
// - 替代原有的HTTP同步调用方式
// - 符合IM项目通知消息设计方案

type NotificationService struct {
	PusherManager *kafka.PusherManager
}

func NewNotificationService(pm *kafka.PusherManager) *NotificationService {
	return &NotificationService{PusherManager: pm}
}

// 推送新会话通知 userId为消息接收者
func (ns *NotificationService) PushNewSession(ctx context.Context, senderId, userId, sessionId int64, sessionType int, notification dto.NewSessionNotificationDTO) {
	message := dto.SystemNotificationMessage{
		MessageId:   generateMessageId(),
		SessionId:   &sessionId,
		SenderId:    &senderId,
		ReceiverId:  userId,
		Type:        constants2.TypeSystemNewSession,
		SessionType: &sessionType,
		Timestamp:   time.Now().UnixMilli(),
		Body: map[string]interface{}{
			"sessionName": notification.SessionName,
			"avatar":      notification.Avatar,
		},
	}

	ns.sendNotification(ctx, message, "新会话通知")
}

// 推送好友申请 userId 为通知接收者
func (ns *NotificationService) PushNewApply(ctx context.Context, userId int64, notification dto.FriendApplicationNotificationDTO) {
	senderId := notification.ApplyUserId
	message := dto.SystemNotificationMessage{
		MessageId:   generateMessageId(),
		SessionId:   nil,
		SenderId:    &senderId,
		ReceiverId:  userId,
		Type:        constants2.TypeSystemNewApply, //101
		SessionType: nil,
		Timestamp:   time.Now().UnixMilli(),
		Body: map[string]interface{}{
			"nickname": notification.ApplyUserName,
			"avatar":   notification.ApplyFriendAvatar,
			"msg":      notification.Message,
		},
	}
	ns.sendNotification(ctx, message, "好友申请通知")
}

// 推送新群聊会话通知 userID为通知接收者
func (ns *NotificationService) PushGroupNewSession(ctx context.Context, userId, sessionId int64, notification dto.NewGroupSessionNotificationDTO) {
	groupType := constants2.SessionTypeGroup
	message := dto.SystemNotificationMessage{
		MessageId:   generateMessageId(),
		SessionId:   &sessionId,
		SenderId:    nil,
		ReceiverId:  userId,
		Type:        constants2.TypeSystemNewGroupSession, //103
		SessionType: &groupType,
		Timestamp:   time.Now().UnixMilli(),
		Body: map[string]interface{}{
			"sessionName":  notification.SessionName,
			"avatar":       notification.Avatar,
			"creatorId":    notification.CreatorId,
			"membersCount": notification.MembersCount,
		},
	}

	ns.sendNotification(ctx, message, "群组邀请通知")
}

// 推送群聊踢出通知 receiver为通知接收者
func (ns *NotificationService) PushGroupKickNotification(ctx context.Context, receiverId, sessionId int64, notification dto.GroupKickNotificationDTO) {
	groupType := constants2.SessionTypeGroup
	message := dto.SystemNotificationMessage{
		MessageId:   generateMessageId(),
		SessionId:   &sessionId,
		SenderId:    nil,
		ReceiverId:  receiverId,
		Type:        constants2.TypeSystemGroupKick, // 104
		SessionType: &groupType,
		Timestamp:   time.Now().UnixMilli(),
		Body: map[string]interface{}{
			"memberIds":  notification.MemberIds,
			"operatorId": notification.OperatorId,
		},
	}

	ns.sendNotification(ctx, message, "群聊踢出通知")
}

// 发送通知消息到 Kafka
// 使用 receiverId 作为key, 保证同一用户的消息顺序
func (ns *NotificationService) sendNotification(ctx context.Context, message dto.SystemNotificationMessage, notificationName string) {
	if ns.PusherManager == nil {
		common.Errorf("PusherManager 未初始化，跳过%s通知", notificationName)
		return
	}

	key := strconv.FormatInt(message.ReceiverId, 10)

	if err := ns.PusherManager.KPush(ctx, constants.KafkaSystemNotificationTopic, key, message); err != nil {
		common.Errorf("%s通知发送失败, userId: %d, error: %s", notificationName, message.ReceiverId, err.Error())
	}
}

func generateMessageId() string {
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixMilli(), utils.NextInt())
}
