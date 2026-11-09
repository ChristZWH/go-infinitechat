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
	"go-infinitechat/common/utils"

	"go-infinitechat/service/redpacket/api/internal/config"
	"go-infinitechat/service/redpacket/api/internal/consumer"
	"go-infinitechat/service/redpacket/api/internal/handler"
	"go-infinitechat/service/redpacket/api/internal/schedular"
	"go-infinitechat/service/redpacket/api/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "etc/redpacket-api.yaml", "the config file")

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
			common.Errorf("未包裹错误：%v", err)
			return http.StatusInternalServerError, utils.Fail(common.SystemError)
		}
		if e.Code >= 50000 {
			// 服务器内部错误，真实 500，网关/监控可识别
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
	server.Use(middleware.IdentityMiddleware) // 解析网关注入的身份（领取/发送要校验"这是我的操作"）
	defer server.Stop()

	// 4. 创建服务上下文（Redis/MySQL/RPC 依赖装配）
	svcCtx := svc.NewServiceContext(c)

	// 5. 注册路由
	handler.RegisterHandlers(server, svcCtx)

	// 6. 启动领取事件发件箱消费者（Redis Stream）：抢包 Lua 原子写入，这里消费入账
	consumer.StartReceiveStreamWorker(svcCtx)

	// 7. 启动事务发件箱投递器（MySQL Outbox → Kafka）：红包消息事件同事务落库，这里异步投递
	consumer.StartOutboxDispatcher(svcCtx)

	// 8. 启动 Kafka 消费者（领完事件 / 过期退款）
	stopFunc := consumer.StartConsumer(svcCtx, c)
	defer func() {
		for _, stopConsumeFunc := range stopFunc {
			stopConsumeFunc()
		}
	}()

	// 注意 defer 的执行顺序（本质是入栈，后注册的先执行）
	// 注册顺序：server.Stop → 停消费者 → KafkaPusherManager.Close → dispatcherCancel
	// 退出顺序：停调度器 → 关 Kafka → 停消费者 → 停 server

	// 9. 关闭 KafkaPusher（生产者连接）
	if svcCtx.KafkaPusherManager != nil {
		defer svcCtx.KafkaPusherManager.Close()
	}

	// 10. 启动过期红包扫描调度器
	dispatcherCtx, dispatcherCancel := context.WithCancel(context.Background())
	defer dispatcherCancel()
	go schedular.StartExpirationDispatcher(dispatcherCtx, svcCtx)

	// 11. 注册到 etcd（网关服务发现）
	if c.Etcd.RegisterKey != "" {
		host := c.Etcd.PublicIP
		if host == "" {
			host = c.Host
		}
		reg, err := etcdreg.RegisterHTTPService(etcdreg.RegisterOptions{
			Endpoints: c.Etcd.Endpoints,
			Key:       c.Etcd.RegisterKey,
			Addr:      fmt.Sprintf("%s:%d", host, c.Port),
		})
		if err != nil {
			common.Errorf("etcd 注册失败：%s", err.Error())
		} else if reg != nil {
			defer reg.Close()
		}
	}

	// 12. 启动 HTTP 服务（阻塞）
	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
