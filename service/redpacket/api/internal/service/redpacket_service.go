package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-infinitechat/common/common"
	"go-infinitechat/common/kafka"
	"go-infinitechat/service/redpacket/api/internal/script"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
	"go-infinitechat/service/redpacket/model/red_packet"
	"go-infinitechat/service/redpacket/model/red_packet_receive"
	"go-infinitechat/service/user/model/balance_log"
	"go-infinitechat/service/user/model/user_balance"
	"go-infinitechat/service/user/rpc/userrpc"
	"math/big"
	"strconv"
	"time"

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

// 执行抢红包 Lua 脚本
// 返回（resultCode, completed标识）
func (s *RedPacketService) ExecuteReceiveLua(ctx context.Context, redPacketId, userId int64) (resultCode int64, completed int64, err error) {
	poolKey := constants.GetPoolKey(redPacketId)      // List  [350, 120, 880, ...]   ← 每个元素一个随机金额（单位：分）
	recordKey := constants.GetRecordsKey(redPacketId) // redpacket:records:123 Hash  {888: 350, 999: 120}   ← field=userId, value=抢到的金额
	script := redis.NewScript(script.ReceiveRedPacketLua)

	result, err := s.rds.ScriptRunCtx(ctx, script, []string{poolKey, recordKey}, strconv.FormatInt(userId, 10), strconv.FormatInt(s.getRedPacketExpireMs(ctx, redPacketId), 10))
	common.ThrowIfWithMsg(err != nil, common.SystemError, "Lua脚本执行失败", err)
	resultList, ok := result.([]interface{})
	if !ok || len(resultList) == 0 {
		common.ThrowWithMsg(common.SystemError, "Lua脚本返回值异常")
	}

	resultCode = ToInt64(resultList[0])
	if len(resultList) > 1 {
		completed = ToInt64(resultList[1])
	}
	return resultCode, completed, nil
}

// getRedPacketExpireMs 获取红包过期时间戳（毫秒）：
// 优先取过期 ZSet 的 score（发送红包时写入），缺失时兜底用创建时间 + 24 小时
func (s *RedPacketService) getRedPacketExpireMs(ctx context.Context, redPacketId int64) int64 {
	expireMs, err := s.rds.ZscoreCtx(ctx, constants.ExpireZSet, strconv.FormatInt(redPacketId, 10))
	if err == nil && expireMs > 0 {
		return expireMs
	}
	// ZSet 中不存在（红包已完成/过期被摘除，或发送流程尚未写入），按 DB 创建时间兜底
	rp, err := s.RedPacketModel.FindOne(ctx, redPacketId)
	if err != nil {
		// 红包不存在，传 0 触发 Lua 的参数校验分支返回错误
		return 0
	}
	return rp.CreatedTime.Add(time.Duration(constants.ExpireTimeMs) * time.Millisecond).UnixMilli()
}

func ToInt64(i interface{}) int64 {
	switch val := i.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	case json.Number:
		n, _ := val.Int64()
		return n
	default:
		return 0
	}
}

// 查询用户在某红包中已领取的金额（从 Redis Hash）
func (s *RedPacketService) GetReceivedAmount(redPacket, userId int64) string {
	recordKey := constants.GetRecordsKey(redPacket)
	amountStr, _ := s.rds.Hget(recordKey, strconv.FormatInt(userId, 10))
	if amountStr != "" {
		amountFen, _ := strconv.ParseInt(amountStr, 10, 64)
		return ConvertFenToYuan(amountFen)
	}
	return ""
}
