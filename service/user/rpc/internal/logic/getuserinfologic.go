package logic

import (
	"context"

	"go-infinitechat/service/user/rpc/internal/svc"
	"go-infinitechat/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserInfoLogic {
	return &GetUserInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 供其他微服务调用：获取用户基本信息
func (l *GetUserInfoLogic) GetUserInfo(in *user.UserIdReq) (*user.UserInfoResp, error) {
	resp, err := l.svcCtx.UserModel.FindOne(l.ctx, in.UserId)
	if err != nil {
		logx.Errorf("查询用户信息失败：userId=%d,err=%s", in.UserId, err.Error())
		return nil, err
	}

	return &user.UserInfoResp{
		UserId:   resp.UserId,
		Nickname: resp.Nickname.String,
		Avatar:   resp.Avatar,
	}, nil
}
