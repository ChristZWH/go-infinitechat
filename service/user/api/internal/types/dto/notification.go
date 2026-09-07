package dto

/*
 * 系统通知消息DTO
 *
 * 功能说明：
 * - 统一的系统通知消息格式，符合IM项目通知消息设计方案
 * - 通过type字段区分不同类型的系统通知
 * - 通过Kafka传输，由RealTimeService消费并推送给用户
 *
 * 消息类型（Type字段）：
 * - 101：收到好友申请通知
 * - 102：新会话创建通知
 * - 103：新群聊会话创建通知（群组邀请）
 * - 104：群聊踢出通知
 *
 */
type SystemNotificationMessage struct {
	// 消息唯一ID，格式：msg_{timestamp}_{snowflakeId}
	MessageId string `json:"messageId"`

	// 会话ID（可为nil）
	SessionId *int64 `json:"sessionId"`

	// 发送者ID
	SenderId *int64 `json:"senderId"`

	// 接收者用户ID
	ReceiverId int64 `json:"receiverId"`

	// 消息类型: 101-好友申请, 102-新会话, 103-群聊邀请, 104-群聊踢出
	Type int `json:"type"`

	// 会话类型（可为nil）: 0-单聊, 1-群聊, 2-机器人
	SessionType *int `json:"sessionType"`

	// 消息创建时间戳（毫秒）
	Timestamp int64 `json:"timestamp"`

	// 消息体（业务数据）
	Body map[string]interface{} `json:"body"`
}

// 好友申请通知DTO
type FriendApplicationNotificationDTO struct {
	ApplyUserName     string `json:"applyUserName"`
	ApplyUserId       int64  `json:"applyUserId"`
	ApplyFriendAvatar string `json:"applyFriendAvatar"`
	Message           string `json:"message"`
}

// 新会话通知DTO
type NewSessionNotificationDTO struct {
	SessionName string `json:"sessionName"`
	Avatar      string `json:"avatar"`
}

// 新群聊会话通知DTO
type NewGroupSessionNotificationDTO struct {
	SessionName  string `json:"sessionName"`
	Avatar       string `json:"avatar"`
	CreatorId    int64  `json:"creatorId"`
	MembersCount int    `json:"membersCount"`
}

// 群聊踢出通知DTO
type GroupKickNotificationDTO struct {
	MemberIds  []int64 `json:"memberIds"`
	OperatorId int64   `json:"operatorId"`
}

// 好友申请创建事件
type FriendRequestCreationEvent struct {
	ApplyFriendId int64 `json:"applyFriendId"`
	CreateTime    int64 `json:"createTime"`
	ExpireTime    int64 `json:"expireTime"`
}
