package service

import (
	"errors"
	"fmt"
	"go-infinitechat/common/kafka"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
	"go-infinitechat/service/redpacket/model/red_packet"
	"go-infinitechat/service/redpacket/model/red_packet_receive"
	"go-infinitechat/service/user/model/balance_log"
	"go-infinitechat/service/user/model/user_balance"
	"go-infinitechat/service/user/rpc/userrpc"
	"math/big"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type RedPacketService struct {
	RedPacketModel        red_packet.RedPacketModel
	RedPacketReceiveModel red_packet_receive.RedPacketReceiveModel
	UserBalanceModel      user_balance.UserBalanceModel
	BalanceLogModel       balance_log.BalanceLogModel
	rds                   *redis.Redis
	SqlConn               sqlx.SqlConn
	KafkaPusherManager    *kafka.PusherManager
	UserRpc               userrpc.UserRpc
}

func NewRedPacketService(
	redPacketModel red_packet.RedPacketModel,
	redPacketReceiveModel red_packet_receive.RedPacketReceiveModel,
	userBalanceModel user_balance.UserBalanceModel,
	balanceLogModel balance_log.BalanceLogModel,
	rds *redis.Redis,
	conn sqlx.SqlConn,
	kafkaPusherManager *kafka.PusherManager,
	userRpc userrpc.UserRpc,
) *RedPacketService {
	return &RedPacketService{
		RedPacketModel:        redPacketModel,
		RedPacketReceiveModel: redPacketReceiveModel,
		UserBalanceModel:      userBalanceModel,
		BalanceLogModel:       balanceLogModel,
		rds:                   rds,
		SqlConn:               conn,
		KafkaPusherManager:    kafkaPusherManager,
		UserRpc:               userRpc,
	}
}

// 元转分（十进制精确解析，浮点截断会少算 1 分）
func YuanToFen(yuanStr string) (int64, error) {
	r, ok := new(big.Rat).SetString(yuanStr)
	if !ok {
		return 0, errors.New("金额格式错误")
	}
	r = r.Mul(r, big.NewRat(int64(constants.YuanToFenMultiplier), 1))
	if !r.IsInt() {
		return 0, errors.New("金额最多两位小数")
	}
	return r.Num().Int64(), nil
}

// 分转元
func ConvertFenToYuan(fen int64) string {
	return fmt.Sprintf("%.2f", float64(fen)/float64(constants.YuanToFenMultiplier))
}
