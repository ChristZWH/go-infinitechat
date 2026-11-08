// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go-infinitechat/common/common"
	"go-infinitechat/common/identity"
	commonconsts "go-infinitechat/common/model/constants"
	"go-infinitechat/common/model/dto"
	"go-infinitechat/common/model/txctx"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/redpacket/api/internal/algorithm"
	"go-infinitechat/service/redpacket/api/internal/service"
	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
	"go-infinitechat/service/redpacket/model/event_outbox"
	"go-infinitechat/service/redpacket/model/red_packet"

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

// 发送红包
func (l *SendRedPacketLogic) SendRedPacket(req *types.RedPacketSendRequest) (resp *types.RedPacketSendVO, err error) {
	// 发送者只能是"我"，以网关登录为准
	identity.OverrideIfPresent(l.ctx, &req.SenderId)
	l.validateSendRequest(req)

	// 业务数据准备
	senderId := req.SenderId
	totalAmountFen, _ := service.YuanToFen(req.Body.TotalAmount)
	totalCount := req.Body.TotalCount
	redPacketType := req.Body.RedPacketType

	redPacketId := utils.NextInt()
	now := time.Now()

	// 先红包金额预分配 ——> 再开事务（分配是纯 CPU 计算，先做完再动钱）
	var amounts []int64
	if redPacketType == constants.TypeNormal {
		amounts, err = algorithm.AllocateNormalRedPacket(totalAmountFen, int(totalCount))
	} else {
		amounts, err = algorithm.AllocateRandomRedPacket(totalAmountFen, int(totalCount))
	}
	common.ThrowIfWithMsg(err != nil, common.SystemError, "红包金额分配失败")

	// 构建红包消息 + 建池事件（事务前拿到 messageId / msgJSON / poolInitPayload）
	messageId, msgJSON := l.buildRedPacketMessage(req, redPacketId)
	poolInitPayload := l.buildPoolInitPayload(redPacketId, amounts)

	// 开启事务：扣减余额 + 创建红包记录 + 记录余额日志 + 写发件箱
	err = txctx.WithTransaction(l.ctx, l.svcCtx.SqlxConn, func(ctx context.Context) error {
		// 1. 扣减发送者余额
		deductErr := l.svcCtx.RedPacketService.DeductBalance(ctx, req.SenderId, totalAmountFen)
		common.ThrowIfWithMsg(deductErr != nil, common.OperationError, "余额不足或扣款失败", deductErr)

		// 2. 创建红包记录（必须用 InsertTx，go-zero 的 CachedConn.Insert 不读 ctx 里的 tx session，
		//    否则第 3 步失败回滚时这条主表记录会留下来，变成永不退款的孤儿红包）
		_, insertErr := l.svcCtx.RedPacketModel.InsertTx(ctx, &red_packet.RedPacket{
			RedPacketId:          redPacketId,
			SenderId:             senderId,
			SessionId:            req.SessionId,
			SessionType:          sql.NullInt64{Int64: int64(req.SessionType), Valid: true},
			RedPacketWrapperText: req.Body.RedPacketWrapperText,
			RedPacketType:        int64(req.Body.RedPacketType),
			TotalAmount:          totalAmountFen,
			TotalCount:           int64(totalCount),
			Status:               constants.StatusNotCompleted,
			CreatedTime:          now,
			UpdatedTime:          now,
		})
		common.ThrowIfWithMsg(insertErr != nil, common.SystemError, "红包创建失败", insertErr)

		// 3. 记录余额变动日志
		l.svcCtx.RedPacketService.InsertBalanceLog(ctx, req.SenderId, totalAmountFen, constants.BalanceLogTypeSend, redPacketId)

		// 4. 写事务发件箱：store / push 两条消息事件 + pool-init 建池事件，
		//    与红包记录同事务落库，由 outbox_dispatcher 异步投递，投递失败可重试
		l.insertOutboxEvents(ctx, req.SessionId, messageId, redPacketId, msgJSON, poolInitPayload)

		return nil
	})
	common.ThrowIfWithMsg(err != nil, common.SystemError, "红包发送失败", err)
	// 事务结束

	// 同步建池（快速路径，幂等）：让红包立即可领。
	// 失败不回 500、也不在这里补写事件——pool-init 事件已在事务里落库（与扣款原子），
	// 投递器会异步重建，客户端不会重复扣款，红包也不会领不了/退不了。
	if err := l.svcCtx.RedPacketService.InitRedisPool(l.ctx, redPacketId, amounts); err != nil {
		logx.Errorf("同步建池失败，等待投递器异步重建: redPacketId=%d, err=%v", redPacketId, err)
	}

	logx.Infof("红包发送成功，红包ID: %d, 消息ID: %d, 发送者: %d, 金额: %d, 数量: %d", redPacketId, messageId, senderId, totalAmountFen, totalCount)

	// 构建返回结果
	return &types.RedPacketSendVO{
		RedPacketId: redPacketId,
		MessageId:   messageId,
	}, nil
}

// buildRedPacketMessage 构建红包消息并序列化为 JSON（发件箱 payload）。
// 返回 (messageId, payloadJSON)。
func (l *SendRedPacketLogic) buildRedPacketMessage(req *types.RedPacketSendRequest, redPacketId int64) (int64, string) {
	messageId := utils.NextInt()
	now := time.Now()
	msgReq := &dto.MessageRequest{
		SessionId:       req.SessionId,
		SenderId:        req.SenderId,
		Type:            dto.MessageTypeRedPacket, // 3
		SessionType:     int(req.SessionType),
		ClientMessageId: req.ClientMessageId,
		MessageId:       messageId,
		CreatedTime:     &now,
	}

	// 单聊：receiverId 必须不为 null；群聊：receiverId 必须为 null
	if req.SessionType == constants.SessionTypeSignal {
		msgReq.ReceiverId = req.ReceiverId
	}

	// 构建红包消息体
	msgReq.Body = &dto.MessageBody{
		RedPacketId:          strconv.FormatInt(redPacketId, 10),
		RedPacketWrapperText: req.Body.RedPacketWrapperText,
	}

	// 校验消息
	common.ThrowIf(req.SessionType == constants.SessionTypeSignal && msgReq.ReceiverId == nil, common.SignalTypeError)
	common.ThrowIf(req.SessionType == constants.SessionTypeGroup && msgReq.ReceiverId != nil, common.GroupTypeError)

	msgJSON, err := json.Marshal(msgReq)
	common.ThrowIfWithMsg(err != nil, common.MarshalFailedError, "红包消息序列化失败", err)
	return messageId, string(msgJSON)
}

// insertOutboxEvents 在事务内把 store / push 两条消息事件 + pool-init 建池事件写入发件箱。
// 必须调用在 txctx.WithTransaction 内，复用事务 session，与红包记录原子提交。
// pool-init 事件与扣款同事务，保证 Redis 抖动时金额池也能被投递器异步重建。
func (l *SendRedPacketLogic) insertOutboxEvents(ctx context.Context, sessionId, messageId, redPacketId int64, msgJSON, poolInitPayload string) {
	now := time.Now()

	// store 事件：无 key，offline 服务消费落库（消息持久化）
	_, err := l.svcCtx.EventOutboxModel.InsertTx(ctx, &event_outbox.EventOutbox{
		Topic:       commonconsts.KafkaMessageTopicStore,
		MessageKey:  "",
		DedupKey:    "store:" + strconv.FormatInt(messageId, 10),
		Payload:     msgJSON,
		Status:      event_outbox.StatusPending,
		CreatedTime: now,
		UpdatedTime: now,
	})
	common.ThrowIfWithMsg(err != nil, common.MysqlError, "写 store 发件箱失败", err)

	// push 事件：sessionId 做 key 保证同会话有序，realtime 服务消费推送
	_, err = l.svcCtx.EventOutboxModel.InsertTx(ctx, &event_outbox.EventOutbox{
		Topic:       commonconsts.KafkaMessageTopicPush,
		MessageKey:  strconv.FormatInt(sessionId, 10),
		DedupKey:    "push:" + strconv.FormatInt(messageId, 10),
		Payload:     msgJSON,
		Status:      event_outbox.StatusPending,
		CreatedTime: now,
		UpdatedTime: now,
	})
	common.ThrowIfWithMsg(err != nil, common.MysqlError, "写 push 发件箱失败", err)

	// pool-init 事件：与扣款同事务落库，投递器幂等重建金额池（快速路径失败时的兜底）
	_, err = l.svcCtx.EventOutboxModel.InsertTx(ctx, &event_outbox.EventOutbox{
		Topic:       event_outbox.TopicRedisPoolInit,
		MessageKey:  "",
		DedupKey:    "pool:" + strconv.FormatInt(redPacketId, 10),
		Payload:     poolInitPayload,
		Status:      event_outbox.StatusPending,
		CreatedTime: now,
		UpdatedTime: now,
	})
	common.ThrowIfWithMsg(err != nil, common.MysqlError, "写 pool-init 发件箱失败", err)
}

// buildPoolInitPayload 序列化「重建金额池」事件的 payload。
func (l *SendRedPacketLogic) buildPoolInitPayload(redPacketId int64, amounts []int64) string {
	payload, err := json.Marshal(map[string]interface{}{
		"redPacketId": redPacketId,
		"amounts":     amounts,
	})
	common.ThrowIfWithMsg(err != nil, common.MarshalFailedError, "序列化红包池初始化事件失败", err)
	return string(payload)
}

func (l *SendRedPacketLogic) validateSendRequest(req *types.RedPacketSendRequest) {
	common.ThrowIfWithMsg(req.Body.TotalCount <= 0, common.ParamsError, "红包数量必须大于0")
	totalAmountFen, err := service.YuanToFen(req.Body.TotalAmount)
	common.ThrowIfWithMsg(err != nil, common.ParamsError, "红包金额格式错误")
	common.ThrowIfWithMsg(totalAmountFen <= 0, common.ParamsError, "红包金额必须大于0")
	common.ThrowIfWithMsg(totalAmountFen < int64(req.Body.TotalCount), common.ParamsError, "红包总金额不能少于红包数量（每个红包至少1分）")
	singleAmountYuan := float64(totalAmountFen) / float64(req.Body.TotalCount) / float64(constants.YuanToFenMultiplier)
	common.ThrowIfWithMsg(singleAmountYuan > float64(constants.MaxSingleAmountYuan), common.ParamsError, fmt.Sprintf("单个红包金额不能超过%d元", constants.MaxSingleAmountYuan))
}
