package service

import (
	"context"
	"go-infinitechat/common/common"
	"go-infinitechat/service/user/model/session"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type SessionService struct {
	SessionModel session.SessionModel
	rds          *redis.Redis
}

func NewSessionService(sessionService session.SessionModel, rds *redis.Redis) *SessionService {
	return &SessionService{
		SessionModel: sessionService,
		rds:          rds,
	}
}

// 创建会话
func (ss *SessionService) CreateSession(ctx context.Context, data *session.Session) int64 {
	res, err := ss.SessionModel.InsertTx(ctx, data)
	common.ThrowIfWithMsg(err != nil || res == nil, common.SystemError, "创建会话失败", err)
	id, _ := res.LastInsertId()
	return id
}
