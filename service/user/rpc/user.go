package main

import (
	"flag"
	"fmt"

	"go-infinitechat/service/user/rpc/internal/config"
	"go-infinitechat/service/user/rpc/internal/server"
	"go-infinitechat/service/user/rpc/internal/svc"
	"go-infinitechat/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/user.yaml", "the config file")

func main() {
	// 1. 加载配置
	flag.Parse()
	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 2. 创建服务上下文（依赖装配）
	ctx := svc.NewServiceContext(c)

	// 3. 创建 RPC server 并注册服务（zrpc 内部自动完成 etcd 服务注册）
	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		user.RegisterUserRpcServer(grpcServer, server.NewUserRpcServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	// 4. 启动 RPC 服务（阻塞）
	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
