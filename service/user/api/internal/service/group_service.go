package service

import (
	"context"
	"go-infinitechat/common/common"
	dto2 "go-infinitechat/common/model/dto"
	"go-infinitechat/common/model/txctx"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/user/model/friend"
	"go-infinitechat/service/user/model/session"
	"go-infinitechat/service/user/model/user_session"
	"strconv"
	"time"

	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/constants"
	"go-infinitechat/service/user/api/internal/types/dto"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	RoleGroupOwner     = 0
	RoleGroupAdmin     = 1
	RoleGroupMenber    = 2
	DefaultGroupAvatar = "https://video.shanyangcode.com/image/default/A9C9C83CCCE043EC8253DB5D7545DCB4-6-2.png"
)

type GroupService struct {
	UserService         *UserService
	FriendModel         friend.FriendModel
	SessionModel        session.SessionModel
	UserSessionModel    user_session.UserSessionModel
	NotificationService *NotificationService
	SqlConn             sqlx.SqlConn
}

func NewGroupService(
	userService *UserService,
	friendModel friend.FriendModel,
	sessionModel session.SessionModel,
	userSessionModel user_session.UserSessionModel,
	notificationService *NotificationService,
	conn sqlx.SqlConn,
) *GroupService {
	return &GroupService{
		UserService:         userService,
		FriendModel:         friendModel,
		SessionModel:        sessionModel,
		UserSessionModel:    userSessionModel,
		NotificationService: notificationService,
		SqlConn:             conn,
	}
}

func (gs *GroupService) GetGroupMenberCount(ctx context.Context, sessionId int64) int {
	common.ThrowIfWithMsg(sessionId <= 0, common.ParamsError, "会话ID不能为空")
	var count int
	query := "SELECT count(*) FROM `user_session` WHERE `session_id` = ? AND `status` = 0"
	err := gs.SqlConn.QueryRowCtx(ctx, &count, query, sessionId)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询群成员数量失败", err)
	return count
}

func (gs *GroupService) GetUserGroups(ctx context.Context, userId int64, pageNum, pageSize int32) *dto2.PageResponse[dto.UserGroupDTO] {
	common.ThrowIfWithMsg(userId <= 0, common.ParamsError, "用户ID不能为空")

	// 1. 查询总数
	var total int64
	countQuery := `SELECT COUNT(*) 
	FROM user_session us INNER JOIN session s ON us.session_id = s.session_id 
	WHERE us.user_id = ? AND us.status = 0 AND s.type = 1 `
	err := gs.SqlConn.QueryRowCtx(ctx, &total, countQuery, userId)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询用户群聊数量失败")

	// 空数据直接返回
	if total == 0 {
		return dto2.NewPageResponse([]dto.UserGroupDTO{}, int(total), int(pageNum), int(pageSize))
	}

	// 2.分页查询
	offset := (pageNum - 1) * pageSize
	var userSession []user_session.UserSession
	query := `SELECT 
	FROM user_session us INNER JOIN session s ON us.session_id = s.session_id
	WHERE us.user_id = ? AND us.status = 0 AND s.type = 1
	ORDER BY us.created_time DESC
	LIMIT ?, ?`
	err = gs.SqlConn.QueryRowsCtx(ctx, &userSession, query, userId, offset, pageSize)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询用户群聊列表失败")

	// 3.组装DTO
	var list []dto.UserGroupDTO
	for _, us := range userSession {
		sess, err := gs.SessionModel.FindOne(ctx, us.SessionId)
		if err != nil || sess == nil {
			continue
		}

		// 查群主
		creatorId := ""
		var ownerUserId int64
		_ = gs.SqlConn.QueryRowCtx(ctx, &ownerUserId, "select `user_id` from `user_session` where `session_id` = ? and `role` = 0 limit 1", us.SessionId)

		if ownerUserId > 0 {
			creatorId = strconv.FormatInt(ownerUserId, 10)
		}

		// 查群成员数
		var menberCount int
		_ = gs.SqlConn.QueryRowCtx(ctx, &menberCount, "select count(*) from `user_session` where `session_id` = ? and `status` = 0", us.SessionId)

		list = append(list, dto.UserGroupDTO{
			SessionId:   strconv.FormatInt(us.SessionId, 10),
			SessionName: sess.Name,
			Avatar:      sess.Avatar,
			CreatorId:   creatorId,
			Role:        us.Role,
			MemberCount: menberCount,
			CreatedTime: us.CreatedTime.Format("2006-01-02 15:04:05"),
		})
	}

	// 4. 构建分页响应
	return dto2.NewPageResponse(list, int(total), int(pageNum), int(pageSize))
}

func (gs *GroupService) generateGroupName(creatorId int64, memberIds []int64) string {
	allIds := append([]int64{creatorId}, memberIds...)
	var name string
	for _, id := range allIds {
		u, err := gs.UserService.GetUserById(id)
		if err != nil || u == nil {
			continue
		}
		if name != "" {
			name += "、"
		}
		name += u.Nickname.String + " "
		if len([]rune(name)) >= 16 {
			name = string([]rune(name)[:16])
			break
		}
	}
	return name
}

// 校验有效ID
func (gs *GroupService) filterFriends(ctx context.Context, userId int64, candidateIds []int64, failedStrIds *[]string) []int64 {
	var valid []int64
	for _, cid := range candidateIds {
		rel, err := gs.FriendModel.FindOneByUserIdFriendId(ctx, userId, cid)
		if err != nil || rel == nil || rel.Status != constants.Normal.Code {
			if failedStrIds != nil {
				*failedStrIds = append(*failedStrIds, strconv.FormatInt(cid, 10))
			}
			continue
		}
		valid = append(valid, cid)
	}
	return valid
}

func (gs *GroupService) CreateGroup(ctx context.Context, req *types.CreateGroupRequest) *dto.CreateGroupResponse {
	creatorId := req.CreatorId
	memberIds := req.MemberIds
	common.ThrowIfWithMsg(creatorId <= 0, common.ParamsError, "创建者ID不能为空")
	common.ThrowIfWithMsg(len(memberIds) == 0, common.ParamsError, "成员ID列表不能为空")

	// 验证创建者
	creator, err := gs.UserService.GetUserById(creatorId)
	common.ThrowIfWithMsg(err != nil || creator == nil || creator.State != 0, common.NotFoundError, "用户不存在或状态异常")

	// 验证好友关系，过滤无效成员
	var failedIds []string
	validIds := gs.filterFriends(ctx, creatorId, memberIds, &failedIds)
	common.ThrowIfWithMsg(len(validIds) == 0, common.OperationError, "没有有效的好友可加入群聊")

	sessionId := utils.NextInt()
	groupName := gs.generateGroupName(creatorId, validIds)
	membersCount := len(validIds) + 1

	// 事务开启：创建session + user_session
	err = txctx.WithTransaction(ctx, gs.SqlConn, func(ctx context.Context) error {
		gs.SessionModel.InsertTx(ctx, &session.Session{
			SessionId:   sessionId,
			Name:        groupName,
			Type:        constants.MessageType,
			Status:      constants.StatusSuccess,
			Avatar:      DefaultGroupAvatar,
			CreatedTime: time.Now(),
			UpdatedTime: time.Now(),
		})
		// 创建者 user_session(群主)
		gs.UserSessionModel.InsertTx(ctx, &user_session.UserSession{
			UserId:      creatorId,
			SessionId:   sessionId,
			Role:        RoleGroupOwner,
			Status:      constants.StatusSuccess,
			CreatedTime: time.Now(),
			UpdatedTime: time.Now(),
		})

		// 成员session
		for _, mid := range validIds {
			gs.UserSessionModel.InsertTx(ctx, &user_session.UserSession{
				UserId:      mid,
				SessionId:   sessionId,
				Role:        RoleGroupMenber,
				Status:      constants.StatusSuccess,
				CreatedTime: time.Now(),
				UpdatedTime: time.Now(),
			})
		}
		return nil
	})
	// 事务结束
	common.ThrowIfWithMsg(err != nil, common.SystemError, "创建群聊失败")

	// 推送通知
	notification := dto.NewGroupSessionNotificationDTO{
		SessionName: groupName, Avatar: DefaultGroupAvatar,
		CreatorId: creatorId, MembersCount: membersCount,
	}
	for _, mid := range validIds {
		defer func(uid int64) {
			defer func() {
				recover()
			}()
			if gs.NotificationService != nil {
				gs.NotificationService.PushGroupNewSession(ctx, uid, sessionId, notification)
			}
		}(mid)
	}
	return nil
}

func (gs *GroupService) InviteGroup(ctx context.Context, req *types.InviteGroupRequest) *types.InviteGroupResponse {
	sessionId := req.SessionId
	inviterId := req.InviterId
	inviteeIds := req.InviteeIds
	common.ThrowIfWithMsg(sessionId <= 0, common.ParamsError, "会话ID不能为空")
	common.ThrowIfWithMsg(inviterId <= 0, common.ParamsError, "邀请人ID不能为空")
	common.ThrowIfWithMsg(len(inviteeIds) == 0, common.ParamsError, "被邀请人ID列表不能为空")

	// 校验会话
	sess := gs.validateGroupSession(ctx, sessionId)
	// 校验权限
	gs.validatePermission(ctx, sessionId, inviterId)

	var failedIds []int64
	// 过滤好友
	validIds := gs.filterFriends(ctx, inviterId, inviteeIds, nil)
	// 过滤已在群内的
	validIds = gs.filterExistingMembers(ctx, sessionId, validIds, &failedIds)
	common.ThrowIfWithMsg(len(validIds) == 0, common.OperationError, "没有有效的好友可加入群聊")

	var successIds []int64
	for _, mid := range validIds {
		_, err := gs.UserSessionModel.InsertTx(ctx, &user_session.UserSession{
			UserId:      mid,
			SessionId:   sessionId,
			Role:        RoleGroupMenber,
			Status:      constants.StatusSuccess,
			CreatedTime: time.Now(),
			UpdatedTime: time.Now(),
		})
		if err != nil {
			failedIds = append(failedIds, mid)
			continue
		}
		successIds = append(successIds, mid)
		// 推送通知
		go func(uid int64) {
			defer func() { recover() }()
			if gs.NotificationService != nil {
				gs.NotificationService.PushGroupNewSession(ctx, uid, sessionId, dto.NewGroupSessionNotificationDTO{
					SessionName: sess.Name,
					Avatar:      sess.Avatar,
					CreatorId:   inviterId,
				})
			}
		}(mid)
	}

	return &types.InviteGroupResponse{SuccessIds: successIds, FailedIds: failedIds}
}

func (gs *GroupService) validateGroupSession(ctx context.Context, sessionId int64) *session.Session {
	sess, err := gs.SessionModel.FindOne(ctx, sessionId)
	common.ThrowIfWithMsg(err != nil || sess == nil, common.NotFoundError, "群聊不存在或已解散")
	common.ThrowIfWithMsg(sess.Type != constants.MessageType, common.ParamsError, "该会话不是群聊")
	return sess
}

func (gs *GroupService) validatePermission(ctx context.Context, sessionId, userId int64) {
	us, err := gs.UserSessionModel.FindOne(ctx, userId, sessionId)
	common.ThrowIfWithMsg(err != nil || us == nil, common.NoAuthError, "您不在该群聊中")
	common.ThrowIfWithMsg(us.Role != RoleGroupOwner && us.Role != RoleGroupAdmin, common.NoAuthError, "只有群主或管理员才能执行此操作")
}

func (gs *GroupService) filterExistingMembers(ctx context.Context, sessionId int64, candidateIds []int64, failedIds *[]int64) []int64 {
	var valid []int64
	for _, cid := range candidateIds {
		us, err := gs.UserSessionModel.FindOne(ctx, cid, sessionId)
		if err == nil && us != nil {
			if failedIds != nil {
				*failedIds = append(*failedIds, cid)
			}
			continue //已在群内
		}
		valid = append(valid, cid)
	}
	return valid
}

func (gs *GroupService) KickGroupMembers(ctx context.Context, req *types.KickGroupMembersRequest) *types.KickGroupMembersResponse {
	sessionId := req.SessionId
	operatorId := req.OperatorId
	memberIds := req.MemberIds

	gs.validateGroupSession(ctx, sessionId)

	opSession, err := gs.UserSessionModel.FindOne(ctx, operatorId, sessionId)
	common.ThrowIfWithMsg(err != nil || opSession == nil, common.NoAuthError, "您不在群聊中")
	opRole := opSession.Role

	var successIds []int64
	for _, mid := range memberIds {
		// 不能踢自己
		if mid == operatorId {
			continue
		}
		ms, err := gs.UserSessionModel.FindOne(ctx, mid, sessionId)
		if err != nil || ms == nil {
			continue
		}
		// 权限校验：群主可踢所有人，管理员只能踢普通成员
		if opRole == RoleGroupOwner || (opRole == RoleGroupAdmin && ms.Role == RoleGroupMenber) {
			err = gs.UserSessionModel.DeleteTx(ctx, mid, sessionId)
			if err != nil {
				continue
			}
			successIds = append(successIds, mid)
		}
	}

	// 推送踢人通知
	if len(successIds) > 0 && gs.NotificationService != nil {
		kickedIds := make([]int64, 0, len(successIds))

		for _, kid := range kickedIds {
			go func(u int64) {
				defer func() { recover() }()
				gs.NotificationService.PushGroupKickNotification(ctx, u, sessionId, dto.GroupKickNotificationDTO{
					MemberIds:  memberIds,
					OperatorId: operatorId,
				})
			}(kid)
		}
	}

	return &types.KickGroupMembersResponse{SuccessIds: successIds}
}

func (gs *GroupService) ExitGroup(ctx context.Context, req *types.GroupExitRequest) bool {
	sessionId := req.SessionId
	userId := req.UserId
	common.ThrowIfWithMsg(sessionId <= 0, common.ParamsError, "会话ID不能为空")
	common.ThrowIfWithMsg(userId <= 0, common.ParamsError, "用户ID不能为空")

	gs.validateGroupSession(ctx, sessionId)

	us, err := gs.UserSessionModel.FindOne(ctx, userId, sessionId)
	common.ThrowIfWithMsg(err != nil || us == nil, common.NotFoundError, "您不在该群聊中")
	// 群主不能退出（需要先转让）
	common.ThrowIfWithMsg(us.Role == RoleGroupOwner, common.OperationError, "群主不能退出群聊，请先转让群主")

	err = gs.UserSessionModel.DeleteTx(ctx, userId, sessionId)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "退出群聊失败")
	return true
}

func (gs *GroupService) GetGroupMenbers(ctx context.Context, sessionId int64, pageNum, pageSize int32) *dto2.PageResponse[dto.GroupMemberDTO] {
	common.ThrowIfWithMsg(sessionId <= 0, common.ParamsError, "会话ID不能为空")

	// 1. 查询总数
	var total int64
	err := gs.SqlConn.QueryRowCtx(ctx, &total, "select count(*) form `user_session where `session_id` = ? and `state` = 0`", sessionId)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询群成员数量失败")

	// 空数据直接返回
	if total <= 0 {
		return dto2.NewPageResponseFromAll([]dto.GroupMemberDTO{}, int(pageNum), int(pageSize))
	}

	// 2.分页查询
	offset := (pageNum - 1) * pageSize
	var userSession []user_session.UserSession
	query := "select `user_id`,`session_id`,`role`,`status`,`created_time`,`updated_time` from `user_session` where `session_id` = ? and `status` = 0 order by `role` asc, `created_time` asc limit ?, ?"
	err = gs.SqlConn.QueryRowCtx(ctx, &userSession, query, sessionId, offset, pageSize)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "查询群成员失败")

	// 3. 组装 DTO
	var list []dto.GroupMemberDTO
	for _, us := range userSession {
		u, err := gs.UserService.userModel.FindOne(ctx, us.UserId)
		if err != nil || u == nil {
			continue
		}
		list = append(list, dto.GroupMemberDTO{
			UserId:   strconv.FormatInt(u.UserId, 10),
			Nickname: u.Nickname.String,
			Avatar:   u.Avatar,
			Role:     u.Role,
		})
	}

	// 4. 使用 NewPageResponse 构建分页响应（自动计算 Pages/HasNext/HasPrevious）
	return dto2.NewPageResponseFromAll(list, int(pageNum), int(pageSize))
}
