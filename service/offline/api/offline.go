// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"

	commoncanal "go-infinitechat/common/canal"
	"go-infinitechat/common/common"
	etcdreg "go-infinitechat/common/etcd"
	"go-infinitechat/common/middleware"
	"go-infinitechat/common/model/constants"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/offline/api/internal/canal"
	"go-infinitechat/service/offline/api/internal/config"
	"go-infinitechat/service/offline/api/internal/consumer"
	"go-infinitechat/service/offline/api/internal/handler"
	"go-infinitechat/service/offline/api/internal/svc"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "etc/offline-api.yaml", "the config file")

func main() {
	// 1. 加载配置
	flag.Parse()
	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 2. 注册全局响应处理器（错误/成功统一成 {code, message} 格式）
	httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
		var e common.ErrorCode
		if !errors.As(err, &e) {
			// 未知错误：按服务器内部故障处理，对外脱敏
			// 走到这里说明有代码违反了契约（logic 直接返回了裸错误），记日志方便排查
			common.Errorf("未包装错误: %v", err)
			return http.StatusInternalServerError, utils.Fail(common.SystemError)
		}
		if e.Code >= 50000 {
			// 服务器内部错误：真实 500，网关/监控可识别
			return http.StatusInternalServerError, utils.Fail(e)
		}
		// 业务错误：200 + 业务码
		return http.StatusOK, utils.Fail(e)
	})

	// 统一成功响应
	httpx.SetOkHandler(func(ctx context.Context, data any) any {
		return utils.Success(data)
	})

	// 3. 创建 REST server 并注册中间件
	server := rest.MustNewServer(c.RestConf)
	server.Use(middleware.RecoverMiddleWare)  // 兜住 Throw 的 panic 转成统一响应
	server.Use(middleware.IdentityMiddleware) // 解析网关注入的身份（离线/历史消息会话归属校验）
	defer server.Stop()

	// 4. 创建服务上下文（Redis/MySQL/RPC 依赖装配）
	ctx := svc.NewServiceContext(c)

	// 5. 注册路由
	handler.RegisterHandlers(server, ctx)

	// 6. 注册到 etcd（网关服务发现）
	if c.Etcd.RegisterKey != "" {
		host := c.Etcd.PublicIP
		if host == "" {
			host = c.Host
		}
		// 使用公共封装，内部自动进行续约
		reg, err := etcdreg.RegisterHTTPService(etcdreg.RegisterOptions{
			Endpoints: c.Etcd.Endpoints,
			Key:       c.Etcd.RegisterKey,                 // 最终写入 etcd 的完整 key 是：{Key}/{Addr} = /services/offline.api/127.0.0.1:8101
			Addr:      fmt.Sprintf("%s:%d", host, c.Port), // Addr 同时是 key 后缀和 value ；表示本服务对外可访达的地址
			TTL:       30,
		})
		if err != nil {
			common.Errorf("etcd 注册失败: %s", err.Error())
		} else if reg != nil {
			defer reg.Close()
		}
	}

	// 7. 启动 Kafka 消费者（store-topic / store-notification-topic 落库）
	if len(c.Kafka.Brokers) > 0 {
		go startMessageStoreConsumer(c, ctx)
		go startNotificationStoreConsumer(c, ctx)
	}

	// 8. 启动 Canal 客户端（监听 binlog 写 Redis 离线数据）
	startCanalClient(c, ctx)

	// 9. 启动 HTTP 服务（阻塞）
	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}

// startCanalClient 启动 Canal binlog 订阅，把 message 表的落库变更同步到 Redis 离线数据。
//
// 步骤：
//  1. 创建 Runner（canal-go 客户端，配置目标库与过滤规则）
//  2. 注册 infiniteChat.message 表的处理器（消息落库 → 同步 Redis 四件套）
//  3. Runner.Start 内部是死循环，用 goroutine 异步启动
func startCanalClient(c config.Config, svcCtx *svc.ServiceContext) {
	// SqlConn 为 nil 说明 MySQL 数据源未配置，跳过（本地开发可能不连库）
	if svcCtx.SqlConn == nil {
		return
	}

	runner := commoncanal.NewRunner(commoncanal.Config{
		Address:     c.Canal.Host,
		Port:        c.Canal.Port,
		Destination: c.Canal.Destination,
		Username:    c.Canal.Username,
		Password:    c.Canal.Password,
		Filter:      c.Canal.Filter,
	})

	// 注册 infiniteChat.message 表的处理器：消息落库 → 同步 Redis 四件套
	// 注意：库名是 IM.sql 里的 `infiniteChat`（小写 i），不是原代码硬编码的 InfiniteChat——
	// schema 名与 binlog Header 完全一致才能匹配上（原 monitorTables 写错导致一直不生效）。
	runner.RegisterHandler("infiniteChat", "message",
		canal.NewMessageHandler(svcCtx.Redis, svcCtx.SqlConn, svcCtx.UserRpc))

	// Runner.Start 内部是死循环，要用 go 起协程
	go runner.Start(context.Background())
}

// 消费 store-topic：把用户消息写入 MySQL（Redis 更新由 Canal 监听 binlog 处理）
func startMessageStoreConsumer(c config.Config, svcCtx *svc.ServiceContext) {
	msgConsumer := consumer.NewMessageStoreConsumer(svcCtx.MessageModel)
	q := kq.MustNewQueue(kq.KqConf{
		Brokers: c.Kafka.Brokers,
		Group:   c.Kafka.MessageStoreConsumerGroup,
		Topic:   constants.KafkaMessageTopicStore, // 消费store-topic，持久化到MySQL中
		Offset:  "last",
		// 关键：kq 的 ForceCommit 默认为 true，即 handler 返回 error 也会提交 offset。
		// 显式关掉，落库失败时至少不会立刻把位点推过去（配合 consumer 内的有限重试）。
		ForceCommit: false,
		Consumers:   4,
		Processors:  4,
	}, msgConsumer)
	defer q.Stop()

	common.Infof("Message Store Consumer 启动: topic=%s, group=%s", constants.KafkaMessageTopicStore, c.Kafka.MessageStoreConsumerGroup)
	q.Start() // 阻塞
}

// 消费 store-notification-topic：把离线系统通知写入 MySQL
func startNotificationStoreConsumer(c config.Config, svcCtx *svc.ServiceContext) {
	// kq.MustNewQueue 第二个参数是同一个 handler 实例——不能用 NewMessageStoreConsumer(...) 每次建新的
	notifConsumer := consumer.NewNotificationStoreConsumer(svcCtx.SystemNotificationModel)
	q := kq.MustNewQueue(kq.KqConf{
		Brokers: c.Kafka.Brokers,
		Group:   c.Kafka.NotificationStoreConsumerGroup,
		Topic:   constants.KafkaStoreNotificationTopic, // 消费 store-notification-topic，持久化到MySQL 中
		Offset:  "last",
		// 同 MessageStoreConsumer：关掉 ForceCommit，避免写库失败也推进位点
		ForceCommit: false,
		Consumers:   3,
		Processors:  3,
	}, notifConsumer)
	defer q.Stop()

	common.Infof("Notification Store Consumer 启动: topic=%s, group=%s", constants.KafkaStoreNotificationTopic, c.Kafka.NotificationStoreConsumerGroup)
	q.Start() // 阻塞
}
