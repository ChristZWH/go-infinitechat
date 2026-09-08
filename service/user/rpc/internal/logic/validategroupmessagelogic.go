package logic

import (
	"context"

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

func (l *ValidateGroupMessageLogic) ValidateGroupMessage(in *user.ValidateGroupMessageReq) (*user.MessageValidateResp, error) {
	// todo: add your logic here and delete this line

	return &user.MessageValidateResp{}, nil
}
