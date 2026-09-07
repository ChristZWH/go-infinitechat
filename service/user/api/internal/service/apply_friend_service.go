package service

import (
	"context"
	"errors"
	"go-infinitechat/common/common"
	dto2 "go-infinitechat/common/model/dto"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/user/api/internal/types/constants"
	"go-infinitechat/service/user/api/internal/types/dto"
	"go-infinitechat/service/user/model/apply_friend"
	"go-infinitechat/service/user/model/friend"
	"go-infinitechat/service/user/model/user"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ApplyFriendService struct {
	ApplyFriendModel    apply_friend.ApplyFriendModel
	FriendModel         friend.FriendModel
	FriendService       *FriendService
	UserService         *UserService
	NotificationService *NotificationService
	rds                 *redis.Redis
	SqlConn             sqlx.SqlConn
}

func NewApplyFriendService(
	applyFriendModel apply_friend.ApplyFriendModel,
	friendModel friend.FriendModel,
	friendService *FriendService,
	userService *UserService,
	notificationService *NotificationService,
	rds *redis.Redis,
	conn sqlx.SqlConn,
) *ApplyFriendService {
	return &ApplyFriendService{
		ApplyFriendModel:    applyFriendModel,
		FriendModel:         friendModel,
		FriendService:       friendService,
		UserService:         userService,
		NotificationService: notificationService,
		rds:                 rds,
		SqlConn:             conn,
	}
}

// 发送好友申请
func (s *ApplyFriendService) SendFriendRequest(ctx context.Context, senderId, receiverId int64, message string) int64 {
	common.ThrowIfWithMsg(senderId <= 0, common.ParamsError, "发送者ID无效")
	common.ThrowIfWithMsg(receiverId <= 0, common.ParamsError, "接收者ID无效")
	common.ThrowIfWithMsg(senderId == receiverId, common.ParamsError, "不能添加自己为好友")

	// 1. 验证发送者和接收者
	sender, err := s.UserService.GetUserById(senderId)
	common.ThrowIfWithMsg(err != nil || sender == nil, common.NotFoundError, "发送者用户不存在")

	receiver, err := s.UserService.GetUserById(receiverId)
	common.ThrowIfWithMsg(err != nil || receiver == nil, common.NotFoundError, "接收者用户不存在")

	// 2. 检查是否已是好友
	existFriend, err := s.FriendModel.FindOneByUserIdFriendId(ctx, senderId, receiverId)
	common.ThrowIfWithMsg(err == nil && existFriend != nil, common.OperationError, "已经是好友关系")

	// 3. 检查是否已有待处理的申请，有则更新
	existApply, err := s.ApplyFriendModel.FindOneBySenderIdReceiverId(ctx, senderId, receiverId)
	if err == nil && existFriend != nil {
		if existApply.Status == constants.ApplicationUnread.Code || existApply.Status == constants.ApplicationRead.Code {
			// 更新已有申请
			existApply.Message = message
			existApply.Status = constants.ApplicationUnread.Code
			existApply.UpdatedTime = time.Now()
			err = s.ApplyFriendModel.Update(ctx, existApply)
			common.ThrowIfWithMsg(err != nil, common.SystemError, "更新好友申请失败")

			// 发送通知
			s.sendApplyNotification(senderId, receiverId, sender, message)
			s.sendExpirationEvent(senderId)
		}
	}

	// 4. 创建新申请
	applyFriendId := utils.NextInt()
	newApply := &apply_friend.ApplyFriend{
		ApplyFriendId: applyFriendId,
		SenderId:      senderId,
		ReceiverId:    receiverId,
		Message:       message,
		Status:        constants.ApplicationUnread.Code,
		CreatedTime:   time.Now(),
		UpdatedTime:   time.Now(),
	}

	_, err = s.ApplyFriendModel.InsertTx(ctx, newApply)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "创建好友申请失败")

	// 5. 发送通知
	s.sendApplyNotification(senderId, receiverId, sender, message)
	s.sendExpirationEvent(applyFriendId)

	return applyFriendId
}

// 获取好友申请列表（分页，含用户信息）
func (s *ApplyFriendService) GetReceivedRequestsWithUserInfo(ctx context.Context, userId int64, pageNum, pageSize int32) *dto2.PageResponse[dto.ApplyFriendDTO] {
	common.ThrowIfWithMsg(userId <= 0, common.ParamsError, "用户ID无效")

	// 1. 分页查询
	applyList, total, err := s.ApplyFriendModel.FindPageByUserIdWithPagination(ctx, userId, pageNum, pageSize)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询好友申请列表失败", err)

	// 2. 转换为 DTO
	dtoList := make([]dto.ApplyFriendDTO, 0, len(applyList))
	for _, af := range applyList {
		d := dto.ApplyFriendDTO{
			Msg:    af.Message,
			Status: af.Status,
			Time:   af.UpdatedTime.Format("2006-01-02 15:04:05"),
		}

		if af.SenderId == userId {
			receiverUser, err := s.UserService.GetUserById(af.ReceiverId)
			if err == nil && receiverUser != nil {
				d.UserId = strconv.FormatInt(receiverUser.UserId, 10)
				d.Nickname = receiverUser.Nickname.String
				d.Avatar = receiverUser.Avatar
				d.IsReceiver = 0
			}
		} else {
			senderUser, err := s.UserService.GetUserById(af.SenderId)
			if err == nil && senderUser != nil {
				d.UserId = strconv.FormatInt(senderUser.UserId, 10)
				d.Nickname = senderUser.Nickname.String
				d.Avatar = senderUser.Avatar
				d.IsReceiver = 1
			}
		}

		dtoList = append(dtoList, d)
	}

	//使用 NewPageResponse，自动计算 Pages/HasNext/HasPrevious
	return dto2.NewPageResponse(dtoList, int(total), int(pageNum), int(pageSize))
}

// 获取未读好友申请数量
func (s *ApplyFriendService) GetUnreadCount(ctx context.Context, userId int64) int64 {
	common.ThrowIfWithMsg(userId <= 0, common.ParamsError, "用户ID无效")

	// 我作为接收者，我没读的好友申请数量
	count, err := s.ApplyFriendModel.CountByReceiverAndStatus(ctx, userId, constants.ApplicationUnread.Code)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询未读数量失败")

	return count
}

// 修改好友申请状态
//
//	status: 1=通过, 2=拒绝, 3=已读
func (s *ApplyFriendService) ModifyApplicationStatus(ctx context.Context, receiverId int64, senderIds []int64, status int64) interface{} {
	common.ThrowIfWithMsg(receiverId <= 0, common.ParamsError, "接收者ID不能为空")
	common.ThrowIfWithMsg(len(senderIds) == 0, common.ParamsError, "发送者ID列表不能为空")

	switch status {
	case constants.ApplicationAccepted.Code:
		// 通过申请 - 只处理一个
		return s.handleAcceptApplication(ctx, receiverId, senderIds[0])
	case constants.ApplicationRejected.Code:
		// 拒绝申请
		_, err := s.ApplyFriendModel.UpdateStatusBySenderIdsAndReceiverId(ctx, receiverId, senderIds, status, constants.ApplicationRejected.Code)
		common.ThrowIfWithMsg(err != nil, common.SystemError, "拒绝好友申请失败")
		return true
	case constants.ApplicationRead.Code:
		_, err := s.ApplyFriendModel.UpdateStatusBySenderIdsAndReceiverId(ctx, receiverId, senderIds, status, constants.ApplicationRead.Code)
		common.ThrowIfWithMsg(err != nil, common.SystemError, "标记已读失败")
		return true
	default:
		common.ThrowIfWithMsg(true, common.ParamsError, "不允许修改为该状态值")
		return nil
	}
}

// handleAcceptApplication 处理通过好友申请
func (s *ApplyFriendService) handleAcceptApplication(ctx context.Context, receiverId int64, senderId int64) *dto.ModifyFriendApplicationResponse {
	common.ThrowIfWithMsg(senderId < 0, common.ParamsError, "发送者ID不能为空")

	// 1. 查找申请记录
	applyRecord, err := s.ApplyFriendModel.FindOneBySenderIdReceiverId(ctx, senderId, receiverId)
	common.ThrowIfWithMsg(err != nil && !errors.Is(err, apply_friend.ErrNotFound), common.SystemError, "查询好友申请记录失败")
	common.ThrowIfWithMsg(applyRecord == nil, common.NotFoundError, "好友申请不存在")

	// 2.验证状态
	common.ThrowIfWithMsg(applyRecord.Status != constants.ApplicationUnread.Code && applyRecord.Status != constants.ApplicationRead.Code, common.OperationError, "该申请不可被处理")

	// 3.更新状态为通过
	applyRecord.Status = constants.ApplicationAccepted.Code
	applyRecord.UpdatedTime = time.Now()
	err = s.ApplyFriendModel.UpdateTx(ctx, applyRecord)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "更新申请状态失败")

	// 4.创建好友关系和会话
	receiver, err := s.UserService.GetUserById(receiverId)
	common.ThrowIfWithMsg(err != nil || receiver == nil, common.NotFoundError, "接收者用户不存在")

	resp := s.FriendService.AddFriend(ctx, receiver, senderId)
	return &resp
}

// 发送好友申请通知
func (s *ApplyFriendService) sendApplyNotification(senderId, receiverId int64, sender *user.User, message string) {
	if s.NotificationService == nil || s.NotificationService.PusherManager == nil {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				common.Errorf("发送好友申请通知异常：%v", r)
			}
		}()

		notification := dto.FriendApplicationNotificationDTO{
			ApplyUserName:     sender.Nickname.String,
			ApplyUserId:       senderId,
			ApplyFriendAvatar: sender.Avatar,
			Message:           message,
		}

		s.NotificationService.PushNewApply(context.Background(), receiverId, notification)
		common.Infof("好友申请通知发送成功，发送者: %d, 接收者: %d", senderId, receiverId)
	}()
}

// sendExpirationEvent 发送好友申请过期事件
func (s *ApplyFriendService) sendExpirationEvent(applyFriendId int64) {
	if s.NotificationService == nil || s.NotificationService.PusherManager == nil {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				common.Errorf("发送好友申请过期事件异常: %v", r)
			}
		}()

		createTime := time.Now().UnixMilli()
		expireTime := createTime + constants.FriendRequestExpirationMillis

		event := dto.FriendRequestCreationEvent{
			ApplyFriendId: applyFriendId,
			CreateTime:    createTime,
			ExpireTime:    expireTime,
		}

		err := s.NotificationService.PusherManager.Push(context.Background(), constants.KafkaFriendRequestCreationTopic, event)
		common.ThrowIf(err != nil, common.SystemError, err)

		common.Infof("注册好友申请过期任务成功，申请ID: %d", applyFriendId)
	}()
}
