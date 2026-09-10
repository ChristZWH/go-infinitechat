package consumer

import (
	"context"
	"encoding/json"
	"go-infinitechat/common/common"
	"go-infinitechat/common/model/dto"
	rtc "go-infinitechat/service/realtime/internal/constants"
	"go-infinitechat/service/realtime/internal/svc"
	ws "go-infinitechat/service/realtime/internal/websocket"
	"go-infinitechat/service/user/rpc/user"
	"strconv"
	"time"
)

// 消息消费者
type MessageConsumer struct {
	svcCtx *svc.ServiceContext
}

func NewMessageConsumer(svcCtx *svc.ServiceContext) *MessageConsumer {
	return &MessageConsumer{svcCtx: svcCtx}
}

// Consume 实现 go-queue 的 kq.ConsumeHandler 接口
//
// 每消费一条 Kafka 消息就调用一次
//
// 参数:
//   - key - Kafka 消息的 key（即 sessionId，用于保证同一会话消息有序）
//   - val - Kafka 消息的 value（JSON 格式的 MessageRequest）
func (c *MessageConsumer) Consume(ctx context.Context, key, val string) error {
	common.Infof("[MessageConsumer] 收到消息: key=%s, value=%s", key, val)

	var msgReq dto.MessageRequest
	if err := json.Unmarshal([]byte(val), &msgReq); err != nil {
		common.Errorf("[MessageConsumer] 消息解析失败: err=%s, raw=%s", err.Error(), val)
		return nil
	}

	switch msgReq.SessionType {
	case rtc.SessionTypeSignal:
		c.handleSignalMessage(&msgReq)
	case rtc.SessionTypeGroup:
		c.handleGroupMessage(ctx, &msgReq)
	case rtc.SessionTypeRobot:
		common.Infof("[MessageConsumer] 机器人消息暂不处理: sessionId=%d", msgReq.SessionId)
	default:
		common.Warnf("[MessageConsumer] 未知的 sessionType=%d", msgReq.SessionType)
	}

	return nil
}

// handleSignalMessage 处理单聊消息：推送给发送者和接收者
// 流程：
//  1. 将 MessageRequest 转成 MessageResponse（补充格式化时间）
//  2. 序列化为 JSON
//  3. 推送给发送者（让发送者的其他设备同步 / 确认消息已送达）
//  4. 推送给接收者
func (c *MessageConsumer) handleSignalMessage(msgReq *dto.MessageRequest) {
	msgResp := createMessageResponse(msgReq)
	respJSON, err := json.Marshal(msgResp)
	if err != nil {
		common.Errorf("[MessageConsumer][single] 消息序列化失败,msgReq:%+v, msgResp:%+v, 原因:%s", msgReq, msgResp, err.Error())
		return
	}

	// 推送给发送者
	senderId := strconv.FormatInt(msgReq.SenderId, 10)
	if ok := ws.SendMessageToUser(c.svcCtx.ChannelManager, senderId, respJSON); ok {
		common.Infof("[MessageConsumer][single] 单聊消息推送给 '发送者' 成功: fromUserId=%d, toUserId=%d, sessionId=%d, messageId=%d", msgReq.SenderId, *msgReq.ReceiverId, msgReq.SessionId, msgReq.MessageId)
	} else {
		common.Warnf("[MessageConsumer][single] 单聊消息推送给 '发送者' 成功: fromUserId=%d, toUserId=%d, sessionId=%d, messageId=%d", msgReq.SenderId, *msgReq.ReceiverId, msgReq.SessionId, msgReq.MessageId)
	}

	// 推送给接收者
	if msgReq.ReceiverId != nil {
		receiverId := strconv.FormatInt(*msgReq.ReceiverId, 10)
		if ok := ws.SendMessageToUser(c.svcCtx.ChannelManager, receiverId, respJSON); ok {
			common.Infof("[MessageConsumer][single] 单聊消息推送给 '接收者' 成功: fromUserId=%d, toUserId=%d, sessionId=%d, messageId=%d", msgReq.SenderId, *msgReq.ReceiverId, msgReq.SessionId, msgReq.MessageId)
		} else {
			common.Infof("[MessageConsumer][single] 接收者不在线: userId=%s, messageId=%d", receiverId, msgReq.MessageId)
		}
	}
}

// 处理群聊消息：推送给群内所有在线成员
func (c *MessageConsumer) handleGroupMessage(ctx context.Context, msgReq *dto.MessageRequest) {
	sessionId := msgReq.SessionId
	senderId := msgReq.SenderId

	// 1.通过 RPC 获取群员列表
	memberResp, err := c.svcCtx.UserRpc.GetGroupMemberIds(ctx, &user.SessionIdReq{
		SessionId: sessionId,
	})
	if err != nil {
		common.Errorf("[MessageConsumer][group] RPC 查询群成员失败: sessionId=%d, err=%s", sessionId, err.Error())
		return
	}

	// 2. RPC 查询发送者信息（昵称、头像）
	userInfoResp, err := c.svcCtx.UserRpc.GetUserInfo(ctx, &user.UserIdReq{
		UserId: senderId,
	})
	if err != nil {
		common.Errorf("[MessageConsumer][group] RPC 查询发送者信息失败: senderId=%d, err=%s", senderId, err.Error())
		return
	}

	// 3. RPC 查询发送者在群中的角色
	roleResp, err := c.svcCtx.UserRpc.GetUserSessionRole(ctx, &user.UserSessionRoleReq{
		SessionId: sessionId,
		UserId:    senderId,
	})
	if err != nil {
		common.Errorf("[MessageConsumer][group] RPC 查询发送者角色失败: senderId=%d, sessionId=%d, err=%s", senderId, sessionId, err.Error())
		return
	}

	// 4. 构建 MessageResponsse
	msgResp := createMessageResponse(msgReq)
	msgResp.Nickname = userInfoResp.Nickname
	msgResp.Avatar = userInfoResp.Avatar
	role := int(roleResp.Role)
	msgResp.Role = &role

	respJSON, err := json.Marshal(msgResp)
	if err != nil {
		common.Errorf("[MessageConsumer][group] 群聊消息序列化失败: %s", err.Error())
		return
	}

	// 5. 优先推送给发送者自己
	senderIdStr := strconv.FormatInt(senderId, 10)
	ws.SendMessageToUser(c.svcCtx.ChannelManager, senderIdStr, respJSON)

	// 6. 遍历群成员，推送给其他在线成员
	for _, memId := range memberResp.UserIds {
		if memId == senderId {
			continue
		}
		if ok := ws.SendMessageToUser(c.svcCtx.ChannelManager, strconv.FormatInt(memId, 10), respJSON); ok {
			common.Debugf("[MessageConsumer][group] 群聊消息推送成功: userId=%s, sessionId=%d", memId, sessionId)
		}
	}
	common.Infof("[MessageConsumer][group] 群聊消息推送完成: sessionId=%d, senderId=%d, memberCount=%d, messageId=%d", sessionId, senderId, len(memberResp.UserIds), msgReq.MessageId)
}

func createMessageResponse(msgReq *dto.MessageRequest) *dto.MessageResponse {
	createdTime := ""
	if msgReq.CreatedTime != nil {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		createdTime = msgReq.CreatedTime.In(loc).Format("2006-01-02 15:04:05") //时间格式化的模版,时间随便写,格式对了就行了
	}

	return &dto.MessageResponse{
		SessionId:       msgReq.SessionId,
		SenderId:        msgReq.SenderId,
		ReceiverId:      msgReq.ReceiverId,
		Type:            msgReq.Type,
		SessionType:     msgReq.SessionType,
		Body:            msgReq.Body,
		CreatedTime:     createdTime,
		MessageId:       msgReq.MessageId,
		ClientMessageId: msgReq.ClientMessageId,
	}
}
