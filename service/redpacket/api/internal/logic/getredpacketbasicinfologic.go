// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"go-infinitechat/common/common"
	"go-infinitechat/service/redpacket/api/internal/service"
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

// 查询红包基本信息
func (l *GetRedPacketBasicInfoLogic) GetRedPacketBasicInfo(req *types.RedPacketBasicRequest) (resp *types.RedPacketBasicVO, err error) {
	common.Infof("查询红包基本信息，红包ID: %d\n", req.RedPacketId)

	// 1. 查询红包基本信息
	rp, findErr := l.svcCtx.RedPacketModel.FindOne(l.ctx, req.RedPacketId)
	common.ThrowIfWithMsg(findErr != nil, common.NotFoundError, "红包不存在", findErr)

	// 2. 计算已领取统计信息（聚合查询，只返回一行）
	receivedCount, receivedAmountFen, statErr := l.svcCtx.RedPacketReceiveModel.CountSumByRedPacketId(l.ctx, req.RedPacketId)
	common.ThrowIfWithMsg(statErr != nil, common.SystemError, "统计红包领取信息失败", statErr)

	common.Infof("红包基本信息查询完成，红包ID: %d, 状态: %d, 已领取数量: %d\n", req.RedPacketId, rp.Status, receivedCount)

	return &types.RedPacketBasicVO{
		RedPacketId:    rp.RedPacketId,
		RedPacketType:  int(rp.RedPacketType),
		TotalAmount:    service.ConvertFenToYuan(rp.TotalAmount),
		TotalCount:     int(rp.TotalCount),
		ReceivedCount:  receivedCount,
		ReceivedAmount: service.ConvertFenToYuan(receivedAmountFen),
		Status:         int(rp.Status),
		CreatedTime:    rp.CreatedTime.Format("2006-01-02 15:04:05"),
	}, nil
}
