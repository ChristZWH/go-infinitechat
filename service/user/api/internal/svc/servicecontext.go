// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"go-infinitechat/common/common"
	"go-infinitechat/service/user/api/internal/config"
	"go-infinitechat/service/user/api/internal/service"
	"go-infinitechat/service/user/model/apply_friend"
	"go-infinitechat/service/user/model/balance_log"
	"go-infinitechat/service/user/model/friend"
	"go-infinitechat/service/user/model/session"
	"go-infinitechat/service/user/model/user"
	"go-infinitechat/service/user/model/user_balance"
	"go-infinitechat/service/user/model/user_session"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config config.Config
	Redis  *redis.Redis

	WsServerLocator *service.WsServerLocator
	SqlConn         sqlx.SqlConn

	// 模型
	ApplyFriendModel apply_friend.ApplyFriendModel
	BalanceLogModel  balance_log.BalanceLogModel
	FriendModel      friend.FriendModel
	SessionModel     session.SessionModel
	UserModel        user.UserModel
	UserBalanceModel user_balance.UserBalanceModel
	UserSessionModel user_session.UserSessionModel

	// Kafka 相关

	// service
	UserService *service.UserService
}

func NewServiceContext(c config.Config) *ServiceContext {
	rds := redis.MustNewRedis(c.Redis) // 创建 Redis 实例

	conn := sqlx.NewMysql(c.DataSourse)
	applyFriendModel := apply_friend.NewApplyFriendModel(conn, c.Cache)
	balanceLogModel := balance_log.NewBalanceLogModel(conn, c.Cache)
	friendModel := friend.NewFriendModel(conn, c.Cache)
	sessionModel := session.NewSessionModel(conn, c.Cache)
	userModel := user.NewUserModel(conn, c.Cache)
	userBalanceModel := user_balance.NewUserBalanceModel(conn, c.Cache)
	userSessionModel := user_session.NewUserSessionModel(conn, c.Cache)

	locator, err := service.NewWsServerLocator(c.Etcd.Endpoints, c.Etcd.Prefix)
	if err != nil {
		common.Errorf("etcd 连接失败, wsServerUri将为空: ", err)
		common.ThrowWithMsg(common.SystemError, "NewServiceContext(c config.Config) *ServiceContext : etcd 连接失败", err)
		return nil
	}

	// service
	userService := service.NewUserService(userModel, rds)

	return &ServiceContext{
		Config: c,
		Redis:  rds, // 添加 Redis 实例

		WsServerLocator: locator,
		SqlConn:         conn,

		// 模型
		ApplyFriendModel: applyFriendModel,
		BalanceLogModel:  balanceLogModel,
		FriendModel:      friendModel,
		SessionModel:     sessionModel,
		UserModel:        userModel,
		UserBalanceModel: userBalanceModel,
		UserSessionModel: userSessionModel,

		// service
		UserService: userService,
	}
}
