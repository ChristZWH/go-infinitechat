package service

import (
	"context"
	"database/sql"
	"errors"
	"go-infinitechat/common/common"
	dto2 "go-infinitechat/common/model/dto"
	"go-infinitechat/common/model/txctx"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/constants"
	"go-infinitechat/service/user/api/internal/types/dto"
	"go-infinitechat/service/user/model/apply_friend"
	"go-infinitechat/service/user/model/friend"
	"go-infinitechat/service/user/model/session"
	"go-infinitechat/service/user/model/user"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type FriendService struct {
	IUserService        *UserService
	ISessionService     *SessionService
	IUserSessionService *UserSessionService
	FriendModel         friend.FriendModel
	ApplyFriendModel    apply_friend.ApplyFriendModel
	rds                 *redis.Redis
	SqlConn             sqlx.SqlConn // 用于事务
	NotificationService *NotificationService
}

func NewFriendService(iUserService *UserService, iSessionService *SessionService, iUserSessionService *UserSessionService, notificationService *NotificationService, friendModel friend.FriendModel, applyFriendModel apply_friend.ApplyFriendModel, conn sqlx.SqlConn, rds *redis.Redis) *FriendService {
	return &FriendService{
		IUserService:        iUserService,
		ISessionService:     iSessionService,
		IUserSessionService: iUserSessionService,
		FriendModel:         friendModel,
		rds:                 rds,
		SqlConn:             conn,
		ApplyFriendModel:    applyFriendModel,
		NotificationService: notificationService,
	}
}

// 添加好友关系
//
//	处理流程：
//	1. 验证用户存在性
//	2. 检查是否已是好友关系
//	3. 创建双向好友关系
//	4. 创建会话和用户会话关系
//	5. 发送Kafka通知
//	  recipient 接收好友请求的用户 【to】
//	  userId    申请添加的好友ID 【from】
//	  ModifyFriendApplicationResponse 响应对象
func (fs FriendService) AddFriend(ctx context.Context, recipient *user.User, friendId int64) dto.ModifyFriendApplicationResponse {
	common.ThrowIfWithMsg(friendId <= 0, common.ParamsError, "好友申请的发起者用户ID不存在")
	common.ThrowIfWithMsg(recipient.UserId <= 0, common.NotFoundError, "好友申请的接收者用户ID不存在")

	applicant, err1 := fs.IUserService.GetUserById(friendId) // 查询申请者
	common.ThrowIfWithMsg(err1 != nil && errors.Is(err1, sqlx.ErrNotFound), common.NotFoundError, "获取好友申请者信息失败", err1)
	common.ThrowIfWithMsg(applicant == nil, common.NotFoundError, "好友申请者不存在")

	// 检查是否已是好朋友
	friendRelation, err2 := fs.FriendModel.FindOneByUserIdFriendId(ctx, friendId, recipient.UserId)
	common.ThrowIfWithMsg(err2 != nil && !errors.Is(err2, friend.ErrNotFound), common.SystemError, "获取好友关系失败")
	common.ThrowIfWithMsg(friendRelation != nil, common.OperationError, "已经是好友关系")

	// 开启事务
	sessionId := utils.NextInt()
	err := txctx.WithTransaction(ctx, fs.SqlConn, func(ctx context.Context) error {
		// 创建双向好友关系
		fs.createFriendRelations(ctx, friendId, recipient.UserId)
		// 创建会话
		fs.ISessionService.CreateSession(ctx, &session.Session{
			SessionId:   sessionId,
			Name:        "",
			Type:        constants.SignalType,
			Status:      constants.StatusSuccess,
			CreatedTime: time.Now(),
			UpdatedTime: time.Now(),
		})
		// 创建用户会话关系
		fs.IUserSessionService.CreateUserSession(ctx, friendId, recipient.UserId, sessionId)

		return nil
	})
	// 结束事务
	common.ThrowIfWithMsg(err != nil, common.SystemError, "添加好友失败", err)

	// 发送给 Kafka 通知申请者
	fs.sendNewSessionNotification(ctx, friendId, recipient, sessionId)

	// 构建响应对象 返回给接收者
	return dto.ModifyFriendApplicationResponse{
		UserId:      strconv.FormatInt(applicant.UserId, 10), // 申请者ID
		SessionId:   strconv.FormatInt(sessionId, 10),
		SessionType: constants.SignalType,
		SessionName: applicant.Nickname.String, // 申请者名字
		Avatar:      applicant.Avatar,          // 申请者头像
	}
}

// 发送新会话通知给申请者
// 好友申请被通过后，通知申请者有新的会话创建
// 通知内容包含接收者（recipient）的昵称和头像，用于申请者的聊天列表展示
func (fs FriendService) sendNewSessionNotification(ctx context.Context, friendId int64, recipient *user.User, sessionId int64) {
	if fs.NotificationService == nil {
		common.Errorf("Notification 未初始化，跳过发送新绘画通知")
		return
	}

	notification := dto.NewSessionNotificationDTO{
		SessionName: recipient.Nickname.String,
		Avatar:      recipient.Avatar,
	}

	// goroutine 异步发送
	go func() {
		defer func() {
			if err := recover(); err != nil {
				common.Errorf("发送新会话通知异常，申请者ID: %d, 会话ID: %d, 错误: %v", friendId, sessionId, err)
			}
		}()

		fs.NotificationService.PushNewSession(
			ctx,
			recipient.UserId,            // senderID
			friendId,                    // userId (接收通知的人 也是申请好友的人)
			sessionId,                   // 新会话ID
			constants.SessionTypeSignal, // 单聊
			notification,
		)

		common.Infof("发送新会话通知成功, 申请者ID: %d, 会话ID: %d", friendId, sessionId)
	}()
}

// 创建双向好友关系
// userId 当前用户ID
// friendId 好友用户ID
func (fs FriendService) createFriendRelations(ctx context.Context, userId, friendId int64) {
	friend1 := &friend.Friend{
		UserId:      userId,
		FriendId:    friendId,
		Status:      constants.Normal.Code,
		CreatedTime: time.Now(),
		UpdatedTime: time.Now(),
	}
	friend2 := &friend.Friend{
		UserId:      friendId,
		FriendId:    userId,
		Status:      constants.Normal.Code,
		CreatedTime: time.Now(),
		UpdatedTime: time.Now(),
	}
	err := txctx.WithTransaction(ctx, fs.SqlConn, func(ctx context.Context) error {
		_, err1 := fs.FriendModel.InsertTx(ctx, friend1)
		_, err2 := fs.FriendModel.InsertTx(ctx, friend2)
		if err1 != nil {
			return err1
		}
		if err2 != nil {
			return err2
		}
		return nil
	})
	common.ThrowIfWithMsg(err != nil, common.SystemError, "添加好友关系失败", err)
}

// 根据关键字搜索用户（手机号或邮箱）
func (fs *FriendService) SearchUserByKey(ctx context.Context, userId string, keyword string) *types.FriendDetailVO {
	common.ThrowIfWithMsg(keyword == "", common.ParamsError, "搜索关键字不能未空")

	var foundUser *user.User
	var err error

	if constants.PhoneRegex.MatchString(keyword) {
		foundUser, err = fs.IUserService.userModel.FindOneByPhone(ctx, sql.NullString{
			String: keyword, Valid: true})
	} else if constants.EmailRegex.MatchString(keyword) {
		foundUser, err = fs.IUserService.userModel.FindOneByEmail(ctx, sql.NullString{
			String: keyword, Valid: true})
	} else {
		common.ThrowIfWithMsg(true, common.ParamsError, "请输入有效的手机号或邮箱")
	}

	common.ThrowIfWithMsg(err != nil && errors.Is(err, sqlx.ErrNotFound), common.NotFoundError, "搜索用户失败", err)
	common.ThrowIfWithMsg(foundUser == nil, common.NotFoundError, "查询用户不存在")

	return fs.GetFriendDetails(ctx, userId, strconv.FormatInt(foundUser.UserId, 10))
}

// 获取好友详细信息
func (fs *FriendService) GetFriendDetails(ctx context.Context, userId string, friendId string) *types.FriendDetailVO {
	uid, err := strconv.ParseInt(userId, 10, 64)
	common.ThrowIfWithMsg(err != nil, common.ParamsError, "用户ID格式错误")

	fid, err := strconv.ParseInt(friendId, 10, 64)
	common.ThrowIfWithMsg(err != nil, common.ParamsError, "好友ID格式错误")

	// 1.获取好友用户信息
	friendUser, err := fs.IUserService.GetUserById(fid)
	common.ThrowIfWithMsg(err != nil && errors.Is(err, sqlx.ErrNotFound), common.NotFoundError, "查询用户信息失败", err)
	common.ThrowIfWithMsg(friendUser == nil, common.NotFoundError, "用户不存在")

	// 检查用户状态
	if friendUser.State == 1 {
		common.ThrowIfWithMsg(true, common.ForbiddenError, "该用户已被封禁")
	} else if friendUser.State == 2 {
		common.ThrowIfWithMsg(true, common.NotFoundError, "该用户已注销")
	}

	// 2.构建详情VO
	vo := &types.FriendDetailVO{
		UserId:    friendId,
		Nickname:  nullToEmpty(friendUser.Nickname),
		Avatar:    friendUser.Avatar,
		Email:     nullToEmpty(friendUser.Email),
		Phone:     nullToEmpty(friendUser.Phone),
		Signature: nullToEmpty(friendUser.Description),
		Gender:    int32(friendUser.Gender),
	}

	// 3.填充会话ID
	vo.SessionId = fs.findSessionIdBetweenUsers(ctx, uid, fid)

	// 4.填充好友状态
	friendRelation, err := fs.FriendModel.FindOneByUserIdFriendId(ctx, uid, fid)
	if err != nil && friendRelation != nil {
		vo.Status = int32(friendRelation.Status)
	} else {
		vo.Status = -1
	}

	return vo
}

func nullToEmpty(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

// 查找两个好友之间的会话状态
func (fs *FriendService) findSessionIdBetweenUsers(ctx context.Context, userId, friendId int64) string {
	conn := fs.SqlConn
	var sessionId int64
	// 查找的是 同时装着你和你朋友的会话ID
	query := `SELECT us1.session_id FROM user_session us1
	INNER JOIN user_session us2 ON us1.session_id = us2_session_id
	INNER JOIN session s ON us1.session_id = s.session_id 
	WHERE us1.user_id = ? AND us2.user_id = ? AND s.type = 0
	LIMIT 1`
	err := conn.QueryRowCtx(ctx, &sessionId, query, userId, friendId)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(sessionId, 10)
}

// 获取好友列表
func (fs *FriendService) GetFriends(ctx context.Context, userId string, pageNum, pageSize int32, key string) *dto2.PageResponse[dto.FriendDTO] {
	uid, err := strconv.ParseInt(userId, 10, 64)
	common.ThrowIfWithMsg(err != nil, common.ParamsError, "用户ID格式错误")
	common.ThrowIfWithMsg(uid <= 0, common.ParamsError, "用户ID无效")

	// 1. 查询好友关系列表
	friendList, err := fs.FriendModel.FindListByUserIdTx(ctx, uid)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询好友列表失败", err)

	// 空数据时用 NewPageResponseFromAll（自动填充所有字段）
	if len(friendList) == 0 {
		return dto2.NewPageResponseFromAll([]dto.FriendDTO{}, int(pageNum), int(pageSize))
	}

	// 2. 获取好友ID列表
	friendIds := make([]int64, 0, len(friendList))
	friendRelationMap := make(map[int64]*friend.Friend)
	for _, f := range friendList {
		friendIds = append(friendIds, f.FriendId)
		friendRelationMap[f.FriendId] = f
	}

	// 3. 查询好友用户信息
	var friendDTOList []dto.FriendDTO
	for _, fid := range friendIds {
		u, err := fs.IUserService.GetUserById(fid)
		if u == nil || err != nil {
			continue
		}

		if key != "" {
			nickName := nullToEmpty(u.Nickname)
			phone := nullToEmpty(u.Phone)
			uidStr := strconv.FormatInt(uid, 10)
			if !strings.Contains(nickName, key) && strings.Contains(phone, key) && strings.Contains(uidStr, key) {
				continue
			}
		}

		rel := friendRelationMap[fid]
		status := int64(-1)
		if rel != nil {
			status = rel.Status
		}

		// 查会话ID
		sessionId := fs.findSessionIdBetweenUsers(ctx, uid, fid)

		friendDTOList = append(friendDTOList, dto.FriendDTO{
			UserId:    strconv.FormatInt(u.UserId, 10),
			Nickname:  nullToEmpty(u.Nickname),
			Avatar:    u.Avatar,
			Signature: u.Description.String,
			Status:    status,
			SessionId: sessionId,
		})

	}
	return dto2.NewPageResponseFromAll(friendDTOList, int(pageNum), int(pageSize))
}

// 删除好友
func (fs *FriendService) DeleteFriend(ctx context.Context, userId, friendId int64) bool {
	common.ThrowIfWithMsg(userId <= 0, common.ParamsError, "用户ID无效")
	common.ThrowIfWithMsg(friendId <= 0, common.ParamsError, "好友ID无效")

	err := txctx.WithTransaction(ctx, fs.SqlConn, func(ctx context.Context) error {
		// 1. 双向删除好友申请记录
		if err := fs.ApplyFriendModel.DeleteByBothDirectionsTx(ctx, userId, friendId); err != nil {
			return err
		}
		// 2. 双向删除好友关系
		if err := fs.FriendModel.DeleteByBothDirectionsTx(ctx, userId, friendId); err != nil {
			return err
		}
		// 3. 删除会话记录（查找并删除共同的单聊会话）
		fs.deleteSessionRecords(ctx, userId, friendId)

		return nil
	})
	common.ThrowIfWithMsg(err != nil, common.SystemError, "删除好友失败")

	// 4. 清除好友关系缓存
	fs.evictFriendCache(userId, friendId)
	return true
}

// 清除双向好友关系缓存
func (fs *FriendService) evictFriendCache(userId, friendId int64) {
	if fs.rds == nil {
		return
	}
	key1 := constants.FriendStatusKeyPrefix + strconv.FormatInt(userId, 10) + ":" + strconv.FormatInt(friendId, 10)
	key2 := constants.FriendStatusKeyPrefix + strconv.FormatInt(friendId, 10) + ":" + strconv.FormatInt(userId, 10)
	_, _ = fs.rds.Del(key1)
	_, _ = fs.rds.Del(key2)
	common.Infof("已清除好友关系缓存: %d <-> %d", userId, friendId)
}

// 删除两个用户间的单聊会话记录
func (fs *FriendService) deleteSessionRecords(ctx context.Context, userId, friendId int64) {
	conn := fs.SqlConn
	//  查找共同的单聊会话ID
	var sessionIds []int64
	query := `SELECT us1.session_id FROM user_session us1
	INNER JOIN user_session us2 ON us1.session_id = us2.session_id
	INNER JOIN session s ON us1.session_id = s.session_id
	WHERE us1.user_id = ? AND us2.user_id = ? AND s.type = 0`

	if session := txctx.GetSession(ctx); session != nil {
		_ = session.QueryRowCtx(ctx, &sessionIds, query, userId, friendId)
		if len(sessionIds) > 0 {
			for _, sid := range sessionIds {
				_, _ = session.ExecCtx(ctx, "delete from `user_session` where `session_id` = ?", sid)
				_, _ = session.ExecCtx(ctx, "delete from `session` where `session_id` = ?", sid)
			}
		}
	} else {
		_ = conn.QueryRowCtx(ctx, &sessionIds, query, userId, friendId)
		if len(sessionIds) > 0 {
			for _, sid := range sessionIds {
				_, _ = session.ExecCtx(ctx, "delete from `user_session` where `session_id` = ?", sid)
				_, _ = session.ExecCtx(ctx, "delete from `session` where `session_id` = ?", sid)
			}
		}
	}
}

// 取消拉黑
func (fs *FriendService) UnblockFriend(ctx context.Context, userId, friendId int64) bool {
	common.ThrowIfWithMsg(userId <= 0, common.ParamsError, "用户ID无效")
	common.ThrowIfWithMsg(friendId <= 0, common.ParamsError, "好友ID无效")

	// 1.检查好友关系
	friendRelation, err := fs.FriendModel.FindOneByUserIdFriendId(ctx, userId, friendId)
	common.ThrowIfWithMsg(err != nil && !errors.Is(err, friend.ErrNotFound), common.SystemError, "查询好友关系失败")
	common.ThrowIfWithMsg(friendRelation == nil, common.NotFoundError, "好友关系不存在")
	common.ThrowIfWithMsg(friendRelation.UserId != userId, common.NoAuthError, "无权限操作")
	common.ThrowIfWithMsg(friendRelation.Status != constants.Blocked.Code, common.OperationError, "该好友未被拉黑")

	// 2. 更新状态为正常
	err = fs.FriendModel.UpdateStatusByUserIdFriendId(ctx, userId, friendId, constants.Normal.Code)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "取消拉黑失败")

	// 3. 清缓存
	fs.evictFriendCache(userId, friendId)
	return true
}

// 拉黑好友
func (fs *FriendService) BlockFriend(ctx context.Context, userId, friendId int64) bool {
	common.ThrowIfWithMsg(userId <= 0, common.ParamsError, "用户ID无效")
	common.ThrowIfWithMsg(friendId <= 0, common.ParamsError, "好友ID无效")

	// 1. 检查好友关系
	friendRelation, err := fs.FriendModel.FindOneByUserIdFriendId(ctx, userId, friendId)
	common.ThrowIfWithMsg(err != nil && !errors.Is(err, friend.ErrNotFound), common.SystemError, "查询好友关系失败")
	common.ThrowIfWithMsg(friendRelation == nil, common.NotFoundError, "好友关系不存在")
	common.ThrowIfWithMsg(friendRelation.UserId != userId, common.NoAuthError, "无权操作")

	// 2. 更新状态为拉黑
	err = fs.FriendModel.UpdateStatusByUserIdFriendId(ctx, userId, friendId, constants.Blocked.Code)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "拉黑好友失败")

	// 3. 清缓存
	fs.evictFriendCache(userId, friendId)
	return true
}
