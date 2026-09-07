// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"
	"strconv"

	"go-infinitechat/common/common"
	commonconstants "go-infinitechat/common/model/constants"
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

func (l *RefreshUriLogic) RefreshUri(req *types.UserIdRequest) (string, error) {
	wsServerUri := l.svcCtx.WsServerLocator.GetWsServerUri(strconv.FormatInt(req.UserId, 10))
	common.ThrowIfWithMsg(wsServerUri == "", common.SystemError, "WS服务暂不可用, 请稍后重试")
	l.svcCtx.Redis.Hset(commonconstants.RedisWsServerUri, strconv.FormatInt(req.UserId, 10), wsServerUri)
	return wsServerUri, nil
}
