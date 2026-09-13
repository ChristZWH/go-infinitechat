// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRedPacketDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRedPacketDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRedPacketDetailLogic {
	return &GetRedPacketDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRedPacketDetailLogic) GetRedPacketDetail(req *types.RedPacketDetailRequest) (resp *types.RedPacketDetailVO, err error) {
	// todo: add your logic here and delete this line

	return
}
