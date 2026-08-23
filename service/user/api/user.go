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
		if errors.As(err, &e) {
			return http.StatusOK, utils.Fail(e)
		}
		return http.StatusOK, utils.FailWithCode(500, err.Error())
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

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
