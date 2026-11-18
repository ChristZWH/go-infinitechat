// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"

	"go-infinitechat/common/common"
	"go-infinitechat/common/identity"
	"go-infinitechat/service/redpacket/api/internal/service"
	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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
	common.Infof("查询红包基本信息，红包ID: %d", req.RedPacketId)

	// 1. 查询红包基本信息（DB 故障与「红包不存在」分开处理）
	rp, findErr := l.svcCtx.RedPacketModel.FindOne(l.ctx, req.RedPacketId)
	common.ThrowIf(findErr != nil && !errors.Is(findErr, sqlx.ErrNotFound), common.SystemError, findErr)
	common.ThrowIfWithMsg(rp == nil, common.NotFoundError, "红包不存在")

	// 2. 权限校验：仅发送者或已领取者可见
	assertCanViewRedPacket(l.ctx, l.svcCtx, req.RedPacketId, rp.SenderId)

	// 3. 计算已领取统计信息（聚合查询，只返回一行）
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

// assertCanViewRedPacket 校验当前登录用户是否有权查看该红包：仅发送者或已领取者可见。
// 用于 basic/detail 两个查询接口，堵住「任意登录用户按 redPacketId 遍历他人红包详情」的信息泄露。
func assertCanViewRedPacket(ctx context.Context, svcCtx *svc.ServiceContext, redPacketId, senderId int64) {
	userId, ok := identity.UserId(ctx)
	common.ThrowIfWithMsg(!ok, common.NoAuthError, "未登录")
	if userId == senderId {
		return
	}
	rec, err := svcCtx.RedPacketReceiveModel.FindOneByRedPacketIdReceiverId(ctx, redPacketId, userId)
	common.ThrowIf(err != nil && !errors.Is(err, sqlx.ErrNotFound), common.SystemError, err)
	common.ThrowIfWithMsg(rec == nil, common.NoAuthError, "无权查看该红包")
}
