// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"context"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BlockFriendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBlockFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BlockFriendLogic {
	return &BlockFriendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BlockFriendLogic) BlockFriend(req *types.UserIdAndReceiveUserIdRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
