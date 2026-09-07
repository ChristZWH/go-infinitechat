// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"go-infinitechat/common/common"
	commonDto "go-infinitechat/common/model/dto"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/dto"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type GetBalanceDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetBalanceDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBalanceDetailLogic {
	return &GetBalanceDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetBalanceDetailLogic) GetBalanceDetail(req *types.BalanceDetailRequest) (*commonDto.PageResponse[dto.BalanceLogDTO], error) {
	// 参数校验
	userId, err := strconv.ParseInt(req.UserId, 10, 64)
	common.ThrowIfWithMsg(err != nil, common.ParamsError, "用户ID格式错误")
	common.ThrowIfWithMsg(userId <= 0, common.ParamsError, "用户ID不能为空")

	pageNum := int(req.PageNum)
	pageSize := int(req.PageSize)
	// 兜底校验：api 里的 range 标签未生效（httpx 未注册 validator），logic 层补齐
	common.ThrowIfWithMsg(pageNum < 1, common.ParamsError, "页码必须大于0")
	common.ThrowIfWithMsg(pageSize < 1 || pageSize > 100, common.ParamsError, "每页条数需在1-100之间")

	// 查询总数
	total, err := l.svcCtx.BalanceLogModel.CountByUserIdTx(l.ctx, userId)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询用户余额明细总数失败", err)

	// 空数据直接返回
	if total == 0 {
		return commonDto.NewPageResponse([]dto.BalanceLogDTO{}, int(total), pageNum, pageSize), nil
	}

	//查询用户昵称（非关键字段：查询失败仅记日志，不阻断主流程）
	userName := ""
	userInfo, err := l.svcCtx.UserModel.FindOne(l.ctx, userId)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		common.Errorf("查询用户昵称失败, userId=%d: %v", userId, err)
	} else if userInfo != nil {
		userName = userInfo.Nickname.String
	}

	//分页查询余额日志
	logs, err := l.svcCtx.BalanceLogModel.FindPageByUserIdTx(l.ctx, userId, pageNum, pageSize)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询用户余额明细失败", err)
	list := make([]dto.BalanceLogDTO, 0, len(logs))
	for _, log := range logs {
		item := dto.BalanceLogDTO{
			UserName: userName,
			Type:     int32(log.Type),
			// 金额从分转换为元，保留2位小数
			Amount: decimal.NewFromInt(log.Amount).Div(decimal.NewFromInt(100)).Round(2),
			// 格式化时间：MM月dd日 HH:mm
			Time: fmt.Sprintf("%02d月%02d日 %02d:%02d",
				log.CreatedTime.Month(), log.CreatedTime.Day(),
				log.CreatedTime.Hour(), log.CreatedTime.Minute()),
		}
		list = append(list, item)
	}

	return commonDto.NewPageResponse(list, int(total), pageNum, pageSize), nil
}
