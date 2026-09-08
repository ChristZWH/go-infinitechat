// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"go-infinitechat/common/common"
	"go-infinitechat/common/kafka"
	"go-infinitechat/service/user/api/internal/config"
	"go-infinitechat/service/user/api/internal/service"
	constants2 "go-infinitechat/service/user/api/internal/types/constants"
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
	KafkaPusherManager  *kafka.PusherManager
	NotificationService *service.NotificationService

	// service
	UserService        *service.UserService
	UserBalanceService *service.BalanceService
	SessionService     *service.SessionService
	UserSessionService *service.UserSessionService
	FriendService      *service.FriendService
	ApplyFriendServer  *service.ApplyFriendService
	GroupService       *service.GroupService
}

func NewServiceContext(c config.Config) *ServiceContext {
	rds := redis.MustNewRedis(c.Redis) // 创建 Redis 实例

	// model
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

	// 注册 Kafka
	brokers := c.Kafka.Brokers
	topics := constants2.AllKafkaTopics()
	pusherManager := kafka.NewPusherManager(brokers, topics)

	// service
	userService := service.NewUserService(userModel, rds)
	userBalanceService := service.NewBalanceService(userBalanceModel, rds)
	sessionService := service.NewSessionService(sessionModel, rds)
	userSessionService := service.NewUserSessionService(userSessionModel, rds, conn)
	notificationService := service.NewNotificationService(pusherManager)
	friendService := service.NewFriendService(userService, sessionService, userSessionService, notificationService, friendModel, applyFriendModel, conn, rds)
	applyFriendServer := service.NewApplyFriendService(applyFriendModel, friendModel, friendService, userService, notificationService, rds, conn)
	groupService := service.NewGroupService(userService, friendModel, sessionModel, userSessionModel, notificationService, conn)

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

		// Kafka
		KafkaPusherManager:  pusherManager,
		NotificationService: notificationService,

		// service
		UserService:        userService,
		UserBalanceService: userBalanceService,
		SessionService:     sessionService,
		UserSessionService: userSessionService,
		FriendService:      friendService,
		ApplyFriendServer:  applyFriendServer,
		GroupService:       groupService,
	}
}
