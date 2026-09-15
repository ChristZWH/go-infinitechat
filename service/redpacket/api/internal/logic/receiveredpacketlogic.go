// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"time"

	"go-infinitechat/common/common"
	"go-infinitechat/service/redpacket/api/internal/service"
	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types"
	"go-infinitechat/service/redpacket/api/internal/types/constants"

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
	logx.Infof("领取红包请求，用户ID: %d, 红包ID: %d", req.UserId, req.RedPacketId)

	// 执行抢红包 Lua 脚本
	resultCode, completed, err := l.svcCtx.RedPacketService.ExecuteReceiveLua(l.ctx, req.RedPacketId, req.UserId)
	common.ThrowIfWithMsg(err != nil, common.ParamsError, "红包参数错误", err)

	resp = &types.ReceiveResultVO{}

	// 已领取过 -1
	if resultCode == constants.AlreadyReceived {
		// 获取具体领取到的金额
		resp.Amount = l.svcCtx.RedPacketService.GetReceivedAmount(req.RedPacketId, req.UserId)
		resp.Message = "您已经领取过该红包了"
		// "此分支 Status 仅为占位，客户端以 Message 为准"—— 毕竟重复领取是低频路径，为它多查一次库不值
		resp.Status = constants.StatusNotCompleted
	}
	// 已过期 -2
	if resultCode == constants.AlreadyExpired {
		resp.Amount = ""
		resp.Message = "红包已过期"
		resp.Status = constants.StatusExpired
	}

	// 无 poolKey ；redis 数据为空，查库获取真实数据
	if resultCode == constants.EmptyPool {
		rp, err := l.svcCtx.RedPacketModel.FindOne(l.ctx, req.RedPacketId)
		common.ThrowIfWithMsg(err != nil, common.MysqlError, "红包查询操作失败", err)
		resp.Status = int(rp.Status)
		switch resp.Status {
		case constants.StatusCompleted:
			resp.Message = "红包已领取完"
		case constants.StatusExpired:
			resp.Message = "红包已过期"
		default:
			// Redis 是实时真相，MySQL 是异步追账，两者之间永远有个小时间窗，default 分支就是给这个时间窗兜底的
			resp.Message = "暂时无法领取"
		}
		return resp, nil
	}

	// 领取成功（Lua 返回正数=领取金额，负数=错误码已在上面处理）
	if resultCode <= 0 {
		common.ThrowWithMsg(common.SystemError, "Lua返回未知结果码")
	}

	// 领取成功
	amount := resultCode
	resp.Amount = service.ConvertFenToYuan(amount)
	resp.Message = "恭喜您，领取成功"

	// 发送领取记录到 Kafka
	err = l.svcCtx.KafkaPusherManager.Push(l.ctx, constants.TopicRedpacketReceive, map[string]interface{}{
		"userId":         req.UserId,
		"redPacketId":    req.RedPacketId,
		"receivedAmount": amount,
		"receivedTime":   time.Now().UnixMilli(),
	})
	common.ThrowIfWithMsg(err != nil, common.SystemError, "Kafka 消息推送失败 TopicRedpacketReceive", err)

	if constants.IsCompleted(completed) {
		resp.Status = constants.StatusCompleted
		err := l.svcCtx.KafkaPusherManager.Push(l.ctx, constants.TopicRedpacketCompleted, map[string]interface{}{
			"redPacketId": req.RedPacketId,
		})
		common.ThrowIfWithMsg(err != nil, common.SystemError, "Kafka 消息推送失败 TopicRedpacketCompleted", err)
	}

	logx.Infof("红包领取处理完成，用户ID: %d, 状态: %d, 金额: %s", req.UserId, resp.Status, resp.Amount)
	return resp, nil
}
