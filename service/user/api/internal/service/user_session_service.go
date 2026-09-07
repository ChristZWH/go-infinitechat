package service

import (
	"context"
	"go-infinitechat/common/common"
	"go-infinitechat/common/model/txctx"
	"go-infinitechat/service/user/api/internal/types/constants"
	"go-infinitechat/service/user/model/user_session"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type UserSessionService struct {
	UserSessionModel user_session.UserSessionModel
	rds              *redis.Redis
	SqlConn          sqlx.SqlConn
}

func NewUserSessionService(userSession user_session.UserSessionModel, rds *redis.Redis, conn sqlx.SqlConn) *UserSessionService {
	return &UserSessionService{
		UserSessionModel: userSession,
		rds:              rds,
		SqlConn:          conn,
	}
}

/**
 * 创建用户会话关系
 *
 * userId    当前用户ID
 * friendId  好友ID
 * sessionId 会话ID
 */
func (ss *UserSessionService) CreateUserSession(ctx context.Context, userId, friendId, sessionId int64) {
	err := txctx.WithTransaction(ctx, ss.SqlConn, func(ctx context.Context) error {
		_, err1 := ss.UserSessionModel.InsertTx(ctx, &user_session.UserSession{
			UserId:      userId,
			SessionId:   sessionId,
			Role:        constants.UserRoleNormal, //角色：0 群主，1 管理员，2 普通用户
			Status:      constants.StatusSuccess,  // 0 正常，1 删除
			CreatedTime: time.Now(),
			UpdatedTime: time.Now(),
		})
		if err1 != nil {
			return err1
		}
		_, err2 := ss.UserSessionModel.InsertTx(ctx, &user_session.UserSession{
			UserId:      friendId,
			SessionId:   sessionId,
			Role:        constants.UserRoleNormal, //角色：0 群主，1 管理员，2 普通用户
			CreatedTime: time.Now(),
			UpdatedTime: time.Now(),
		})
		if err2 != nil {
			return err2
		}
		return nil
	})
	common.ThrowIfWithMsg(err != nil, common.SystemError, "创建用户会话关系失败", err)
}
