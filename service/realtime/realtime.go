// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"flag"
	"fmt"

	"go-infinitechat/service/realtime/internal/config"
	"go-infinitechat/service/realtime/internal/svc"
	"go-infinitechat/service/realtime/internal/websocket"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/realtime.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)

	// WebSocket 服务在独立端口（默认9101）监听，与 REST 服务并行运行
	// StartWebSocket 内部是阻塞的 ListenAndServe，所以放到独立 goroutine
	go websocket.StartWebSocket(svcCtx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
