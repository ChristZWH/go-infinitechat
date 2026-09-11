// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package systemnotification

import (
	"context"

	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkAsReadBatchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkAsReadBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkAsReadBatchLogic {
	return &MarkAsReadBatchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkAsReadBatchLogic) MarkAsReadBatch(req *types.MarkAsReadBatchRequest) (resp int32, err error) {
	// todo: add your logic here and delete this line

	return
}
