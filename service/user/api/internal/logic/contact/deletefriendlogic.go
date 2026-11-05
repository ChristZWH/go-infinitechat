// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"context"

	"go-infinitechat/common/common"
	"go-infinitechat/common/identity"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteFriendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteFriendLogic {
	return &DeleteFriendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteFriendLogic) DeleteFriend(req *types.UserIdAndReceiveUserIdRequest) (bool, error) {
	// 只能删自己的好友：操作者身份取自网关注入的登录态，不采信路径里的 userId
	if userId, ok := identity.UserId(l.ctx); ok {
		req.UserId = userId
	}

	common.ThrowIfWithMsg(req.UserId <= 0, common.UserNotExists, "用户不存在")
	common.ThrowIfWithMsg(req.ReceiveUserId <= 0, common.UserNotExists, "用户不存在")
	return l.svcCtx.FriendService.DeleteFriend(l.ctx, req.UserId, req.ReceiveUserId), nil
}
