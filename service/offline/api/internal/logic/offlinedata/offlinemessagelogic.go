// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package offlinedata

import (
	"context"

	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OfflineMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOfflineMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OfflineMessageLogic {
	return &OfflineMessageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OfflineMessageLogic) OfflineMessage(req *types.OfflineMessageRequest) (resp []types.OfflineHistoryMessageResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
