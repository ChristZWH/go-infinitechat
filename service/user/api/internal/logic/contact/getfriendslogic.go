// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"context"
	"strconv"

	dto2 "go-infinitechat/common/model/dto"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/dto"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFriendsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetFriendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFriendsLogic {
	return &GetFriendsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFriendsLogic) GetFriends(req *types.GetFriendRequest) (resp *dto2.PageResponse[dto.FriendDTO], err error) {
	return l.svcCtx.FriendService.GetFriends(l.ctx, strconv.FormatInt(req.UserId, 10), int32(req.PageNum), int32(req.PageSize), req.KeyWord), nil
}
