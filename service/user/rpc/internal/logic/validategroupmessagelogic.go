package logic

import (
	"context"

	"go-infinitechat/common/common"
	"go-infinitechat/service/user/rpc/internal/svc"
	"go-infinitechat/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateGroupMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateGroupMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateGroupMessageLogic {
	return &ValidateGroupMessageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 供其他微服务调用：验证群聊是否能发
func (l *ValidateGroupMessageLogic) ValidateGroupMessage(in *user.ValidateGroupMessageReq) (*user.MessageValidateResp, error) {
	senderId := in.SenderId
	sessionId := in.SessionId

	// 1.验证发送者用户状态
	sender, err := l.svcCtx.UserModel.FindOne(l.ctx, senderId)
	if err != nil || sender == nil || sender.State != 0 {
		return &user.MessageValidateResp{
			Allowed:      false,
			RejectReason: common.SenderDisabled.Message,
			ErrorCode:    int32(common.SenderDisabled.Code),
		}, nil
	}

	// 2.校验群成员资格
	us, err := l.svcCtx.UserSessionModel.FindOne(l.ctx, sender.UserId, sessionId)
	if err != nil || us == nil {
		return &user.MessageValidateResp{
			Allowed:      false,
			RejectReason: common.NotGroupMember.Message,
			ErrorCode:    int32(common.NotGroupMember.Code),
		}, nil
	}

	return &user.MessageValidateResp{
		Allowed: true,
	}, nil
}
