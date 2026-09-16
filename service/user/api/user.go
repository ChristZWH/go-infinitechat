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
	"go-infinitechat/service/user/api/internal/config"
	"go-infinitechat/service/user/api/internal/handler"
	"go-infinitechat/service/user/api/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "etc/user-api.yaml", "the config file")

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
		host := c.Etcd.PublicIp
		if host == "" {
			host = c.Host // fallback 本地开发
		}
		reg, err := etcdreg.RegisterHTTPService(etcdreg.RegisterOptions{
			Endpoints: c.Etcd.Endpoints,
			Key:       c.Etcd.RegisterKey,
			Addr:      fmt.Sprintf("%s:%d", host, c.Port),
		})
		if err != nil {
			common.Errorf("etcd 注册失败: %s", err.Error())
		} else if reg != nil {
			defer reg.Close()
		}
	}

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
