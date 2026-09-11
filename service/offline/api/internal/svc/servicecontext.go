// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"go-infinitechat/service/offline/api/internal/config"
	"go-infinitechat/service/offline/model/message"
	"go-infinitechat/service/offline/model/system_notification"
	"go-infinitechat/service/user/rpc/userrpc"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config                  config.Config
	Redis                   *redis.Redis
	MessageModel            message.MessageModel
	SystemNotificationModel system_notification.SystemNotificationModel
	UserRpc                 userrpc.UserRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.DataSource)

	return &ServiceContext{
		Config:                  c,
		Redis:                   redis.MustNewRedis(c.Redis),
		MessageModel:            message.NewMessageModel(conn, c.Cache),
		SystemNotificationModel: system_notification.NewSystemNotificationModel(conn, c.Cache),
		UserRpc:                 userrpc.NewUserRpc(zrpc.MustNewClient(c.UserRpc)),
	}
}
