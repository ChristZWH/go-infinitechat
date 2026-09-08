package logic

import (
	"context"

	"go-infinitechat/service/user/rpc/internal/svc"
	"go-infinitechat/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupMemberIdsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGroupMemberIdsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupMemberIdsLogic {
	return &GetGroupMemberIdsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 供其他微服务调用：获取群聊所有成员的 userId 列表
func (l *GetGroupMemberIdsLogic) GetGroupMemberIds(in *user.SessionIdReq) (*user.GroupMemberIdsResp, error) {
	resp, err := l.svcCtx.UserSessionModel.FindMemberIdsBySessionId(l.ctx, in.SessionId)
	if err != nil {
		logx.Errorf("查询群成员失败: sessionId=%d, err=%s", in.SessionId, err.Error())
		return nil, err
	}

	return &user.GroupMemberIdsResp{
		UserIds: resp,
	}, nil
}
