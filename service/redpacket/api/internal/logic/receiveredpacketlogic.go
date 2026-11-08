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

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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
	// 领取人只能是"我"，以网关登录为准
	identity.OverrideIfPresent(l.ctx, &req.UserId)
	logx.Infof("领取红包请求，用户ID: %d, 红包ID: %d", req.UserId, req.RedPacketId)

	// 不能领自己的红包（查库拿到发送者，DB 故障与"红包不存在"分开处理）
	rp, err := l.svcCtx.RedPacketModel.FindOne(l.ctx, req.RedPacketId)
	common.ThrowIf(err != nil && !errors.Is(err, sqlx.ErrNotFound), common.SystemError, err)
	common.ThrowIfWithMsg(rp == nil, common.NotFoundError, "红包不存在")
	common.ThrowIfWithMsg(rp.SenderId == req.UserId, common.OperationError, "不能领取自己的红包")

	// 执行抢红包 Lua 脚本
	resultCode, completed := l.svcCtx.RedPacketService.ExecuteReceiveLua(l.ctx, req.RedPacketId, req.UserId)

	resp = &types.ReceiveResultVO{}

	// 已领取过 -1
	if resultCode == constants.AlreadyReceived {
		// 获取具体领取到的金额
		resp.Amount = l.svcCtx.RedPacketService.GetReceivedAmount(req.RedPacketId, req.UserId)
		resp.Message = "您已经领取过该红包了"
		// "此分支 Status 仅为占位，客户端以 Message 为准"—— 毕竟重复领取是低频路径，为它多查一次库不值
		resp.Status = constants.StatusNotCompleted
		return resp, nil
	}
	// 已过期 -2
	if resultCode == constants.AlreadyExpired {
		resp.Amount = ""
		resp.Message = "红包已过期"
		resp.Status = constants.StatusExpired
		return resp, nil
	}

	// 无 poolKey -3 ：Redis 金额池为空（被并发领完或缓存过期），用 DB 真实状态兜底。
	// rp 已在方法开头查过并校验存在，这里直接复用，避免二次查库；
	// 也修掉原先「第 71 行刚拆出 ErrNotFound、第 72 行又对所有 err 抛 MysqlError」的自相矛盾
	// （否则"红包不存在"会被误标成系统错误 500）。
	if resultCode == constants.EmptyPool {
		resp.Status = int(rp.Status)
		switch resp.Status {
		case constants.StatusCompleted:
			resp.Message = "红包已领取完"
		case constants.StatusExpired:
			resp.Message = "红包已过期"
		default:
			// Redis 是实时真相，MySQL 是异步追账，两者之间永远有个小时间窗，default 分支就是给这个时间窗兜底的，追求最终一致性；中间态
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

	// 领取入账不再走 Kafka：抢包 Lua 已把领取事件原子写入 Redis Stream（发件箱），
	// 由 receive stream worker 消费落库。LPOP + HSET + XADD 同一脚本 = 原子，
	// 不存在「抢到但事件丢失、钱永远不到账」的窗口。
	//
	// 领完事件也一样：Lua 在 left==0 时把 completed=1 写进同一条事件，
	// 由 worker 推 Kafka（CompletedConsumer 落库标记 + 清缓存），请求路径不再裸推。
	if constants.IsCompleted(completed) {
		resp.Status = constants.StatusCompleted
	}

	logx.Infof("红包领取处理完成，用户ID: %d, 状态: %d, 金额: %s", req.UserId, resp.Status, resp.Amount)
	return resp, nil
}
