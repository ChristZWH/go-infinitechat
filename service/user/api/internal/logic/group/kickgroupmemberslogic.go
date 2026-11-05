// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package group

import (
	"context"

	"go-infinitechat/common/identity"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type KickGroupMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKickGroupMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KickGroupMembersLogic {
	return &KickGroupMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 踢人
func (l *KickGroupMembersLogic) KickGroupMembers(req *types.KickGroupMembersRequest) (resp *types.KickGroupMembersResponse, err error) {
	// 操作者只能是"我"（是否有权限踢人在 service 里再按角色校验）
	identity.OverrideIfPresent(l.ctx, &req.OperatorId)
	return l.svcCtx.GroupService.KickGroupMembers(l.ctx, req), nil
}
