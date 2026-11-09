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
	server.Use(middleware.IdentityMiddleware) // 解析网关注入的 X-User-Id，供 logic 读取
	defer server.Stop()

	// 4. 创建服务上下文（Redis/MySQL/RPC 依赖装配）
	ctx := svc.NewServiceContext(c)

	// 5. 注册路由
	handler.RegisterHandlers(server, ctx)

	// 6. 注册到 etcd（网关服务发现）
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

	// 7. 启动 HTTP 服务（阻塞）
	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
