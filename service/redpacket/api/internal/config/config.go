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
	Cache      cache.CacheConf
	Redis      redis.RedisConf
	Kafka      KafkaConf
	UserRpc    zrpc.RpcClientConf
	// ServiceConf service.ServiceConf `json:",optional"`

	// 过期红包扫描调度器
	ExpirationDispatcher ExpirationDispatcherConf `json:",optional"`

	// etcd 服务注册（网关服务发现用）
	Etcd struct {
		Endpoints   []string `json:",optional"`
		RegisterKey string   `json:",optional"` // 注册到 etcd 的 key
		PublicIP    string   `json:",optional"`
	} `json:",optional"`
}

type KafkaConf struct {
	Brokers []string
}

// 过期红包扫描调度器配置
type ExpirationDispatcherConf struct {
	// 每批最多扫描的红包数量，默认 500
	BatchSize int `json:",default=500"`
	// 每轮最多处理的批次数，默认 5
	MaxBatchesPerTick int `json:",default=5"`
	// 单次扫描的最大时间预算（毫秒），默认 400
	TimeBudgetMs int64 `json:",default=400"`
}
