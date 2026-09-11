// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package offlinedata

import (
	"context"

	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type HistoricalMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHistoricalMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HistoricalMessageLogic {
	return &HistoricalMessageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HistoricalMessageLogic) HistoricalMessage(req *types.HistoryMessageRequest) (resp []types.OfflineHistoryMessageResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
