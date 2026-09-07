package service

import (
	"context"
	"errors"
	"go-infinitechat/common/common"
	"go-infinitechat/service/user/api/internal/types/dto"
	"go-infinitechat/service/user/model/user_balance"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
)

type BalanceService struct {
	UserBalanceModel user_balance.UserBalanceModel
	rds              *redis.Redis
}

func NewBalanceService(userModel user_balance.UserBalanceModel, rds *redis.Redis) *BalanceService {
	return &BalanceService{
		UserBalanceModel: userModel,
		rds:              rds,
	}
}

/**
 * 初始化用户余额
 *  userId 用户ID
 *  initialBalance 初始余额（单位：分）
 */
func (bs *BalanceService) InitializaBalance(ctx context.Context, userId int64, initialBalance int64) {
	common.ThrowIfWithMsg(userId <= 0, common.ParamsError, "用户ID不能为空")
	common.ThrowIfWithMsg(initialBalance < 0, common.ParamsError, "初始越不能为负数")

	// 检查是否已存在余额记录
	userBalance, err := bs.UserBalanceModel.FindOne(ctx, userId)
	// 出现非正常下的错误
	common.ThrowIf(err != nil && errors.Is(err, sqlc.ErrNotFound), common.SystemError, err)
	common.ThrowIfWithMsg(userBalance != nil, common.OperationError, "用户余额记录已存在")

	newUserBalance := &user_balance.UserBalance{
		UserId:      userId,
		Balance:     initialBalance,
		CreatedTime: time.Now(),
		UpdatedTime: time.Now(),
	}

	_, err = bs.UserBalanceModel.InsertTx(ctx, newUserBalance)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "初始化用户余额失败", err)

	common.Infof("初始化用户余额成功, userId:%d, initialBalance:%d\n", userId, initialBalance)
}

// 查询用户余额
func (bs *BalanceService) GetUserBalance(userId int64) dto.UserBalanceResponse {
	common.ThrowIfWithMsg(userId <= 0, common.UserAlreadyExists, "用户ID不能为空")
	userBalance, err := bs.UserBalanceModel.FindOne(context.Background(), userId)
	common.ThrowIf(err != nil && errors.Is(err, sqlc.ErrNotFound), common.SystemError, err)
	common.ThrowIfWithMsg(userBalance == nil, common.NotFoundError, "用户余额记录不存在")
	//构造响应对象（将分转换为元）
	return dto.UserBalanceResponse{
		Balance: decimal.NewFromInt(userBalance.Balance).Div(decimal.NewFromInt(100)).Round(2),
	}
}
