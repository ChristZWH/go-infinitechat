// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"context"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetApplyListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetApplyListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetApplyListLogic {
	return &GetApplyListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetApplyListLogic) GetApplyList(req *types.FriendApplyListRequest) (resp *types.FriendApplyListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
