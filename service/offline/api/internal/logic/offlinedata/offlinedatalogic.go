// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package offlinedata

import (
	"context"

	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OfflineDataLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOfflineDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OfflineDataLogic {
	return &OfflineDataLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OfflineDataLogic) OfflineData(req *types.OfflineDataRequest) (resp []types.OfflineDataResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
