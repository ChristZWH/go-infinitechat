package channelmgr

import (
	"sync"

	"github.com/gorilla/websocket"
)

// ChannelManager 管理所有在线用户的 WebSocket 连接
// 核心功能：维护 userId 与 WebSocket 连接的双向映射，支持：
//   - 根据 userId 找到连接 -> 用于给指定用户推送消息
//   - 根据连接找到userId -> 用于连接断开时清理用户状态
// 为什么需要双向映射？
//   正向（userId -> conn）:Kafka Consumer 消费到消息后，需要根据 receiverId 找到连接去推送
//   反向（conn -> userId）:WebSocket 连接断开时，只有 conn 对象，需要反查 userId 来清理 Redis
// 并发安全：
//   Go 版本 sync.RWMutex + 普通 map，读多写少场景下 RWMutex 性能更好

// 全局单例：整个进程只有一个 ChannelManager 实例，通过 GetManager()获取
type ChannelManger struct {
	mu          sync.RWMutex
	userConnMap map[string]*websocket.Conn // 正向映射：userId -> WebSocket 连接
	connUserMap map[*websocket.Conn]string // 反向映射：WebSocket 连接 -> userId
}

// 包内私有全局单例实例 - 通过GetManager导出
// 连接映射必须被 HTTP handler 和 Kafka同时共享，天然要求进程级唯一
// 包初始化阶段执行，main之前
var manager = &ChannelManger{
	userConnMap: make(map[string]*websocket.Conn),
	connUserMap: map[*websocket.Conn]string{},
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
func (cm *ChannelManger) AddUserConn(userId string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 踢掉旧连接
	if oldConn, ok := cm.userConnMap[userId]; ok {
		cm.userConnMap[userId] = nil
		oldConn.Close()
	}

	// 写入新的双向连接
	cm.userConnMap[userId] = conn
	cm.connUserMap[conn] = userId
}

// GetConnByUserId 根据 userId 查找该用户的 WebSocket 连接
// 返回 nil 表示用户不在线（没有连接在当前节点上）
// 使用场景：Kafka Consumer 消费到消息后，根据接收者 userId 查找连接来推送
func (cm *ChannelManger) GetConnByUserId(userId string) *websocket.Conn {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.userConnMap[userId]
}

// GetUserIdByConn 根据 WebSocket 连接反查 userId
// 返回空字符串表示该连接没有绑定用户（异常情况）
// 使用场景：连接断开时，需要知道是哪个用户断开了，以便清理 Redis 在线状态
func (cm *ChannelManger) GetUserIdByConn(conn *websocket.Conn) string {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.connUserMap[conn]
}

// 用户下线：根据连接移除双向映射
// 返回被移除的 userId（用于后续清理 Redis 在线状态）
// 返回空字符串表示该连接已经被移除过了（幂等安全）
func (cm *ChannelManger) RemoveByConn(conn *websocket.Conn) string {
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
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.userConnMap)
}
