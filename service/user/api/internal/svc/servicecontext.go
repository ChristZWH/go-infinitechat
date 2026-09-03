// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"go-infinitechat/service/user/api/internal/config"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type ServiceContext struct {
	Config config.Config
	Redis  *redis.Redis
}

func NewServiceContext(c config.Config) *ServiceContext {
	rds := redis.MustNewRedis(c.Redis) // 创建 Redis 实例

	return &ServiceContext{
		Config: c,
		Redis:  rds, // 添加 Redis 实例
	}
}
