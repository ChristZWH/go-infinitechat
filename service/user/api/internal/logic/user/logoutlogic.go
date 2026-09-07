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
	"go-infinitechat/service/user/api/internal/types/constants"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout(req *types.UserIdRequest) (bool, error) {
	indexKey := constants.UserRefreshTokenKeyPrefix + strconv.FormatInt(req.UserId, 10)
	refreshToken, err := l.svcCtx.Redis.Get(indexKey)
	common.ThrowIf(err != nil, common.SystemError, err)
	if refreshToken != "" {
		l.svcCtx.Redis.Del(refreshToken)
	}
	l.svcCtx.Redis.Del(indexKey)
	l.svcCtx.Redis.Hdel(commonconstants.RedisWsServerUri, strconv.FormatInt(req.UserId, 10))
	return true, nil
}
