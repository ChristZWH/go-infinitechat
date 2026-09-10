package dto

import "time"

// 消息体，承载消息的具体内容
//
// 普通文本消息只用 Content 字段；
// 回复消息额外带 ReplyId 指向被回复的消息；
// 红包消息用 RedPacketId + RedPacketWrapperText
type MessageBody struct {
	// 消息文本内容（普通聊天消息的正文）
	Content string `json:"content,omitempty"`

	// 被回复的消息ID（回复消息时才有值，普通消息为 nil）
	ReplyId *int64 `json:"replyId,omitempty"`

	// 红包ID（红包消息专用，普通消息为空）
	RedPacketId string `json:"redPacketId,omitempty"`

	// 红包封面文案，例如 "恭喜发财"（红包消息专用）
	RedPacketWrapperText string `json:"redPacketWrapperText,omitempty"`
}

// MessageRequest 客户端通过 WebSocket 发送的消息请求
//
// 完整流程：
//   客户端发 JSON → WebSocket 服务解析为 MessageRequest
//   → 服务端补充 MessageId + CreatedTime
//   → 序列化后发到 Kafka（store-topic 持久化 + message-topic 推送）
type MessageRequest struct {
	// 会话ID，标识这条消息属于哪个聊天会话
	SessionId int64 `json:"sessionId"`

	// 接收者用户ID
	// 单聊时必填（指定发给谁）；群聊时为 nil（群内所有人都能收到）
	ReceiverId *int64 `json:"receiverId,omitempty"`

	// 发送者用户ID
	SenderId int64 `json:"senderId"`

	// 消息类型：0-文本消息, 1-图片消息, 2-表情消息, 3-红包消息
	Type int `json:"type"`

	// 会话类型：0-单聊, 1-群聊, 2-机器人
	SessionType int `json:"sessionType"`

	// 消息体（包含文本内容、回复ID、红包信息等）
	Body *MessageBody `json:"body"`

	// 消息创建时间（由服务端填充，客户端不需要传）
	CreatedTime *time.Time `json:"createdTime,omitempty"`

	// 消息唯一ID（由服务端用雪花算法生成，客户端不需要传）
	MessageId int64 `json:"messageId,omitempty"`

	// 客户端消息ID（由客户端生成的临时ID，用于消息发送状态的前端回显对账）
	ClientMessageId string `json:"clientMessageId,omitempty"`
}

// 服务端推送给客户端的消息响应
type MessageResponse struct {
	// 会话ID
	SessionId int64 `json:"sessionId"`

	// 发送者用户ID
	SenderId int64 `json:"senderId"`

	//单聊时必填（指定发给谁）；群聊时为 nil（群内所有人都能收到）
	ReceiverId *int64 `json:"receiverId,omitempty"`

	// 消息类型：0-文本, 1-图片, 2-红包 等
	Type int `json:"type"`

	// 会话类型：0-单聊, 1-群聊, 2-机器人
	SessionType int `json:"sessionType"`

	// 消息体
	Body *MessageBody `json:"body"`

	// 消息创建时间（格式化后的字符串，如 "2024-01-15 14:30:00"）
	CreatedTime string `json:"createdTime"`

	// 消息唯一ID（雪花算法生成）
	MessageId int64 `json:"messageId"`

	// 客户端消息ID（原样回传，用于前端对账）
	ClientMessageId string `json:"clientMessageId,omitempty"`

	// 发送者昵称（群聊消息时由 Consumer 从数据库查出并填充）
	Nickname string `json:"nickname,omitempty"`

	// 发送者头像URL（群聊消息时由 Consumer 从数据库查出并填充）
	Avatar string `json:"avatar,omitempty"`

	// 发送者在群聊中的角色：0-群主, 1-管理员, 2-普通成员
	// 仅群聊消息有值，单聊为 nil
	Role *int `json:"role,omitempty"`
}

// MessageErrorResponse 消息发送失败时的错误响应
//
// 当消息参数校验失败（如单聊缺少接收者）或权限校验失败时，
// 通过 WebSocket 发送此响应给客户端，告知具体错误原因
type MessageErrorResponse struct {
	// 响应类型，固定为 "ERROR"
	Type string `json:"type"`

	// 错误码：40000-参数错误, 90000-单聊缺接收者, 90001-群聊多了接收者,
	// 91xxx-权限校验错误 等
	ErrorCode int `json:"errorCode"`

	// 错误描述信息，可直接展示给用户
	ErrorMessage string `json:"errorMessage"`

	// 客户端消息ID（原样回传，方便客户端定位是哪条消息发送失败）
	ClientMessageId string `json:"clientMessageId,omitempty"`

	// 错误发生的时间戳（毫秒）
	Timestamp int64 `json:"timestamp"`
}

const (
	MessageTypeText      = 0 //文本消息
	MessageTypeImage     = 1 //图片消息
	MessageTypeEmoji     = 2 //表情消息
	MessageTypeRedPacket = 3 //红包消息
)
