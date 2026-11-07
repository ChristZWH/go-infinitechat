// Package canal 封装 Canal（MySQL binlog 增量订阅）的通用骨架：
// 连接 → 订阅 → 长轮询拉取 → 解析 RowChange → 按 schema.table 分发 → 处理成功后 Ack。
//
// 业务方只需要实现 Handler 并注册到 Runner，不用关心 binlog 协议、连接生命周期和 Ack 时机。
// 典型用法是"事务发件箱（outbox）"：业务把要发的领域事件写进一张 outbox 表，
// Canal 监听这张表，把每一行 INSERT 转成一条消息转发给下游（Kafka/Stream）。
package canal

// Config Canal 连接配置（与具体业务解耦）。
type Config struct {
	Address     string // Canal Server 地址，如 127.0.0.1
	Port        int    // Canal Server 端口，默认 11111
	Destination string // destination，对应 Canal instance
	Username    string
	Password    string
	Filter      string // 订阅过滤表达式，如 "InfiniteChat\\.message"
}

// Handler 处理一张表的一行 binlog 变更（INSERT 场景下的 AfterColumns）。
type Handler interface {
	// Handle 处理 schema.table 的一行 INSERT，columns 是插入后的完整行（列名 → 列值）。
	// 返回 error 表示处理失败：Runner 将不 Ack 该批次，Canal 服务端稍后重投（at-least-once）。
	Handle(schema, table string, columns map[string]string) error
}
