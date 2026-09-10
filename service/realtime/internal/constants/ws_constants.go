package constants

// 会话类型
//
// 用于 MessageRequest.SessionType 字段，决定消息的路由逻辑：
//   - 单聊：消息只推给 senderId 和 receiverId 两个人
//   - 群聊：消息推给该 sessionId 下所有成员
//   - 机器人：消息发给 AI 服务，AI 回复后推回给发送者
const (
	SessionTypeSignal = iota // 单聊
	SessionTypeGroup         // 群聊
	SessionTypeRobot         // 机器人（AI 聊天）
)

// WebSocket 心跳
//
// 客户端定期发 "ping"，服务端回 "pong"，用于保活连接。
// 如果 60 秒内没收到任何消息（包括 ping），服务端判定连接超时并主动断开。
// 这里是应用层心跳（纯文本），不是 WebSocket 协议层的 Ping/Pong 帧。
const (
	HeartbeatPing = "ping" // 客户端发送的心跳请求
	HeartbeatPong = "pong" // 服务端回复的心跳响应
)

// 消息类型标识
const (
	// MessageTypeError 错误响应的 type 字段值
	// 客户端收到 type="ERROR" 的消息时，应展示错误提示而非聊天气泡
	MessageTypeError = "ERROR"
)

// 错误码
//
// 当消息参数校验失败时，通过 MessageErrorResponse 返回给客户端，
// 客户端可以根据错误码做不同的 UI 提示
const (
	ErrorCodeParamsError = 40000 // 通用参数错误（如 JSON 格式不对）
	ErrorCodeSignalType  = 90000 // 单聊消息必须指定 receiverId
	ErrorCodeGroupType   = 90001 // 群聊消息不能指定 receiverId（群聊是广播给所有成员的）
)

// Redis Key
const (
	// RedisWsServerUri 是一个 Redis Hash 结构，记录每个在线用户连接在哪个 WebSocket 节点上
	//
	// Hash Key "wsServerUri"
	// Hash Field：userId（字符串形式）
	// Hash Value：WebSocket 节点地址，如 "115.290.231.67:9101"
	//
	// 用途：当多个 WebSocket 节点部署时，其他服务（如 UserService）需要知道某个用户连接在那个节点上，才能把消息路由到正确的节点去推送。用户上线时写入，下线时删除。
	RedisWsServerUri = "wsServerUri"
)

const (
	ErrorCodeValidationFailed  = 91001 // 消息校验失败（通用）
	ErrorCodeNotFriend         = 91002 // 对方不是您的好友
	ErrorCodeBlockedByReceiver = 91003 // 您已被对方拉黑
	ErrorCodeFriendDeleted     = 91004 // 好友关系已删除
	ErrorCodeNotGroupMember    = 91005 // 您不是该群成员
	ErrorCodeSenderDisabled    = 91006 // 发送者账号异常
	ErrorCodeReceiverDisabled  = 91007 // 接收者账号异常
	ErrorCodeServiceUnavail    = 91008 // 服务暂时不可用
)

// ValidationErrorMessages 错误码对应的用户友好提示
var ValidationErrorMessages = map[int]string{
	ErrorCodeValidationFailed:  "消息校验失败",
	ErrorCodeNotFriend:         "对方不是您的好友",
	ErrorCodeBlockedByReceiver: "您已被对方拉黑",
	ErrorCodeFriendDeleted:     "好友关系已删除",
	ErrorCodeNotGroupMember:    "您不是该群成员",
	ErrorCodeSenderDisabled:    "发送者账号异常",
	ErrorCodeReceiverDisabled:  "接收者账号异常",
	ErrorCodeServiceUnavail:    "服务暂时不可用，请稍后重试",
}
