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
	"go-infinitechat/service/redpacket/api/internal/types/constants"
	"go-infinitechat/service/user/rpc/userrpc"

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
	common.Infof("查询红包基本信息，红包ID: %d\n", req.RedPacketId)

	// 1. 查询红包基本信息（DB 故障与「红包不存在」分开处理）
	rp, findErr := l.svcCtx.RedPacketModel.FindOne(l.ctx, req.RedPacketId)
	common.ThrowIf(findErr != nil && !errors.Is(findErr, sqlx.ErrNotFound), common.SystemError, findErr)
	common.ThrowIfWithMsg(rp == nil, common.NotFoundError, "红包不存在")

	// 2. 权限校验：会话成员可见（封面所有人可见，领取记录才限发送者+领取者）
	userId, ok := identity.UserId(l.ctx)
	common.ThrowIfWithMsg(!ok, common.NoAuthError, "未登录")
	common.ThrowIfWithMsg(!isSessionMember(l.ctx, l.svcCtx, userId, rp.SenderId, rp.SessionId, rp.SessionType.Int64), common.NoAuthError, "无权查看该红包")

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

// isSessionMember 判断 userId 是否是红包所在会话的成员。
// 群聊（SessionType=0）：查 user_session 群成员关系；单聊（SessionType=1）：必须是发送者好友（即单聊接收者）。
// 用于 basic 查询的「封面可见」与 receive 的「领取授权」。
func isSessionMember(ctx context.Context, svcCtx *svc.ServiceContext, userId, senderId, sessionId, sessionType int64) bool {
	if userId == senderId {
		return true
	}
	if svcCtx.UserRPC == nil {
		return false
	}
	if sessionType == constants.SessionTypeGroup {
		// 群聊：非成员时 GetUserSessionRole 返回 error
		_, err := svcCtx.UserRPC.GetUserSessionRole(ctx, &userrpc.UserSessionRoleReq{UserId: userId, SessionId: sessionId})
		return err == nil
	}
	// 单聊：接收者必须是发送者好友（双方活跃且未拉黑）
	resp, err := svcCtx.UserRPC.ValidateSingleMessage(ctx, &userrpc.ValidateSingleMessageReq{SenderId: senderId, ReceiverId: userId})
	if err != nil {
		return false
	}
	return resp != nil && resp.Allowed
}
