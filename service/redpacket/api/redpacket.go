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
	"go-infinitechat/service/redpacket/api/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "etc/redpacket-api.yaml", "the config file")

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
			common.Errorf("未包裹错误：%v", err)
			return http.StatusInternalServerError, utils.Fail(common.SystemError)
		}
		if e.Code > 50000 {
			// 服务器内部错误，真是500，网关/检控可识别
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

	// 启动 Kafka 消费者
	stopFunc := consumer.StartConsumer(ctx, c)
	defer func() {
		for _, stopConsumeFunc := range stopFunc {
			stopConsumeFunc()
		}
	}()

	// 注意 defer 的注册顺序（本质是入栈）
	// 先注册 KafkaPusherManager.Close()（最后执行）
	// 再注册 dispatcherCancel()（先执行）
	// 这样关闭顺序是：停调度器 → 停消费者 → 关 Kafka → 停 server
	//
	// 关闭 KafkaRusher（后关）
	if ctx.KafkaPusherManager != nil {
		defer ctx.KafkaPusherManager.Close()
	}

	// 启动过期红包扫描调度器（先停）

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

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
