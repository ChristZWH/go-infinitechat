// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"go-infinitechat/common/kafka"
	"go-infinitechat/service/redpacket/api/internal/config"
	"go-infinitechat/service/redpacket/api/internal/service"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
	"go-infinitechat/service/redpacket/model/red_packet"
	"go-infinitechat/service/redpacket/model/red_packet_receive"
	"go-infinitechat/service/user/model/balance_log"
	"go-infinitechat/service/user/model/user_balance"
	"go-infinitechat/service/user/rpc/userrpc"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config   config.Config
	Redis    *redis.Redis
	SqlxConn sqlx.SqlConn

	// 模型
	RedPacketModel        red_packet.RedPacketModel
	RedPacketReceiveModel red_packet_receive.RedPacketReceiveModel
	UserBalanceModel      user_balance.UserBalanceModel
	BalanceLogModel       balance_log.BalanceLogModel

	// RPC
	UserRPC userrpc.UserRpc

	// kafka
	KafkaPusherManager *kafka.PusherManager

	// Service
	RedPacketService *service.RedPacketService
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.DataSource)
	redis := redis.MustNewRedis(c.Redis)

	redPacketModel := red_packet.NewRedPacketModel(conn, c.Cache)
	redPacketReceiveModel := red_packet_receive.NewRedPacketReceiveModel(conn, c.Cache)
	userBalanceModel := user_balance.NewUserBalanceModel(conn, c.Cache)
	balanceLogModel := balance_log.NewBalanceLogModel(conn, c.Cache)
	userRpc := userrpc.NewUserRpc(zrpc.MustNewClient(c.UserRpc))

	var kafkaPusherManager *kafka.PusherManager
	if len(c.Kafka.Brokers) > 0 {
		kafkaPusherManager = kafka.NewPusherManager(c.Kafka.Brokers, constants.AllRedPacketKafkaTopics())
	}

	redPacketService := service.NewRedPacketService(redPacketModel, redPacketReceiveModel, userBalanceModel, balanceLogModel, redis, conn, kafkaPusherManager, userRpc)

	return &ServiceContext{
		Config:   c,
		Redis:    redis,
		SqlxConn: conn,
		// 模型
		RedPacketModel:        redPacketModel,
		RedPacketReceiveModel: redPacketReceiveModel,
		UserBalanceModel:      userBalanceModel,
		BalanceLogModel:       balanceLogModel,
		// RPC
		UserRPC:            userRpc,
		KafkaPusherManager: kafkaPusherManager,
		RedPacketService:   redPacketService,
	}
}
