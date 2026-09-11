// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package offlinedata

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go-infinitechat/common/common"
	"go-infinitechat/common/model/constants"
	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type HistoricalMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHistoricalMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HistoricalMessageLogic {
	return &HistoricalMessageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HistoricalMessageLogic) HistoricalMessage(req *types.HistoryMessageRequest) (resp []types.OfflineHistoryMessageResponse, err error) {
	sessionId := req.SessionId
	limit := constants.LimitMessageCount //30

	// 解析客户端传来的时间字符串
	loc, _ := time.LoadLocation("Asia/Shanghai")
	requestTimeObj, err := time.ParseInLocation("2006-01-02 15:04:05", req.Time, loc)
	if err != nil {
		common.Errorf("历史消息请求时间格式错误: time=%s, err=%s", req.Time, err.Error())
		return []types.OfflineHistoryMessageResponse{}, nil
	}
	requestTime := requestTimeObj.UnixMilli()
	currentTime := time.Now().UnixMilli()
	hotBoundary := currentTime - constants.SevenDaysMillis

	result := []types.OfflineHistoryMessageResponse{}

	// 是否超过七天时间
	if requestTime > hotBoundary {
		// 请求时间在热数据范围内，先查 Redis
		redisMessages := getHistoryMessageFromRedis(l.svcCtx.Redis, sessionId, requestTime, limit)
		if len(redisMessages) > 0 {
			result = append(result, redisMessages...)
		}
		// Redis 不够，补充 MySQL
		if len(redisMessages) < limit {
			remaining := limit - len(redisMessages)
			mysqlMessage := GetHistoryMessagesFromMysql(l.svcCtx, l.ctx, req.SessionId, hotBoundary, remaining)
			if len(mysqlMessage) > 0 {
				result = append(result, mysqlMessage...)
			}
		}
	} else {
		// 请求时间超出热数据范围，直接查 MySQL
		// 边界必须是 requestTime（客户端要的是"这个时间之前"的消息），
		// 用 hotBoundary 会把 (requestTime, hotBoundary] 区间里客户端已有的消息重复返回
		result = GetHistoryMessagesFromMysql(l.svcCtx, l.ctx, sessionId, requestTime, limit)
	}

	//兜底 Redis 段和 MySQL 段拼接后的乱序
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].CreatedTime > result[j].CreatedTime
	})

	return result, nil
}

// 从 Redis 获取指定时间之前的历史消息
func getHistoryMessageFromRedis(rds *redis.Redis, sessionId int64, requestTime int64, limit int) []types.OfflineHistoryMessageResponse {
	sessionKey := fmt.Sprintf("session:%d", sessionId)

	// ZREVRANGEBYSCORE key max min LIMIT 0 count
	// 查找 score < requestTime 的消息，按时间倒序
	// 注意：第一个参数是 max，第二个是 min
	pairs, err := rds.ZrevrangebyscoreWithScoresAndLimit(sessionKey, requestTime-1, 0, 0, limit)
	if err != nil || len(pairs) == 0 {
		return []types.OfflineHistoryMessageResponse{}
	}

	msgJsonSet := make([]string, 0, len(pairs))
	for _, p := range pairs {
		msgJsonSet = append(msgJsonSet, p.Key)
	}

	return ParseMessagesFromJSON(msgJsonSet)
}
