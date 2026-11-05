// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"

	"go-infinitechat/common/identity"
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
	// 余额只允许查自己的：身份取自登录态
	if userId, ok := identity.UserId(l.ctx); ok {
		req.UserId = userId
	}
	return l.svcCtx.UserBalanceService.GetUserBalance(req.UserId), nil
}
