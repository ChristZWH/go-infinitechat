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
	common.Infof("查询红包详情，红包ID: %d, pageNum: %d, pageSize: %d\n", req.RedPacketId, req.PageNum, req.PageSize)

	// 1. 查询红包基本信息
	rp, findErr := l.svcCtx.RedPacketModel.FindOne(l.ctx, req.RedPacketId)
	common.ThrowIfWithMsg(findErr != nil, common.NotFoundError, "红包不存在", findErr)

	sessionType := int(rp.SessionType.Int64)

	resp = &types.RedPacketDetailVO{
		RedPacketId:          rp.RedPacketId,
		SenderId:             rp.SenderId,
		SessionId:            rp.SessionId,
		SessionType:          sessionType,
		RedPacketWrapperText: rp.RedPacketWrapperText,
		RedPacketType:        int(rp.RedPacketType),
		TotalAmount:          service.ConvertFenToYuan(rp.TotalAmount),
		TotalCount:           int(rp.TotalCount),
		Status:               int(rp.Status),
		CreatedTime:          rp.CreatedTime.Format("2006-01-02 15:04:05"),
	}

	// 2. 分页查询 已接收红包的个数
	receivedCount, receivedAmountFen, statErr := l.svcCtx.RedPacketReceiveModel.CountSumByRedPacketId(l.ctx, req.RedPacketId)
	common.ThrowIfWithMsg(statErr != nil, common.SystemError, "统计领取信息失败", statErr)
	resp.ReceivedAmount = service.ConvertFenToYuan(receivedAmountFen)
	resp.ReceivedCount = receivedCount

	// 3. 分页查询领取记录
	receiveList, queryErr := l.svcCtx.RedPacketReceiveModel.FindPageByRedPacketId(l.ctx, req.RedPacketId, req.PageNum, req.PageSize)
	common.ThrowIfWithMsg(queryErr != nil, common.SystemError, "分页查询领取记录失败", queryErr)

	// 4. 批量获取用户信息
	userIds := []int64{rp.SenderId}
	for _, r := range receiveList {
		userIds = append(userIds, r.ReceiverId)
	}
	userInfoMap := l.svcCtx.RedPacketService.BatchGetUserInfos(l.ctx, userIds)

	// 5. 填充发送者信息
	if info, ok := userInfoMap[rp.SenderId]; ok {
		resp.SenderNickname = info.NickName
		resp.SenderAvatar = info.Avatar
	}

	// 6. 填充领取记录
	records := make([]types.RedPacketReceiveVO, 0, len(receiveList))
	for _, r := range receiveList {
		record := types.RedPacketReceiveVO{
			ReceiverId: r.ReceiverId,
			Amount:     service.ConvertFenToYuan(r.Amount),
			ReceivedAt: r.ReceivedAt.Format("2006-01-02 15:04:05"),
		}
		if info, ok := userInfoMap[r.ReceiverId]; ok {
			record.ReceiverAvatar = info.Avatar
			record.ReceiverNickname = info.NickName
		}
		records = append(records, record)
	}
	resp.ReceiveRecords = records

	common.Infof("红包详情查询完成，红包ID: %d, 状态: %d, 已领取数量: %d\n", req.RedPacketId, resp.Status, resp.ReceivedCount)
	return resp, nil
}
