package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf

	// Mysql 连接字符串
	Database string

	// 	Redis 配置
	// 注意：这里有个坑，在rpc服务是要配置RpcServerConf的，然后我这里的redis和go-zero的缓存都是用的redis，导致启动出现conflict key redis错误
	// 只需要把Reids名字改成UserRedis或者其他名字，只要不叫Redis就行了。这种情况在api服务不会出现，目前仅出现在rpc服务中
	UserRedis redis.RedisConf

	// go-zero model 层缓存配置
	Cache cache.CacheConf
}
