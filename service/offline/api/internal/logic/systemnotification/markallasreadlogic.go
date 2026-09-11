// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package systemnotification

import (
	"context"

	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkAllAsReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkAllAsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkAllAsReadLogic {
	return &MarkAllAsReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkAllAsReadLogic) MarkAllAsRead(req *types.MarkAllAsReadRequest) (resp int, err error) {
	// todo: add your logic here and delete this line

	return
}
