package offlinedata

import (
	"context"

	"go-infinitechat/common/common"
	"go-infinitechat/common/identity"
	"go-infinitechat/service/user/rpc/user"
	"go-infinitechat/service/user/rpc/userrpc"
)

// ensureSessionMember 返回当前登录用户 ID，并校验其属于该会话。
//
// 历史消息 / 离线消息这类"只凭 sessionId 取数"的接口，此前完全没有归属校验：
// 任何登录用户遍历 sessionId 就能读别人的聊天记录。这里补上：
//   - 未登录（直连服务、缺 X-User-Id）→ 抛 PleaseLogin
//   - 不是该会话成员 → 抛 NoAuthError
func ensureSessionMember(ctx context.Context, rpc userrpc.UserRpc, sessionId int64) int64 {
	userId, ok := identity.UserId(ctx)
	common.ThrowIf(!ok, common.PleaseLogin)

	_, err := rpc.GetUserSessionRole(ctx, &user.UserSessionRoleReq{SessionId: sessionId, UserId: userId})
	common.ThrowIfWithMsg(err != nil, common.NoAuthError, "您不在该会话中")
	return userId
}
