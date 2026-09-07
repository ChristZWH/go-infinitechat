// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"context"
	"strconv"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchUserLogic {
	return &SearchUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchUserLogic) SearchUser(req *types.SearchUserRequest) (resp *types.FriendDetailVO, err error) {
	return l.svcCtx.FriendService.SearchUserByKey(l.ctx, strconv.FormatInt(req.UserId, 10), req.KeyWord), nil
}
