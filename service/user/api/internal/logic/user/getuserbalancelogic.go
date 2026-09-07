// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/dto"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserBalanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserBalanceLogic {
	return &GetUserBalanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserBalanceLogic) GetUserBalance(req *types.UserIdPathRequest) (dto.UserBalanceResponse, error) {
	return l.svcCtx.UserBalanceService.GetUserBalance(req.UserId), nil
}
