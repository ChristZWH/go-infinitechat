// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package group

import (
	"context"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExitGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExitGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExitGroupLogic {
	return &ExitGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 退群
func (l *ExitGroupLogic) ExitGroup(req *types.GroupExitRequest) (bool, error) {
	return l.svcCtx.GroupService.ExitGroup(l.ctx, req), nil
}
