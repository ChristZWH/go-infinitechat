package logic

import (
	"context"
	"errors"
	"strconv"

	"go-infinitechat/common/common"
	"go-infinitechat/common/model/constants"
	"go-infinitechat/service/user/rpc/internal/svc"
	"go-infinitechat/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	// 好友状态缓存 TTL（秒）。拉黑/解除拉黑后最多 2 分钟生效，
	// 不设 TTL 会导致一次脏写入永久生效。
	friendCacheTTLSeconds = 120
)

type ValidateSingleMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateSingleMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateSingleMessageLogic {
	return &ValidateSingleMessageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 供其他微服务调用：验证单聊能不能发（好友关系、拉黑）
func (l *ValidateSingleMessageLogic) ValidateSingleMessage(in *user.ValidateSingleMessageReq) (*user.MessageValidateResp, error) {
	senderId := in.SenderId
	receiverId := in.ReceiverId

	// 1. 校验发送者用户状态
	sender, err := l.svcCtx.UserModel.FindOne(l.ctx, in.SenderId)
	if err != nil || sender == nil || sender.State != 0 {
		return &user.MessageValidateResp{
			Allowed:      false,
			RejectReason: common.SenderDisabled.Message,
			ErrorCode:    int32(common.SenderDisabled.Code),
		}, nil
	}

	// 2. 校验接收者用户状态
	receiver, err := l.svcCtx.UserModel.FindOne(l.ctx, receiverId)
	if err != nil || receiver == nil || receiver.State != 0 {
		return &user.MessageValidateResp{
			Allowed:      false,
			RejectReason: common.ReceiverDisabled.Message,
			ErrorCode:    int32(common.ReceiverDisabled.Code),
		}, nil
	}

	// 3. 校验发送者→接收者的好友状态
	senderStatus := l.getFriendStatusWithCache(senderId, receiverId)
	if senderStatus < 0 {
		return &user.MessageValidateResp{
			Allowed:      false,
			RejectReason: common.NotFriend.Message,
			ErrorCode:    int32(common.NotFriend.Code),
		}, nil
	} else if senderStatus == 2 {
		return &user.MessageValidateResp{
			Allowed:      false,
			RejectReason: common.FriendDeleted.Message,
			ErrorCode:    int32(common.FriendDeleted.Code),
		}, nil
	}

	// 4. 校验接收者→发送者的状态（是否被拉黑）
	receiverStatus := l.getFriendStatusWithCache(receiverId, senderId)
	if receiverStatus < 0 {
		return &user.MessageValidateResp{
			Allowed:      false,
			RejectReason: common.NotFriend.Message,
			ErrorCode:    int32(common.NotFoundError.Code),
		}, nil
	}

	if receiverStatus == 1 {
		return &user.MessageValidateResp{
			Allowed:      false,
			RejectReason: common.BlockedByReceiver.Message,
			ErrorCode:    int32(common.BlockedByReceiver.Code),
		}, nil
	}

	return &user.MessageValidateResp{Allowed: true}, nil
}

//	带 Redis 缓存的好友状态查询
//
// 返回值：好友表的 status 字段（0=好友, 1=拉黑, 2=删除），-1 表示非好友
func (l *ValidateSingleMessageLogic) getFriendStatusWithCache(userId, friendId int64) int64 {
	// key 规则与 user-api 的清理逻辑共用同一份实现，避免"写的 key 和删的 key 不是一个"
	key := constants.FriendStatusCacheKey(userId, friendId)

	// 尝试从 Redis 缓存读取
	if l.svcCtx.Redis != nil {
		val, err := l.svcCtx.Redis.Get(key)
		if err == nil && val != "" {
			status, _ := strconv.ParseInt(val, 10, 64)
			return status
		}
	}

	var status int64
	// 缓存未命中，查数据库
	rel, err := l.svcCtx.FriendModel.FindOneByUserIdFriendId(l.ctx, userId, friendId)
	if err != nil {
		if !errors.Is(err, sqlx.ErrNotFound) {
			// 数据库故障：不落缓存（否则一次抖动就会让这对用户被永久判成非好友），本次按非好友保守处理
			common.Errorf("查询好友关系失败, userId=%d, friendId=%d: %v", userId, friendId, err)
		}
		status = -1
	} else if rel == nil {
		status = -1
	} else {
		status = rel.Status
	}

	if l.svcCtx.Redis != nil {
		// 必须带 TTL：go-zero 的 Set 不传过期时间就是永久缓存
		_ = l.svcCtx.Redis.Setex(key, strconv.FormatInt(status, 10), friendCacheTTLSeconds)
	}

	return status
}
