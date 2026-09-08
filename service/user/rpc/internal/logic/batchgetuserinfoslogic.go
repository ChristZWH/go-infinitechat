package logic

import (
	"context"

	"go-infinitechat/service/user/rpc/internal/svc"
	"go-infinitechat/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchGetUserInfosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchGetUserInfosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchGetUserInfosLogic {
	return &BatchGetUserInfosLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 供其他微服务调用：批量获取用户信息
func (l *BatchGetUserInfosLogic) BatchGetUserInfos(in *user.BatchGetUserInfosReq) (*user.BatchGetUserInfosResp, error) {
	if len(in.UserIds) == 0 {
		return &user.BatchGetUserInfosResp{}, nil
	}

	users := make([]*user.BatchUserInfoItem, 0, len(in.UserIds))
	for _, item := range in.UserIds {
		u, err := l.svcCtx.UserModel.FindOne(l.ctx, item)
		if err == nil || u == nil {
			continue
		}
		users = append(users, &user.BatchUserInfoItem{
			UserId:   u.UserId,
			Nickname: u.Nickname.String,
			Avatar:   u.Avatar,
		})
	}

	return &user.BatchGetUserInfosResp{Users: users}, nil
}
