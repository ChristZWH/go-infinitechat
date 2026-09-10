package channelmgr

import (
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
)

// ClientConn 包装 gorilla 连接，把所有写操作串行化
//
// 为什么需要写锁？
//
//	gorilla/websocket 规定：一个连接只支持"一个读协程 + 一个写协程"，
//	禁止两个 goroutine 同时调用 WriteMessage（会导致帧交错、连接损坏）。
//	实际场景中写方有多个：
//	  - 读循环协程：心跳 pong、错误响应
//	  - Kafka Consumer 协程：推送聊天消息
//	因此用一把互斥锁把所有写操作串行化。
//
// 注意：必须保证所有人操作同一个 ClientConn 实例（共享同一把锁）。
// 为此 ChannelManager 中存的就是 *ClientConn 而不是 *websocket.Conn，
// 任何写方都从 ChannelManager 取，而不是自己 new 包装。
type ClientConn struct {
	conn *ws.Conn
	mu   sync.Mutex // 写锁：同一时刻只允许一个 goroutine 写
}

// WriteTimeout 写超时时间
// 向客户端发送消息时，如果 10 秒内没发完就判定写超时
const WriteTimeout = 10 * time.Second

// NewClientConn 创建一个带写锁的连接包装
func NewClientConn(conn *ws.Conn) *ClientConn {
	return &ClientConn{conn: conn}
}

// WriteMessage 并发安全的写：加锁 → 设超时 → 写入
// 所有写方（心跳、错误响应、Kafka 推送）都必须走这个方法
func (c *ClientConn) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	return c.conn.WriteMessage(messageType, data)
}

// 以下读侧方法直接透传，不需要加锁：
// 读操作只会在读循环协程中执行（单协程），天然串行。
// gorilla 规定 Close 可以与任何方法并发调用，所以 Close 也不加锁。

// ReadMessage 读取一条消息（只允许读循环协程调用）
func (c *ClientConn) ReadMessage() (int, []byte, error) {
	return c.conn.ReadMessage()
}

// SetReadDeadline 设置读超时（只允许读循环协程调用）
func (c *ClientConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetReadLimit 设置单条消息最大字节数（只允许读循环协程调用）
func (c *ClientConn) SetReadLimit(limit int64) {
	c.conn.SetReadLimit(limit)
}

// SetPongHandler 设置协议层 Pong 帧回调（只允许读循环协程调用）
func (c *ClientConn) SetPongHandler(h func(appData string) error) {
	c.conn.SetPongHandler(h)
}

// Close 关闭连接（允许与任何方法并发调用）
func (c *ClientConn) Close() error {
	return c.conn.Close()
}

// ----------------------------------------------------------
// ----------------------------------------------------------
// ----------------------------------------------------------

// ChannelManager 管理所有在线用户的 WebSocket 连接
// 核心功能：维护 userId 与 WebSocket 连接的双向映射，支持：
//   - 根据 userId 找到连接 -> 用于给指定用户推送消息
//   - 根据连接找到userId -> 用于连接断开时清理用户状态
// 为什么需要双向映射？
//   正向（userId -> conn）:Kafka Consumer 消费到消息后，需要根据 receiverId 找到连接去推送
//   反向（conn -> userId）:WebSocket 连接断开时，只有 conn 对象，需要反查 userId 来清理 Redis
// 并发安全：
//   Go 版本 sync.RWMutex + 普通 map，读多写少场景下 RWMutex 性能更好
// 为什么存 *ClientConn 而不是 *websocket.Conn？
//   推送方和读循环都要写连接，必须共享同一把写锁，
//   所以这里存的是带写锁的包装类型。

// 全局单例：整个进程只有一个 ChannelManager 实例，通过 GetManager()获取
type ChannelManger struct {
	mu          sync.RWMutex
	userConnMap map[string]*ClientConn // 正向映射：userId -> WebSocket 连接
	connUserMap map[*ClientConn]string // 反向映射：WebSocket 连接 -> userId
}

// 包内私有全局单例实例 - 通过GetManager导出
// 连接映射必须被 HTTP handler 和 Kafka同时共享，天然要求进程级唯一
// 包初始化阶段执行，main之前
var manager = &ChannelManger{
	userConnMap: make(map[string]*ClientConn),
	connUserMap: map[*ClientConn]string{},
}

// 返回全局唯一的 ChannelManeger 实例
func GetManager() *ChannelManger {
	return manager
}

// 用户上线：绑定 userId 与 WebSocket 连接
// 如果该用户已有旧的连接（比如换设备登录），会：
//
//	1.从映射中移除旧连接
//	2.关闭旧连接（踢掉旧设备，保证同一用户同时只有一个连接）
//	3.写入新连接
func (cm *ChannelManger) AddUserConn(userId string, conn *ClientConn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 踢掉旧连接
	if oldConn, ok := cm.userConnMap[userId]; ok {
		oldConn.Close()
		// 同时删除旧连接的反向映射，否则旧连接断开后触发 clearConnection，
		// 会误删新连接的 userConnMap 记录（误杀新连接）
		delete(cm.connUserMap, oldConn)
	}

	// 写入新的双向连接
	cm.userConnMap[userId] = conn
	cm.connUserMap[conn] = userId
}

// GetConnByUserId 根据 userId 查找该用户的 WebSocket 连接
// 返回 nil 表示用户不在线（没有连接在当前节点上）
// 使用场景：Kafka Consumer 消费到消息后，根据接收者 userId 查找连接来推送
func (cm *ChannelManger) GetConnByUserId(userId string) *ClientConn {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.userConnMap[userId]
}

// GetUserIdByConn 根据 WebSocket 连接反查 userId
// 返回空字符串表示该连接没有绑定用户（异常情况）
// 使用场景：连接断开时，需要知道是哪个用户断开了，以便清理 Redis 在线状态
func (cm *ChannelManger) GetUserIdByConn(conn *ClientConn) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.connUserMap[conn]
}

// 用户下线：根据连接移除双向映射
// 返回被移除的 userId（用于后续清理 Redis 在线状态）
// 返回空字符串表示该连接已经被移除过了（如被新连接顶掉，幂等安全）
func (cm *ChannelManger) RemoveByConn(conn *ClientConn) string {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	userId, ok := cm.connUserMap[conn]
	if ok {
		delete(cm.connUserMap, conn)
		delete(cm.userConnMap, userId)
	}

	return userId
}

// 根据 userId 移除双向映射（主动踢人场景使用）
func (cm *ChannelManger) RemoveByUserId(userId string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, ok := cm.userConnMap[userId]
	if ok {
		delete(cm.connUserMap, conn)
		delete(cm.userConnMap, userId)
	}
}

// 返回当前节点的在线用户数（用于健康检查接口展示）
func (cm *ChannelManger) OnlineCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.userConnMap)
}
