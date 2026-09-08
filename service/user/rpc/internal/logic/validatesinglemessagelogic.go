package logic

import (
	"context"
	"strconv"

	"go-infinitechat/common/common"
	"go-infinitechat/service/user/rpc/internal/svc"
	"go-infinitechat/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	friendStatusCachePrefix = "msg:validate:friend:status:"
	friendCacheTTL          = 120 // 2分钟
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
	if err != nil || sender == nil || sender.State == 0 {
		return &user.MessageValidateResp{
			Allowed:      false,
			RejectReason: common.SenderDisabled.Message,
			ErrorCode:    int32(common.SenderDisabled.Code),
		}, nil
	}

	// 2. 校验接收者用户状态
	receiver, err := l.svcCtx.UserModel.FindOne(l.ctx, receiverId)
	if err != nil || receiver == nil || sender.State == 0 {
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
			Allowed: false, RejectReason: "NOT_FRIEND", ErrorCode: 91002,
		}, nil
	}

	if receiverStatus == 1 {
		return &user.MessageValidateResp{
			Allowed: false, RejectReason: "BLOCKED_BY_RECEIVER", ErrorCode: 91003,
		}, nil
	}

	return &user.MessageValidateResp{Allowed: true}, nil
}

//	带 Redis 缓存的好友状态查询
//
// 返回值：好友表的 status 字段（0=好友, 1=拉黑, 2=删除），-1 表示非好友
func (l *ValidateSingleMessageLogic) getFriendStatusWithCache(userId, friendId int64) int64 {
	key := friendStatusCachePrefix + strconv.FormatInt(userId, 10) + strconv.FormatInt(friendId, 10)

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
	if err != nil || rel == nil {
		status = -1
	} else {
		status = rel.Status
	}

	if l.svcCtx.Redis != nil {
		_ = l.svcCtx.Redis.Set(key, strconv.FormatInt(status, 10))
	}

	return status
}
