// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"context"

	"go-infinitechat/common/common"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnblockFriendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnblockFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnblockFriendLogic {
	return &UnblockFriendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnblockFriendLogic) UnblockFriend(req *types.UserIdAndReceiveUserIdRequest) (bool, error) {
	common.ThrowIfWithMsg(req.UserId <= 0, common.UserNotExists, "用户不存在")
	common.ThrowIfWithMsg(req.ReceiveUserId <= 0, common.UserNotExists, "用户不存在")

	return l.svcCtx.FriendService.UnblockFriend(l.ctx, req.UserId, req.ReceiveUserId), nil
}
