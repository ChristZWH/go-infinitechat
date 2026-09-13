// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReceiveRedPacketLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReceiveRedPacketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReceiveRedPacketLogic {
	return &ReceiveRedPacketLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReceiveRedPacketLogic) ReceiveRedPacket(req *types.RedPacketReceiveRequest) (resp *types.ReceiveResultVO, err error) {
	// todo: add your logic here and delete this line

	return
}
