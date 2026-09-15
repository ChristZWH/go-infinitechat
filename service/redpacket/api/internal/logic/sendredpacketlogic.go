// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"go-infinitechat/common/common"
	commonconsts "go-infinitechat/common/model/constants"
	"go-infinitechat/common/model/dto"
	"go-infinitechat/common/model/txctx"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/redpacket/api/internal/algorithm"
	"go-infinitechat/service/redpacket/api/internal/service"
	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
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
	l.validateSendRequest(req)

	// 业务数据准备
	senderId := req.SenderId
	totalAmountFen, _ := service.YuanToFen(req.Body.TotalAmount)
	totalCount := req.Body.TotalCount
	redPacketType := req.Body.RedPacketType

	redPacketId := utils.NextInt()
	now := time.Now()

	// 开启事务：扣减余额 + 创建红包记录 + 记录余额日志
	err = txctx.WithTransaction(l.ctx, l.svcCtx.SqlxConn, func(ctx context.Context) error {
		// 1. 扣减发送者余额
		deductErr := l.svcCtx.RedPacketService.DeductBalance(ctx, req.SenderId, totalAmountFen)
		common.ThrowIfWithMsg(deductErr != nil, common.OperationError, "余额不足或扣款失败", err)

		// 2. 创建红包记录
		_, insertErr := l.svcCtx.RedPacketModel.Insert(ctx, &red_packet.RedPacket{
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
		common.ThrowIfWithMsg(insertErr != nil, common.SystemError, "红包创建失败", err)

		// 3. 记录余额变动日志
		l.svcCtx.RedPacketService.InsertBalanceLog(ctx, req.SenderId, totalAmountFen, constants.BalanceLogTypeSend, redPacketId)
		return nil
	})
	common.ThrowIfWithMsg(err != nil, common.SystemError, "红包发送失败", err)
	// 事务结束

	// 红包金额预分配
	var amounts []int64
	if redPacketType == constants.TypeNormal {
		amounts, err = algorithm.AllocateNormalRedPacket(totalAmountFen, int(totalCount))
	} else {
		amounts, err = algorithm.AllocateRandomRedPacket(totalAmountFen, int(totalCount))
	}
	common.ThrowIfWithMsg(err != nil, common.SystemError, "红包金额分配失败")

	// 初始化 Redis 红包金额池 + 过期 ZSET
	l.svcCtx.RedPacketService.InitRedisPool(redPacketId, amounts)

	// 发红包消息到 Kafka (store + push)
	messageId := l.sendRedPacketMessage(req, redPacketId)

	// 发送红包创建事件到 Kafka（用于过期处理注册）
	_ = l.svcCtx.KafkaPusherManager.Push(l.ctx, constants.TopicRedpacketCreation, map[string]interface{}{
		"redPackageId": redPacketId,
		"createdTime":  time.Now().UnixMilli(),
	})

	logx.Infof("红包发送成功，红包ID: %d, 消息ID: %d, 发送者: %d, 金额: %d, 数量: %d", redPacketId, messageId, senderId, totalAmountFen, totalCount)

	// 构建返回结果
	return &types.RedPacketSendVO{
		RedPacketId: redPacketId,
		MessageId:   messageId,
	}, nil
}

// 发送红包消息到 Kafka
func (l *SendRedPacketLogic) sendRedPacketMessage(req *types.RedPacketSendRequest, redPacketId int64) int64 {
	if l.svcCtx.KafkaPusherManager == nil {
		return 0
	}

	// 构建 MessageRequest
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

	// 消息持久化（store-topic，无 key，offline 服务消费落库）
	_ = l.svcCtx.KafkaPusherManager.Push(l.ctx, commonconsts.KafkaMessageTopicStore, msgReq)

	// 消息推送（message-topic，用 sessionId 作为 key 保证同会话有序，realtime 服务消费推送）
	_ = l.svcCtx.KafkaPusherManager.KPush(l.ctx, commonconsts.KafkaMessageTopicPush, strconv.FormatInt(req.SessionId, 10), msgReq)

	logx.Infof("红包消息已入队，红包ID: %d, 消息ID: %d", redPacketId, messageId)
	return messageId
}

func (l *SendRedPacketLogic) validateSendRequest(req *types.RedPacketSendRequest) {
	totalAmountFen, err := service.YuanToFen(req.Body.TotalAmount)
	common.ThrowIfWithMsg(err != nil, common.ParamsError, "红包金额格式错误")
	common.ThrowIfWithMsg(totalAmountFen <= 0, common.ParamsError, "红包金额必须大于0")
	common.ThrowIfWithMsg(totalAmountFen < int64(req.Body.TotalCount), common.ParamsError, "红包总金额不能少于红包数量（每个红包至少1分）")
	singleAmountYuan := float64(totalAmountFen) / float64(req.Body.TotalCount) / float64(constants.YuanToFenMultiplier)
	common.ThrowIfWithMsg(singleAmountYuan > float64(constants.MaxSingleAmountYuan), common.ParamsError, fmt.Sprintf("单个红包金额不能超过%d元", constants.MaxSingleAmountYuan))
}
