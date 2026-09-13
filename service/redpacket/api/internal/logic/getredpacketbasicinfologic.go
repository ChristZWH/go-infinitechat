// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRedPacketBasicInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRedPacketBasicInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRedPacketBasicInfoLogic {
	return &GetRedPacketBasicInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRedPacketBasicInfoLogic) GetRedPacketBasicInfo(req *types.RedPacketBasicRequest) (resp *types.RedPacketBasicVO, err error) {
	// todo: add your logic here and delete this line

	return
}
