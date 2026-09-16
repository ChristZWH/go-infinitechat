// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"go-infinitechat/common/common"
	"go-infinitechat/service/gateway/internal/config"
	"go-infinitechat/service/gateway/internal/handler"
	"go-infinitechat/service/gateway/internal/middleware"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var configFile = flag.String("f", "etc/gateway.yaml", "the config file")

func main() {
	flag.Parse()

	// 1. 加载配置
	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 2. 初始化 Redis（Auth中间件要用）
	rds := redis.MustNewRedis(c.Redis)

	// 3. 初始化 Etcd 客户端配置（可选）
	// 		没配就跑降级路径：只支持 http:// 静态路由
	var etcdCli *clientv3.Client
	if len(c.Etcd.Endpoints) > 0 {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   c.Etcd.Endpoints,
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			common.Fatalf("Etcd 客户端初始化失败：%s", err.Error())
		}
		etcdCli = cli
		defer func() { _ = etcdCli.Close() }()
		common.Infof("Etcd 客户端初始化完成: endpoints=%v", c.Etcd.Endpoints)
	} else {
		common.Warn("Etcd 未配置，仅支持 http:// 静态路由")
	}

	// 4. 初始化中间件
	corsMiddleware := middleware.NewCorsMiddleware(c.Cors)
	authMiddleware := middleware.NewAuthMiddleware(rds, c.Auth)

	// 5. 初始化反向代理路由器
	proxyRouter := handler.NewProxyRouter(c.Routes, c.Timeout, etcdCli)
	defer proxyRouter.Close()

	// 6. 组装 Handler 链：CORS -> Auth -> Proxy
	finalHandler := corsMiddleware.Handle(
		authMiddleware.Handle(
			proxyRouter.ServeHTTP,
		),
	)

	// 7. 启动 HTTP 服务
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)

	common.Infof("========================================")
	common.Infof("  InfiniteChat Gateway 启动")
	common.Infof("  监听地址: %s", addr)
	common.Infof("  白名单路径: %v", c.Auth.WhiteList)
	common.Infof("  路由规则:")
	for _, route := range c.Routes {
		logx.Infof("    %s -> %s %v", route.Id, route.Upstream, route.Prefixes)
	}
	logx.Infof("========================================")

	server := &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(finalHandler),
	}

	if err := server.ListenAndServe(); err != nil {
		common.Errorf("Gateway 启动失败: %s", err.Error())
	}

	// server := rest.MustNewServer(c.RestConf)
	// defer server.Stop()
	// fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	// server.Start()
}
