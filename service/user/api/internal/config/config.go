// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Redis redis.RedisConf
	// go-zero model 层缓存配置
	Cache cache.CacheConf `json:"cache,optional"`

	Minio struct {
		Url       string
		AccessKey string
		SecretKey string
	}

	DataSource string

	Etcd struct {
		Endpoints   []string
		Prefix      string
		RegisterKey string `json:",optional"`
		PublicIp    string `json:",optional"`
	}

	Kafka struct {
		Brokers []string `json:",optional"`
	}
}
