// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"flag"
	"fmt"

	"go-infinitechat/common/common"
	"go-infinitechat/common/model/constants"
	"go-infinitechat/service/realtime/internal/config"
	"go-infinitechat/service/realtime/internal/consumer"
	"go-infinitechat/service/realtime/internal/svc"
	"go-infinitechat/service/realtime/internal/websocket"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/realtime.yaml", "the config file")

// realTimeService 主入口
//
// 启动四个组件：
//  1. REST API服务（端口8102）—— 健康检查
//  2. WebSocket服务（端口9101）—— 客户端长连接
//  3. Message Consumer —— 消费 message-topoc，推送消息给在线用户
//  4. Notification Consumer —— 消费 system-notification-topic，推送系统消息
func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 初始化服务上下文：Redis、Kafka、Pushers、Etcd 注册
	svcCtx := svc.NewServiceContext(c)

	// 1. REST API 服务（端口8102）
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// 2. WebSocket 服务在独立端口（默认9101）监听，与 REST 服务并行运行
	// StartWebSocket 内部是阻塞的 ListenAndServe，所以放到独立 goroutine
	go websocket.StartWebSocket(svcCtx)

	// 3. 启动 Kafka Consumers
	if len(c.Kafka.Brokers) > 0 {
		// go
	}

	// 打印启动信息
	fmt.Printf("RealTimeService 启动完成\n")
	fmt.Printf("  REST API:       %s:%d/health\n", c.Host, c.Port)
	fmt.Printf("  WebSocket:      %s:%d%s\n", c.Host, c.WebSocket.Port, c.WebSocket.Path)
	fmt.Printf("  Kafka Consumer: message-topic (%s)\n", c.Kafka.MessageConsumerGroup)
	fmt.Printf("  Kafka Consumer: system-notification-topic (%s)\n", c.Kafka.NotificationConsumerGroup)

	server.Start()
}

// 启动消息推送消费者
//
// 使用 go-queue 的 kq.MustNewQueue 创建 Kafka 消费者
// 消费 message-topic，由 MessageConsumer 处理每条消息
//
// kq.KqConf 字段说明：
//
//	Brokers:    Kafka 集群地址
//	Group:      消费者组名（同组内多个实例互相竞争消费，保证一条消息只被一个实例处理）
//	Topic:      消费的 topic 名
//	Offset:     新消费者组首次启动时从哪里开始消费（first=最早, last=最新）
//	Consumers:  消费者并发数
//	Processors: 消息处理并发数
func startMessageConsumer(c config.Config, svcCtx *svc.ServiceContext) {
	// 1.创建我的消费者实例
	msgConsumer := consumer.NewMessageConsumer(svcCtx)

	// 2. 把他交给 go-queue 的队列，同时告诉框架消费那个 topic
	q := kq.MustNewQueue(kq.KqConf{
		Brokers:    c.Kafka.Brokers,                 // Kafka 地址
		Group:      c.Kafka.MessageConsumerGroup,    // 消费者组名
		Topic:      constants.KafkaMessageTopicPush, // "message-topic"
		Offset:     "last",                          // 只消费启动后的新消息
		Consumers:  4,
		Processors: 4,
	}, msgConsumer)

	defer q.Stop()
	common.Infof("Message Consumer 启动: topic=%s, group=%s", constants.KafkaMessageTopicPush, c.Kafka.MessageConsumerGroup)

	// 3. 启动 "框架内死循环开始，此后 Consumer 会被反复调用"
	q.Start() // 阻塞
}

func startNotificationConsumer(c config.Config, svcCtx *svc.ServiceContext) {
	notifConsumer := consumer.NewNotificationConsumer(svcCtx)

	q := kq.MustNewQueue(kq.KqConf{
		Brokers:    c.Kafka.Brokers,
		Group:      c.Kafka.NotificationConsumerGroup,
		Topic:      constants.KafkaSystemNotificationTopic, // "system-notification-topic"
		Offset:     "last",
		Consumers:  3,
		Processors: 3,
	}, notifConsumer)

	defer q.Stop()
	common.Infof("Notification Consumer 启动: topic=%s, group=%s", constants.KafkaSystemNotificationTopic, c.Kafka.NotificationConsumerGroup)
	q.Start() // 阻塞
}
