package svc

import (
	"go-infinitechat/service/user/model/friend"
	"go-infinitechat/service/user/model/user"
	"go-infinitechat/service/user/model/user_session"
	"go-infinitechat/service/user/rpc/internal/config"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config config.Config

	UserModel        user.UserModel
	UserSessionModel user_session.UserSessionModel
	FriendModel      friend.FriendModel
	Redis            *redis.Redis
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.DataSource)

	userModel := user.NewUserModel(conn, c.Cache)
	userSessionModel := user_session.NewUserSessionModel(conn, c.Cache)
	friendModel := friend.NewFriendModel(conn, c.Cache)
	redis := redis.MustNewRedis(c.UserRedis)

	return &ServiceContext{
		Config:           c,
		UserModel:        userModel,
		UserSessionModel: userSessionModel,
		FriendModel:      friendModel,
		Redis:            redis,
	}
}
