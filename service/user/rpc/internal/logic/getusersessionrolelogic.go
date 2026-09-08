package logic

import (
	"context"

	"go-infinitechat/service/user/rpc/internal/svc"
	"go-infinitechat/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserSessionRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserSessionRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserSessionRoleLogic {
	return &GetUserSessionRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 供其他微服务调用：查询用户在群聊中的角色
// 角色值：0-群主，1-管理员，2-普通成员
func (l *GetUserSessionRoleLogic) GetUserSessionRole(in *user.UserSessionRoleReq) (*user.UserSessionRoleResp, error) {
	resp, err := l.svcCtx.UserSessionModel.FindOne(l.ctx, in.UserId, in.SessionId)
	if err != nil {
		logx.Errorf("查询用户群角色失败: userId=%d, sessionId=%d, err=%s", in.UserId, in.SessionId, err.Error())
		return nil, err
	}

	return &user.UserSessionRoleResp{
		Role: int32(resp.Role),
	}, nil
}
