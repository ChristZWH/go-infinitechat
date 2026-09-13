// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"

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
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 统一错误响应
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

	server := rest.MustNewServer(c.RestConf)
	// 注册中间件
	server.Use(middleware.RecoverMiddleWare)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	// 注册到 etcd（网关服务发现）
	if c.Etcd.RegisterKey != "" {
		host := c.Etcd.PublicIP
		if host == "" {
			host = c.Host
		}
		reg, err := etcdreg.RegisterHTTPService(etcdreg.RegisterOptions{
			Endpoints: c.Etcd.Endpoints,
			Key:       c.Etcd.RegisterKey,                 // 最终写入最终写入 etcd 的完整 key 是：{Key}/{Addr} = /services/offline.api/127.0.0.1:8101
			Addr:      fmt.Sprintf("%s:%d", host, c.Port), //Addr 同时是 key 后缀和 value ；表示本服务对外可访达的地址
			TTL:       30,
		})
		if err != nil {
			common.Errorf("etcd 注册失败: %s", err.Error())
		} else if reg != nil {
			defer reg.Close()
		}
	}

	if len(c.Kafka.Brokers) > 0 {
		go startMessageStoreConsumer(c, ctx)
		go startNotificationStoreConsumer(c, ctx)
	}

	// 启动 Canal 客户端：监听 binlog 写 redis 离线数据
	if ctx.SqlConn != nil {
		canalRunner := canal.NewCanalRunner(c.Canal, ctx.Redis, ctx.SqlConn, ctx.UserRpc)
		// CanalRunner.Start 内部是死循环，要用 go 起协程
		go canalRunner.Start(context.Background())
	}

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}

// 消费 store-topic：把用户消息写入 MySQL（Redis 更新由 Canal 监听 binlog 处理）
func startMessageStoreConsumer(c config.Config, svcCtx *svc.ServiceContext) {
	msgConsumer := consumer.NewMessageStoreConsumer(svcCtx.MessageModel)
	q := kq.MustNewQueue(kq.KqConf{
		Brokers:    c.Kafka.Brokers,
		Group:      c.Kafka.MessageStoreConsumerGroup,
		Topic:      constants.KafkaMessageTopicStore, // 消费store-topic，持久化到MySQL中
		Offset:     "last",
		Consumers:  4,
		Processors: 4,
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
		Brokers:    c.Kafka.Brokers,
		Group:      c.Kafka.NotificationStoreConsumerGroup,
		Topic:      constants.KafkaStoreNotificationTopic, // 消费 store-notification-topic，持久化到MySQL 中
		Offset:     "last",
		Consumers:  3,
		Processors: 3,
	}, notifConsumer)
	defer q.Stop()

	common.Infof("Notification Store Consumer 启动: topic=%s, group=%s", constants.KafkaStoreNotificationTopic, c.Kafka.NotificationStoreConsumerGroup)
	q.Start() // 阻塞
}
