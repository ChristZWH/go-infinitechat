// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"context"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUnreadApplyCountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUnreadApplyCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUnreadApplyCountLogic {
	return &GetUnreadApplyCountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUnreadApplyCountLogic) GetUnreadApplyCount(req *types.UserIdPathRequest) (resp map[string]int64, err error) {
	count := l.svcCtx.ApplyFriendServer.GetUnreadCount(l.ctx, req.UserId)
	return map[string]int64{"count": count}, nil
}
