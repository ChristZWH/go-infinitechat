package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go-infinitechat/common/common"
	"go-infinitechat/common/kafka"
	"go-infinitechat/common/model/txctx"
	"go-infinitechat/common/utils"
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

	"github.com/zeromicro/go-zero/core/logx"
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

// 分转元：用整数运算格式化，避免 float64 在大额下丢精度
func ConvertFenToYuan(fen int64) string {
	if fen < 0 {
		return fmt.Sprintf("-%d.%02d", -fen/100, -fen%100)
	}
	return fmt.Sprintf("%d.%02d", fen/100, fen%100)
}

// 执行抢红包 Lua 脚本
// 返回（resultCode, completed标识）
func (s *RedPacketService) ExecuteReceiveLua(ctx context.Context, redPacketId, userId int64) (resultCode int64, completed int64) {
	poolKey := constants.GetPoolKey(redPacketId)       // List  [350, 120, 880, ...]   ← 每个元素一个随机金额（单位：分）
	recordKey := constants.GetRecordsKey(redPacketId)  // redpacket:records:123 Hash  {888: 350, 999: 120}   ← field=userId, value=抢到的金额
	streamKey := constants.ReceiveStreamKey            // 领取事件发件箱（Redis Stream）
	remainKey := constants.GetRemainKey(redPacketId)   // 剩余金额（DECRBY 维护）
	pendingKey := constants.GetPendingKey(redPacketId) // 已 LPOP 未入账计数（INCR/DECR 维护）
	script := redis.NewScript(script.ReceiveRedPacketLua)

	result, err := s.rds.ScriptRunCtx(ctx, script,
		[]string{poolKey, recordKey, streamKey, remainKey, pendingKey},
		strconv.FormatInt(userId, 10),
		strconv.FormatInt(s.getRedPacketExpireMs(ctx, redPacketId), 10),
		strconv.FormatInt(redPacketId, 10),
		strconv.FormatInt(time.Now().UnixMilli(), 10),
	)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "Lua脚本执行失败", err)
	resultList, ok := result.([]interface{})
	if !ok || len(resultList) == 0 {
		common.ThrowWithMsg(common.SystemError, "Lua抢红包脚本返回值异常")
	}

	// 返回值没有异常，说明抢到红包；第一个参数表示抢到的金额，第二个参数表示是否还有剩余
	resultCode = ToInt64(resultList[0])
	if len(resultList) > 1 {
		completed = ToInt64(resultList[1])
	}
	return resultCode, completed
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

type UserInfoItem struct {
	NickName string
	Avatar   string
}

// 批量获取用户信息
func (s *RedPacketService) BatchGetUserInfos(ctx context.Context, userIds []int64) map[int64]UserInfoItem {
	result := make(map[int64]UserInfoItem)
	if s.UserRpc == nil {
		return map[int64]UserInfoItem{}
	}
	req := &userrpc.BatchGetUserInfosReq{UserIds: userIds}
	resp, err := s.UserRpc.BatchGetUserInfos(ctx, req)
	if err != nil {
		logx.Errorf("批量获取用户信息失败: %v", err)
		return result
	}
	if resp != nil && resp.Users != nil {
		for _, item := range resp.Users {
			result[item.UserId] = UserInfoItem{NickName: item.Nickname, Avatar: item.Avatar}
		}
	}
	return result
}

// 增加用户余额
func (s *RedPacketService) AddBalance(ctx context.Context, userId, amount int64) error {
	return s.UserBalanceModel.AddBalanceTx(ctx, userId, amount)
}

// 记录余额变动日志
func (s *RedPacketService) InsertBalanceLog(ctx context.Context, userId, amount int64, logType int, related int64) {
	now := time.Now()
	_, err := s.BalanceLogModel.InsertTx(ctx, &balance_log.BalanceLog{
		BalanceLogId: utils.NextInt(),
		UserId:       userId,
		Amount:       amount,
		Type:         int64(logType),
		RelatedId:    sql.NullInt64{Int64: related, Valid: true},
		CreatedTime:  now,
		UpdatedTime:  now,
	})
	common.ThrowIfWithMsg(err != nil, common.MysqlError, "记录余额变动日志失败", err)
}

// SettleReceive 领取入账：幂等检查 + 事务（插领取记录 + 加余额 + 写余额流水）+ 恰好一次 DECR pending。
// 供领取事件发件箱（Redis Stream worker）复用。at-least-once：重投时靠幂等检查跳过 insert；
// DECR 靠「每事件一次性标志」（SETNX）保证恰好一次，重投不会重复 DECR。
//
// eventID 是 stream 消息 ID，用于每事件一次性标志（settled:<eventID>）。
func (s *RedPacketService) SettleReceive(ctx context.Context, userId, redPacketId, receivedAmount, receivedTime int64, eventID string) error {
	// 幂等：同一红包同一用户已有记录则跳过 insert（Stream at-least-once 会重投）。
	// 但 DECR 仍需执行——由 SETNX 保证恰好一次（重投时 SETNX 已存在则跳过）。
	existing, err := s.RedPacketReceiveModel.FindOneByRedPacketIdReceiverId(ctx, redPacketId, userId)
	if err == nil && existing != nil {
		return s.decrPendingOnce(ctx, redPacketId, eventID)
	}

	now := time.Now()
	if err := txctx.WithTransaction(ctx, s.SqlConn, func(ctx context.Context) error {
		_, err := s.RedPacketReceiveModel.InsertTx(ctx, &red_packet_receive.RedPacketReceive{
			RedPacketReceiveId: utils.NextInt(),
			RedPacketId:        redPacketId,
			ReceiverId:         userId,
			Amount:             receivedAmount,
			ReceivedAt:         time.UnixMilli(receivedTime),
			CreatedTime:        now,
			UpdatedTime:        now,
		})
		common.ThrowIfWithMsg(err != nil, common.MysqlError, "插入领取记录失败", err)

		if err := s.AddBalance(ctx, userId, receivedAmount); err != nil {
			common.ThrowIfWithMsg(true, common.OperationError, "增加余额失败", err)
		}

		s.InsertBalanceLog(ctx, userId, receivedAmount, constants.BalanceLogTypeReceive, redPacketId)
		return nil
	}); err != nil {
		return err
	}

	// 入账成功 → 恰好一次 DECR 未入账计数。
	// 失败返回 error → worker 不 XACK → 重投后 SETNX 已存在，不再重复 DECR。
	return s.decrPendingOnce(ctx, redPacketId, eventID)
}

// decrPendingOnce 未入账计数 -1（抢包 Lua INCR，这里 DECR）。
// 用原子 Lua + SETNX「每事件一次性标志」保证恰好一次 DECR：重投/重试时 SETNX 已存在则跳过，
// 消除「事务提交后 DECR 失败 → pending 永远 >0 → 退款永久冻结」的窗口。
func (s *RedPacketService) decrPendingOnce(ctx context.Context, redPacketId int64, eventID string) error {
	settledKey := constants.GetSettledKey(eventID)
	pendingKey := constants.GetPendingKey(redPacketId)
	ttlSeconds := int64(constants.RedisCacheExpireHours * 3600)

	lua := redis.NewScript(script.DecrPendingOnceLua)
	if _, err := s.rds.ScriptRunCtx(ctx, lua, []string{settledKey, pendingKey}, strconv.FormatInt(ttlSeconds, 10)); err != nil {
		logx.Errorf("DECR 未入账计数失败, redPacketId=%d, eventID=%s: %v", redPacketId, eventID, err)
		return err
	}
	return nil
}

// 清理红包 Redis 缓存（领取完成 / 过期退款后调用）
func (s *RedPacketService) CleanRedisCache(redPacketId int64) {
	_, _ = s.rds.Del(
		constants.GetPoolKey(redPacketId),
		constants.GetRecordsKey(redPacketId),
		constants.GetRemainKey(redPacketId),
		constants.GetPendingKey(redPacketId),
	)
	_, _ = s.rds.Zrem(constants.ExpireZSet, strconv.FormatInt(redPacketId, 10))
}

// CalculateRemainAmount 计算红包剩余金额（用于过期退款）。
//
// 优先读 remainKey（LPOP 时 DECRBY 维护，O(1)）；remainKey 不存在（TTL 过期 / 从未建池）时
// 用 DB 差额兜底：TotalAmount − SUM(领取记录金额)。否则 Redis 过期会返回 0，导致退款归零。
func (s *RedPacketService) CalculateRemainAmount(ctx context.Context, redPacketId int64) (int64, error) {
	remainKey := constants.GetRemainKey(redPacketId)

	exists, err := s.rds.ExistsCtx(ctx, remainKey)
	if err != nil {
		logx.Errorf("判断剩余金额 key 是否存在失败, redPacketId=%d: %v", redPacketId, err)
		return 0, err
	}
	if exists {
		remainStr, err := s.rds.GetCtx(ctx, remainKey)
		if err != nil {
			logx.Errorf("读取剩余金额失败, redPacketId=%d: %v", redPacketId, err)
			return 0, err
		}
		remain, _ := strconv.ParseInt(remainStr, 10, 64)
		if remain < 0 {
			remain = 0
		}
		return remain, nil
	}

	// remainKey 不存在 → DB 差额兜底
	return s.calculateRemainFromDB(ctx, redPacketId)
}

// calculateRemainFromDB 用 DB 差额兜底计算剩余金额：总金额 − 已领取金额。
// 不依赖 Redis pool 存活，pool TTL 过期后仍能正确退款。
//
// 双花防护：先检查 pending 计数（已 LPOP 未入账），> 0 说明还有在途领取未入账，
// DB 差额会漏算它们导致退款多退、worker 稍后入账后双花——返回错误交给 ExpirationConsumer 重试。
func (s *RedPacketService) calculateRemainFromDB(ctx context.Context, redPacketId int64) (int64, error) {
	pendingKey := constants.GetPendingKey(redPacketId)
	exists, err := s.rds.ExistsCtx(ctx, pendingKey)
	if err != nil {
		return 0, err
	}
	if exists {
		pendingStr, err := s.rds.GetCtx(ctx, pendingKey)
		if err != nil {
			return 0, err
		}
		pending, _ := strconv.ParseInt(pendingStr, 10, 64)
		if pending > 0 {
			// 已经 LPOP 但未入账，remainKey 丢了才会 使用 DB 进行兜底，才会用到 pendingKey！
			return 0, fmt.Errorf("红包仍有 %d 条领取未入账，暂不能退款: redPacketId=%d", pending, redPacketId)
		}
	}

	rp, err := s.RedPacketModel.FindOne(ctx, redPacketId)
	if err != nil {
		return 0, err
	}
	_, received, err := s.RedPacketReceiveModel.CountSumByRedPacketId(ctx, redPacketId)
	if err != nil {
		return 0, err
	}
	remain := rp.TotalAmount - received
	if remain < 0 {
		remain = 0
	}
	return remain, nil
}

// 扣减用户余额
func (s *RedPacketService) DeductBalance(ctx context.Context, userId, amount int64) error {
	affected, err := s.UserBalanceModel.DeductBalanceTx(ctx, userId, amount)
	common.ThrowIfWithMsg(err != nil, common.MysqlError, "扣余额失败 DeductBalance", err)
	common.ThrowIfWithMsg(affected == 0, common.MysqlError, "余额不足 DeductBalance")
	return nil
}

// InitRedisPool 初始化 Redis 红包金额池（幂等，可安全重试）。
//
// 用 Lua 原子完成：pool 不存在才写金额（重试不会 RPUSH 翻倍）、SET remainKey = 总额、
// ZADD NX 登记过期 ZSet（不覆盖已有 score，避免 pool-init 事件重复处理漂移过期时间）。
// expireTimestamp 由调用方传入（发送时刻计算的原始过期时间戳），重试不重算。
func (s *RedPacketService) InitRedisPool(ctx context.Context, redPacketId int64, amounts []int64, expireTimestamp int64) error {
	poolKey := constants.GetPoolKey(redPacketId)
	remainKey := constants.GetRemainKey(redPacketId)
	ttlSeconds := int64(constants.RedisCacheExpireHours * 3600)

	// ARGV: [redPacketId, expireTimestamp, ttlSeconds, amount1, amount2, ...]
	argv := make([]any, 0, len(amounts)+3)
	argv = append(argv,
		strconv.FormatInt(redPacketId, 10),
		strconv.FormatInt(expireTimestamp, 10),
		strconv.FormatInt(ttlSeconds, 10),
	)
	for _, a := range amounts {
		argv = append(argv, strconv.FormatInt(a, 10))
	}

	initScript := redis.NewScript(script.InitRedisPoolLua)
	_, err := s.rds.ScriptRunCtx(ctx, initScript, []string{poolKey, constants.ExpireZSet, remainKey}, argv...)
	if err != nil {
		logx.Errorf("初始化红包金额池失败, redPacketId=%d: %v", redPacketId, err)
	}
	return err
}
