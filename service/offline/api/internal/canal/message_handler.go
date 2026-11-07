package canal

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go-infinitechat/common/model/constants"
	dto "go-infinitechat/common/model/dto"
	"go-infinitechat/service/user/rpc/user"
	"go-infinitechat/service/user/rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const onlineUserHashKey = "wsServerUri"

// MessageHandler 处理 InfiniteChat.message 表的 INSERT：
// 把新落库的消息同步到 Redis（会话 ZSet、活跃榜、离线快照、未读数）。
// 它实现 common/canal.Handler 接口，由通用 Runner 驱动。
type MessageHandler struct {
	rds     *redis.Redis
	sqlConn sqlx.SqlConn
	userRpc userrpc.UserRpc
}

func NewMessageHandler(rds *redis.Redis, conn sqlx.SqlConn, userRpc userrpc.UserRpc) *MessageHandler {
	return &MessageHandler{rds: rds, sqlConn: conn, userRpc: userRpc}
}

// Handle 实现 common/canal.Handler。
func (m *MessageHandler) Handle(schema, table string, columns map[string]string) error {
	logx.Infof("表名：%s.%s，数据：%v", schema, table, columns)
	return m.storeMessageToRedis(buildMessageFromMap(columns))
}

type MessageRedisResponse struct {
	SessionId   int64           `json:"sessionId"`
	SenderId    int64           `json:"senderId"`
	MessageId   int64           `json:"messageId"`
	Type        int             `json:"type"`
	SessionType int             `json:"sessionType"`
	CreatedTime string          `json:"createdTime"`
	Body        *dto.MessageBody `json:"body"`
	Name        string          `json:"name"`
	Avatar      string          `json:"avatar"`
}

func buildMessageFromMap(colMap map[string]string) *MessageRedisResponse {
	resp := &MessageRedisResponse{}
	resp.SessionId, _ = strconv.ParseInt(colMap["session_id"], 10, 64)
	resp.SenderId, _ = strconv.ParseInt(colMap["sender_id"], 10, 64)
	resp.MessageId, _ = strconv.ParseInt(colMap["message_id"], 10, 64)
	resp.Type, _ = strconv.Atoi(colMap["type"])
	resp.SessionType, _ = strconv.Atoi(colMap["session_type"])
	resp.CreatedTime = colMap["created_time"]

	// 红包消息：type = MessageTypeRedPacket，content 存的是序列化后的 MessageBody JSON
	if resp.Type == dto.MessageTypeRedPacket {
		body := &dto.MessageBody{}
		_ = json.Unmarshal([]byte(colMap["content"]), body)
		resp.Body = body
	} else {
		body := &dto.MessageBody{Content: colMap["content"]}
		if v := colMap["reply_id"]; v != "" {
			replyId, _ := strconv.ParseInt(v, 10, 64)
			body.ReplyId = &replyId
		}
		resp.Body = body
	}
	return resp
}

// storeMessageToRedis 把一条消息同步到 Redis 四件套。
// 返回 error 表示同步失败（如查成员失败），Runner 会据此决定不 Ack。
func (m *MessageHandler) storeMessageToRedis(msg *MessageRedisResponse) error {
	redisKey := fmt.Sprintf("session:%d", msg.SessionId)

	// 1. 查询会话成员
	var receiverIds []int64
	if err := m.sqlConn.QueryRowsCtx(context.Background(), &receiverIds,
		"SELECT user_id FROM user_session WHERE session_id = ?", msg.SessionId); err != nil {
		logx.Errorf("查询会话成员失败, sessionId=%d: %v", msg.SessionId, err)
		return err
	}
	if len(receiverIds) == 0 {
		return nil
	}

	// 2. RPC 补充昵称/头像
	userInfoMap := m.getUserInfoMap(receiverIds)
	if info, ok := userInfoMap[msg.SenderId]; ok {
		msg.Name = info.Nickname
		msg.Avatar = info.Avatar
	}

	messageJson, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	t, err := time.ParseInLocation("2006-01-02 15:04:05", msg.CreatedTime, loc)
	if err != nil {
		logx.Errorf("解析消息时间失败: %s", err.Error())
		return nil // 时间解析失败：跳过该条（不阻塞整批 Ack）
	}
	score := t.UnixMilli()

	// 3. 获取离线用户
	offlineUserIds := m.getOfflineUserIds(receiverIds)

	// 4. 写入 Redis 四件套
	// 4.1 会话 ZSet + 清理 7 天前旧消息
	_, _ = m.rds.Zadd(redisKey, score, string(messageJson))
	_, _ = m.rds.Zremrangebyscore(redisKey, 0, time.Now().UnixMilli()-constants.SevenDaysMillis)

	// 4.2 活跃会话榜（当前为全局榜，用户维度榜单属待办）
	_, _ = m.rds.Zadd("session:active", score, strconv.FormatInt(msg.SessionId, 10))

	// 4.3 离线用户的会话快照 + 未读计数
	for _, receiverId := range receiverIds {
		if receiverId == msg.SenderId {
			continue
		}
		if !offlineUserIds[receiverId] {
			continue
		}

		objectKey := strconv.FormatInt(msg.SessionId, 10)

		lastMsgContent := ""
		if msg.Type == dto.MessageTypeRedPacket && msg.Body != nil {
			lastMsgContent = "[红包] " + msg.Body.RedPacketWrapperText
		} else if msg.Body != nil {
			lastMsgContent = msg.Body.Content
		}

		sessionFields, _ := json.Marshal(map[string]string{
			"type":           strconv.Itoa(msg.Type),
			"sessionType":    strconv.Itoa(msg.SessionType),
			"senderId":       strconv.FormatInt(msg.SenderId, 10),
			"avatar":         msg.Avatar,
			"name":           msg.Name,
			"lastMsgContent": lastMsgContent,
			"lastMsgTime":    msg.CreatedTime,
		})
		_ = m.rds.Hset(fmt.Sprintf("user:%d", receiverId), objectKey, string(sessionFields))
		_, _ = m.rds.Hincrby(fmt.Sprintf("user:%d:count", receiverId), objectKey, 1)
	}
	return nil
}

func (m *MessageHandler) getUserInfoMap(userIds []int64) map[int64]*user.BatchUserInfoItem {
	result := make(map[int64]*user.BatchUserInfoItem)
	resp, err := m.userRpc.BatchGetUserInfos(context.Background(), &user.BatchGetUserInfosReq{UserIds: userIds})
	if err != nil || resp == nil {
		return result
	}
	for _, item := range resp.GetUsers() {
		result[item.UserId] = item
	}
	return result
}

// getOfflineUserIds 返回 userId 集合中处于离线状态的（从 wsServerUri 判断）。
func (m *MessageHandler) getOfflineUserIds(userIds []int64) map[int64]bool {
	offline := make(map[int64]bool)
	for _, uid := range userIds {
		exists, _ := m.rds.Hexists(onlineUserHashKey, strconv.FormatInt(uid, 10))
		if !exists {
			offline[uid] = true
		}
	}
	return offline
}
