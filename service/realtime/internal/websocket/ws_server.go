package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"go-infinitechat/common/common"
	"go-infinitechat/common/model/constants"
	"go-infinitechat/common/model/dto"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/realtime/internal/channelmgr"
	rtc "go-infinitechat/service/realtime/internal/constants"
	"go-infinitechat/service/realtime/internal/svc"
	"go-infinitechat/service/user/rpc/user"
	"net/http"
	"strconv"
	"time"

	ws "github.com/gorilla/websocket"
)

const (
	// readTimeout 读超时时间
	// 如果 60 秒内没收到客户端任何消息（包括心跳ping），判定连接超时并断开
	readTimeout = 60 * time.Second

	// maxMessageSize 单条消息最大字节数 (64KB)
	// 超过此大小的消息会被 gorilla/websocket 自动拒绝
	maxMessageSize = 65536
)

// upgrader 将普通 HTTP 请求升级为 WebSocket 连接的升级器
var upgrader = ws.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// HandshakeTimeout 握手超时时间
	// 超过 10 秒没完成 WebSocket 握手就断开，防止慢连接占住资源
	HandshakeTimeout: 10 * time.Second,
	// CheckOrigin 控制是否允许跨域的 WebSocket 连接
	// 生产环境应该为白名单校验，只允许自己的前端域名
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// 启动 WebSocket 服务器
// 在独立端口（默认9101）监听 WebSocket 连接请求，
// 与 REST API 服务（8102）是两个独立的 HTTP 服务
func StartWebSocket(svcCtx *svc.ServiceContext) {
	// 确定 WebSocket 路径，默认 /ws/chat
	wsPath := svcCtx.Config.WebSocket.Path
	if wsPath == "" {
		wsPath = constants.WsServiceUri
	}

	// 注册 WebSocket 路由
	mux := http.NewServeMux()
	mux.HandleFunc(wsPath, func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, svcCtx)
	})

	// 启动监听端口号 :9101
	addr := fmt.Sprintf(":%d", svcCtx.Config.WebSocket.Port)

	if err := http.ListenAndServe(addr, mux); err != nil {
		common.Fatalf("WebSocket 服务启动失败: %s", err.Error())
	}
}

// 处理单个 WebSocket 连接的完整生命周期
//
//	一个连接从建立到断开的全流程：
//	  1. 认证：从 HTTP Header 或 URL 参数中提取 UserId
//	  2. 升级：将 HTTP 连接升级为 WebSocket 长连接
//	  3. 绑定：将 userId ↔ conn 写入 ChannelManager
//	  4. 注册在线状态：在 Redis 的 wsServerUri Hash 中记录 userId → 节点地址
//	     → 用于多节点部署时路由消息到正确的节点
//	  5. 消息循环：不断读取客户端发来的消息
//	     - 收到 "ping" → 回复 "pong"（心跳保活）
//	     - 收到其他文本 → 当作业务消息，校验后转发到 Kafka
//	  6. 清理：连接断开后，移除 ChannelManager 映射 + 删除 Redis 在线状态
func handleWebSocket(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext) {
	// 1. 认证
	// 当前简化实现：直接从 Header 取 UserId，不做 JWT校验
	// 后续可以在这里添加 JWT 校验逻辑
	userId := r.Header.Get("userId")
	if userId == "" {
		// 兼容 URL 参数，同时支持 UserId 和 userId 两种写法
		userId = r.URL.Query().Get("UserId")
		if userId == "" {
			userId = r.URL.Query().Get("userId")
		}
	}
	if userId == "" {
		http.Error(w, "缺少 UserId", http.StatusUnauthorized)
		return
	}

	// 2. HTTP -> WebSocket 升级
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		common.Errorf("WebSocket 升级失败, userId=%s, err=%s", userId, err.Error())
		return
	}
	common.Infof("WebSocket 连接建立: userId=%s, remoteAddr=%s", userId, r.RemoteAddr)

	// 包装连接：带上写锁，保证并发写安全
	// 心跳/错误响应（读循环协程）和消息推送（Kafka Consumer 协程）
	// 共享同一个 ClientConn 实例的同一把锁
	clientConn := channelmgr.NewClientConn(conn)

	// 3. 绑定到 ChannelManager
	svcCtx.ChannelManager.AddUserConn(userId, clientConn)

	// 4. Redis 注册在线状态
	// 在 Redis Hash "wsServerUri" 中写入: userId → "ip:port"
	// 其他服务可以通过 HGET wsServerUri {userId} 知道该用户连在哪个 WS 节点上
	wsAddr := fmt.Sprintf("%s:%d", svcCtx.Config.Etcd.PublicIP, svcCtx.Config.WebSocket.Port)
	if err := svcCtx.Redis.Hset(rtc.RedisWsServerUri, userId, wsAddr); err != nil {
		common.Errorf("Redis 写入在线状态失败, userId=%s, err=%s", userId, err.Error())
	}

	// 5. 注册清理函数
	defer clearConnection(clientConn, svcCtx)

	// 6. 配置连接参数
	// 设置最大消息大小限制
	clientConn.SetReadLimit(maxMessageSize)
	// 设置首次读超时
	clientConn.SetReadDeadline(time.Now().Add(readTimeout))
	// 收到 WebSocket 协议层的 Pong 帧时，重置读超时
	// （注意：这里的 WebSocket 协议层的 Pong，不是我们应用层的 "pong" 文本）
	// 这里协议的 pong 并没有使用上
	clientConn.SetPongHandler(func(appData string) error {
		clientConn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})

	// 7.消息读取循环
	for {
		_, msgBytes, err := clientConn.ReadMessage()
		if err != nil {
			// 区分正常关闭和异常断开，方便排查问题
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseNormalClosure) {
				common.Warnf("WebSocket 异常断开: userId=%s, err=%s", userId, err.Error())
			} else {
				common.Infof("WebSocket 连接关闭: userId=%s", userId)
			}
			break // 退出循环，触发 defer clearConnection
		}

		// 每次收到消息都重置读超时（证明连接还活着）
		clientConn.SetReadDeadline(time.Now().Add(readTimeout))

		msgText := string(msgBytes)

		// 心跳处理 —— 客户端发送的是心跳而非消息
		// 客户端定期发 "ping" 文本，服务端回 "pong" 文本
		if msgText == rtc.HeartbeatPing {
			if err := clientConn.WriteMessage(ws.TextMessage, []byte(rtc.HeartbeatPong)); err != nil {
				common.Errorf("心跳 pong 发送失败: userId=%s, err=%s", userId, err.Error())
				break
			}
			continue
		}

		// 业务处理 —— 客户端发送的是消息而非心跳
		handleBusinessMessage(clientConn, svcCtx, msgText, userId)
	}
}

func handleBusinessMessage(conn *channelmgr.ClientConn, svcCtx *svc.ServiceContext, msgText string, userId string) {
	// 1. JSON -> MessageRequest
	var msgReq dto.MessageRequest
	if err := json.Unmarshal([]byte(msgText), &msgReq); err != nil {
		common.Errorf("消息解析失败: err=%s", err.Error())
		sendErrorToClient(conn, msgReq.ClientMessageId, rtc.ErrorCodeParamsError, "消息格式错误")
		return
	}

	// 2. 身份校验
	// 防止身份冒充：因为发送方 UID 由发送方自己填写在 msgReq 中，
	// 必须保证消息的 senderId 与这条连接认证的 userId 一致，
	// 否则用户 B 可以冒充用户 A 给任何人发消息
	uid, err := strconv.ParseInt(userId, 10, 64)
	if err != nil || msgReq.SenderId != uid {
		common.Warnf("身份校验失败: 连接userId=%s, 消息senderId=%d", userId, msgReq.SenderId)
		sendErrorToClient(conn, msgReq.ClientMessageId, rtc.ErrorCodeParamsError, "身份校验失败")
		return
	}

	common.Infof("收到消息: senderId=%d, sessionId=%d, sessionType=%d, content=%s", msgReq.SenderId, msgReq.SessionId, msgReq.SessionType, msgText)

	// 3. 服务端补充字段
	msgReq.MessageId = utils.NextInt()
	now := time.Now()
	msgReq.CreatedTime = &now

	// 4.校验基本参数 （单聊和群聊的校验）
	if errCode, errMsg := checkMessage(&msgReq); errCode != 0 {
		common.Warnf("消息参数校验失败: errorCode=%d, clientMessageId=%s", errCode, msgReq.ClientMessageId)
		sendErrorToClient(conn, msgReq.ClientMessageId, errCode, errMsg)
		return
	}

	// 5. 消息发送权限校验
	if validateResult := validateMessage(svcCtx, &msgReq); validateResult != nil && !validateResult.Allowed {
		errMsg := rtc.ValidationErrorMessages[int(validateResult.ErrorCode)]
		if errMsg == "" {
			errMsg = validateResult.RejectReason
		}
		common.Warnf("消息发送被拒绝: senderId=%d, sessionId=%d, errorCode=%d, reason=%s",
			msgReq.SenderId, msgReq.SessionId, validateResult.ErrorCode, validateResult.RejectReason)
		sendErrorToClient(conn, msgReq.ClientMessageId, int(validateResult.ErrorCode), errMsg)
		return
	}
	// 校验结束

	// 6. 序列化发 Kafka
	msgJSON, err := json.Marshal(msgReq)
	if err != nil {
		common.Errorf("消息序列化失败: %s", err.Error())
		sendErrorToClient(conn, msgReq.ClientMessageId, rtc.ErrorCodeParamsError, "系统错误，请稍后重试")
		return
	}
	msgStr := string(msgJSON)

	// Kafka 发送失败必须通知客户端，否则客户端永远不知道消息丢了
	// 用带超时的 context，避免 Kafka 不可用时长时间阻塞读循环协程
	pushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if svcCtx.StorePusher != nil {
		if err := svcCtx.StorePusher.Push(pushCtx, msgStr); err != nil {
			common.Errorf("Kafka store-topic 发送失败: messageId=%d, err=%s", msgReq.MessageId, err.Error())
			sendErrorToClient(conn, msgReq.ClientMessageId, rtc.ErrorCodeServiceUnavail, "消息发送失败，请稍后重试")
			return
		}
	}
	if svcCtx.MessagePusher != nil {
		sessionKey := strconv.FormatInt(msgReq.SessionId, 10)
		if err := svcCtx.MessagePusher.KPush(pushCtx, sessionKey, msgStr); err != nil {
			common.Errorf("Kafka message-topic 发送失败: messageId=%d, err=%s", msgReq.MessageId, err.Error())
			sendErrorToClient(conn, msgReq.ClientMessageId, rtc.ErrorCodeServiceUnavail, "消息发送失败，请稍后重试")
			return
		}
	}
}

// 消息发送权限校验
//
//   - 单聊：校验用户状态 + 好友关系 + 是否被拉黑
//   - 群聊：校验用户状态 + 群成员资格
//   - 机器人：跳过校验
//
// 严格模式：RPC 调用失败时拒绝发送（宁可误拦不可漏过）
func validateMessage(svcCtx *svc.ServiceContext, msgReq *dto.MessageRequest) *validateResult {
	if svcCtx.UserRpc == nil {
		common.Warn("UserRpc 未初始化，拒绝消息发送")
		return &validateResult{Allowed: false, RejectReason: "SERVICE_UNAVAILABLE", ErrorCode: int32(rtc.ErrorCodeServiceUnavail)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	switch msgReq.SessionType {
	case rtc.SessionTypeGroup:
		resp, err := svcCtx.UserRpc.ValidateGroupMessage(ctx, &user.ValidateGroupMessageReq{
			SenderId:  msgReq.SenderId,
			SessionId: msgReq.SessionId,
		})
		if err != nil {
			common.Errorf("群聊消息校验 RPC 调用失败: err=%s", err.Error())
			return &validateResult{Allowed: false, RejectReason: "SERVICE_UNAVAILABLE", ErrorCode: int32(rtc.ErrorCodeServiceUnavail)}
		}
		return &validateResult{Allowed: resp.Allowed, RejectReason: resp.RejectReason, ErrorCode: resp.ErrorCode}
	case rtc.SessionTypeSignal:
		resp, err := svcCtx.UserRpc.ValidateSingleMessage(ctx, &user.ValidateSingleMessageReq{
			SenderId:   msgReq.SenderId,
			ReceiverId: *msgReq.ReceiverId,
			SessionId:  msgReq.SessionId,
		})
		if err != nil {
			common.Errorf("单聊消息校验 RPC 调用失败: err=%s", err.Error())
			return &validateResult{Allowed: false, RejectReason: "SERVICE_UNAVAILABLE", ErrorCode: int32(rtc.ErrorCodeServiceUnavail)}
		}
		return &validateResult{Allowed: resp.Allowed, RejectReason: resp.RejectReason, ErrorCode: resp.ErrorCode}
	case rtc.SessionTypeRobot:
		// 机器人消息不校验
		return nil
	default:
		// 兜底：checkMessage 已拦截未知会话类型，走到这里说明校验逻辑有遗漏
		return &validateResult{Allowed: false, RejectReason: "未知会话类型", ErrorCode: int32(rtc.ErrorCodeParamsError)} // 40000 参数错误
	}
}

// validateResult 校验结果
type validateResult struct {
	Allowed      bool
	RejectReason string
	ErrorCode    int32
}

// 校验消息的基本参数是否合法
//
//	校验规则：
//	  - 会话类型必须在 0-2 范围内（0-单聊, 1-群聊, 2-机器人）
//	  - 会话ID必须大于 0
//	  - 消息体 body 不能为空
//	  - 消息类型必须在 0-3 范围内（0-文本, 1-图片, 2-表情, 3-红包）
//	  - 单聊（sessionType=0）：必须指定 receiverId（发给谁）
//	  - 群聊（sessionType=1）：不能指定 receiverId（群聊是广播给所有成员的）
//
//	返回值：
//	  errorCode=0 表示校验通过
//	  errorCode>0 表示校验失败，附带错误描述
func checkMessage(msgReq *dto.MessageRequest) (int, string) {
	// 会话类型必须是已知类型
	if msgReq.SessionType < rtc.SessionTypeSignal || msgReq.SessionType > rtc.SessionTypeRobot {
		return rtc.ErrorCodeParamsError, "未知的会话类型"
	}
	// 会话ID必须合法
	if msgReq.SessionId <= 0 {
		return rtc.ErrorCodeParamsError, "会话ID非法"
	}
	// 消息体不能为空
	if msgReq.Body == nil {
		return rtc.ErrorCodeParamsError, "消息内容不能为空"
	}
	// 消息类型必须是已知类型
	if msgReq.Type < dto.MessageTypeText || msgReq.Type > dto.MessageTypeRedPacket {
		return rtc.ErrorCodeParamsError, "不支持的消息类型"
	}
	// 单聊必须指定接收者
	if msgReq.SessionType == rtc.SessionTypeSignal && msgReq.ReceiverId == nil {
		return rtc.ErrorCodeSignalType, "单聊消息必须指定接收者"
	}
	// 群聊不能指定接收者
	if msgReq.SessionType == rtc.SessionTypeGroup && msgReq.ReceiverId != nil {
		return rtc.ErrorCodeGroupType, "群聊消息不需要指定接收者"
	}
	return 0, ""
}

// 通过 WebSocket 向客户端发送错误响应
//
// 客户端收到 type="ERROR" 的消息后，应根据 errorCode 展示对应的错误提示，
// 并可通过 clientMessageId 关联到是哪条消息发送失败了
func sendErrorToClient(conn *channelmgr.ClientConn, clientMessageId string, errorCode int, errorMessage string) {
	resp := dto.MessageErrorResponse{
		Type:            rtc.MessageTypeError,
		ErrorCode:       errorCode,
		ErrorMessage:    errorMessage,
		ClientMessageId: clientMessageId,
		Timestamp:       time.Now().UnixMilli(),
	}
	respJSON, _ := json.Marshal(resp)

	if err := conn.WriteMessage(ws.TextMessage, respJSON); err != nil {
		common.Errorf("错误响应发送失败: errorCode=%d, err=%s", errorCode, err.Error())
	}
}

// 连接断开后的清理工作
//
//	清理步骤：
//	 1. 从 ChannelManager 移除 userId ↔ conn 的双向映射
//	 2. 从 Redis Hash "wsServerUri" 中删除该用户的在线记录
//	 3. 关闭 WebSocket 连接（释放底层 TCP 资源）
//
//	这个函数通过 defer 调用，保证无论连接因为什么原因断开都会执行
func clearConnection(clientConn *channelmgr.ClientConn, svc *svc.ServiceContext) {
	userId := svc.ChannelManager.RemoveByConn(clientConn)

	// 连接已被新连接顶掉（换设备登录）时，RemoveByConn 返回空字符串，
	// 此时映射和 Redis 在线状态都属于新连接，不能动
	if userId == "" {
		common.Infof("连接已清理: 该连接已被新连接替换")
		clientConn.Close()
		return
	}

	// 只删除本节点写入的在线状态：
	// 用户在 A 节点断开后立刻连到 B 节点时，Redis 里已换成 B 节点的地址，
	// 如果无条件 Hdel 会误删 B 节点的在线记录，所以先比对再删
	wsAddr := fmt.Sprintf("%s:%d", svc.Config.Etcd.PublicIP, svc.Config.WebSocket.Port)
	// cur 是 Redis 里当前的值（重连后指向新节点 B），wsAddr 是本节点（旧节点 A）的地址
	if cur, err := svc.Redis.Hget(rtc.RedisWsServerUri, userId); err == nil && cur == wsAddr {
		if _, err := svc.Redis.Hdel(rtc.RedisWsServerUri, userId); err != nil {
			common.Errorf("Redis 移除在线状态失败, userId=%s, err=%s", userId, err.Error())
		}
	}

	common.Infof("连接已清理: userId=%s", userId)
	clientConn.Close()
}

// 向指定在线用户推送消息（供 Kafka Consumer 调用）
// 参数：
//
//	cm      - ChannelManager 实例，用于查找用户的 WebSocket 连接
//	userId  - 目标用户ID（字符串形式）
//	message - 要推送的消息 JSON 字节（已序列化好的 MessageResponse）
//
// 返回值：
//
//	true  - 用户在线且消息发送成功
//	false - 用户不在线（连接不在当前节点上）或发送失败
//
// 注意：如果返回 false 且用户确实离线，Consumer 应考虑将消息存入离线存储
func SendMessageToUser(cm *channelmgr.ChannelManger, userId string, message []byte) bool {
	// 从 ChannelManager 取出的就是读循环持有的同一个 ClientConn 实例，
	// 共享同一把写锁，与心跳/错误响应的写操作天然串行
	conn := cm.GetConnByUserId(userId)
	if conn == nil {
		return false
	}
	if err := conn.WriteMessage(ws.TextMessage, message); err != nil {
		common.Errorf("消息推送失败: userId=%s, err=%s", userId, err.Error())
		return false
	}
	return true
}
