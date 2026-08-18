// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshUriLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshUriLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshUriLogic {
	return &RefreshUriLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshUriLogic) RefreshUri(req *types.UserIdRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
