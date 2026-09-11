// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf

	DataSource string
	Redis      redis.RedisConf
	Cache      cache.CacheConf `json:",optional"`
	UserRpc    zrpc.RpcClientConf

	Minio struct {
		Url       string
		AccessKey string
		SecretKey string
	}

	Etcd struct {
		Endpoints   []string
		RegisterKey string `json:",optional"` // 注册到 etcd 的 key（网关服务发现用）
		PublicIP    string `json:",optional"` // 对外宣告的 IP，如果服务在本地就是 127.0.0.1，如果服务在服务器就是服务器的 IP
	}

	Kafka struct {
		Brokers []string
	}

	// Canal  配置
	Canal struct {
		Host        string `json:",default=127.0.0.1"`
		Port        int    `json:",default=11111"`
		Destination string `json:",default=example"`
		Username    string `json:",default=canal"`
		Password    string `json:",default=canal"`
		Filter      string `json:",default=.*\\..*"`
	}
}
