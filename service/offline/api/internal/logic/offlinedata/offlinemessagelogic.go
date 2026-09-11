// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package offlinedata

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-infinitechat/common/common"
	"go-infinitechat/common/model/constants"
	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"
	"go-infinitechat/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type OfflineMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOfflineMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OfflineMessageLogic {
	return &OfflineMessageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OfflineMessageLogic) OfflineMessage(req *types.OfflineMessageRequest) (resp []types.OfflineHistoryMessageResponse, err error) {
	if req.SessionId <= 0 || req.UnreadOfflineCount <= 0 {
		return []types.OfflineHistoryMessageResponse{}, nil
	}

	// 先从 Redis 中拉数据（只存了七天内的热消息）
	result := []types.OfflineHistoryMessageResponse{}
	redisMessages := GetOfflineMessagesFromRedis(l.svcCtx.Redis, req.SessionId, req.UnreadOfflineCount)
	if len(redisMessages) > 0 {
		result = append(result, redisMessages...)
	}

	// 不够 N 条？再从 Mysql 补差额
	hotBoundary := time.Now().UnixMilli() - constants.SevenDaysMillis
	if len(result) < int(req.UnreadOfflineCount) {
		remaining := int(req.UnreadOfflineCount) - len(result)
		mysqlMessage := GetHistoryMessagesFromMysql(l.svcCtx, l.ctx, req.SessionId, hotBoundary, remaining)
		result = append(result, mysqlMessage...)
	}

	return result, nil
}

// 拉取离线消息
func GetOfflineMessagesFromRedis(redis *redis.Redis, sessionId int64, unreadOfflineCount int32) []types.OfflineHistoryMessageResponse {
	sessionMessagesKey := fmt.Sprintf("session:%d", sessionId)
	messageJsonSet, err := redis.Zrevrange(sessionMessagesKey, 0, int64(unreadOfflineCount-1))
	if err != nil {
		common.Errorf("拉取离线消息失败: sessionId=%d, err=%s", sessionId, err.Error())
		return []types.OfflineHistoryMessageResponse{}
	}
	return ParseMessagesFromJSON(messageJsonSet)
}

// 将 Redis 返回的消息 JSON 字符串列表解析为响应结构体列表
// （供 GetOfflineMessagesFromRedis / getHistoryMessageFromRedis 共用，本函数不访问 Redis）
func ParseMessagesFromJSON(messageJsonSet []string) []types.OfflineHistoryMessageResponse {
	if len(messageJsonSet) < 1 {
		return []types.OfflineHistoryMessageResponse{}
	}

	result := []types.OfflineHistoryMessageResponse{}
	for _, val := range messageJsonSet {
		addRes := types.OfflineHistoryMessageResponse{}
		err := json.Unmarshal([]byte(val), &addRes)
		if err == nil {
			result = append(result, addRes)
		} else {
			common.Errorf("离线消息 JSON 解析失败: raw=%s, err=%s", val, err.Error())
		}
	}
	return result
}

// 从 Mysql 查询指定时间之前的历史消息（并批量填充发送者头像昵称）
func GetHistoryMessagesFromMysql(svcCtx *svc.ServiceContext, ctx context.Context, sessionId int64, beforeTimeMillis int64, limit int) []types.OfflineHistoryMessageResponse {
	beforeTime := time.UnixMilli(beforeTimeMillis)
	messages, err := svcCtx.MessageModel.FindBySessionBeforeTime(ctx, sessionId, beforeTime, limit)
	if err != nil || len(messages) == 0 {
		return []types.OfflineHistoryMessageResponse{}
	}

	// 收集所有 senderId（去重）
	senderIds := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, m := range messages {
		if !seen[m.SenderId] {
			seen[m.SenderId] = true
			senderIds = append(senderIds, m.SenderId)
		}
	}

	// user-rpc 批量获取用户信息，用于填写最终的返回值 OfflineHistoryMessageResponse
	userInfoMap := make(map[int64]*user.BatchUserInfoItem)
	if len(senderIds) > 0 {
		batchResp, err := svcCtx.UserRpc.BatchGetUserInfos(ctx, &user.BatchGetUserInfosReq{UserIds: senderIds})
		if err != nil {
			logx.Errorf("批量获取用户信息失败: %s", err.Error())
			// 批量查询失败了，降级为逐个查询
			for _, uid := range senderIds {
				resp, err := svcCtx.UserRpc.GetUserInfo(ctx, &user.UserIdReq{UserId: uid})
				if err == nil {
					userInfoMap[uid] = &user.BatchUserInfoItem{
						UserId:   resp.UserId,
						Nickname: resp.Nickname,
						Avatar:   resp.Avatar,
					}
				}
			}
		} else {
			for _, u := range batchResp.Users {
				userInfoMap[u.UserId] = u
			}
		}
	}

	// 注意这里的长度是按照 message 取的，并不是按照，去重后的用户长度
	result := make([]types.OfflineHistoryMessageResponse, 0, len(messages))
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, m := range messages {
		createdTime := m.CreatedTime.In(loc).Format("2006-01-02 15:04:05")
		var body types.MessageBody
		if m.Type == 3 {
			err := json.Unmarshal([]byte(m.Content), &body)
			if err != nil {
				common.Errorf("types.MessageBody JSON 解析失败: err=%s", err.Error())
			}
		} else {
			body.Content = m.Content
			if m.ReplyId.Valid {
				body.ReplyId = m.ReplyId.Int64
			}
		}

		avatar := ""
		name := ""
		if info, ok := userInfoMap[m.SenderId]; ok {
			avatar = info.Avatar
			name = info.Nickname
		}

		result = append(result, types.OfflineHistoryMessageResponse{
			Type:        int32(m.Type),
			SenderId:    m.SenderId,
			Avatar:      avatar,
			Name:        name,
			CreatedTime: createdTime,
			Body:        body,
		})
	}

	return result
}
