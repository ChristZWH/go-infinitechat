// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendRedPacketLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendRedPacketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendRedPacketLogic {
	return &SendRedPacketLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SendRedPacketLogic) SendRedPacket(req *types.RedPacketSendRequest) (resp *types.RedPacketSendVO, err error) {
	// todo: add your logic here and delete this line

	return
}
