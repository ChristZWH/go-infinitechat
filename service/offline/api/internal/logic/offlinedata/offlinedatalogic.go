// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package offlinedata

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go-infinitechat/common/common"
	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

type OfflineDataLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOfflineDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OfflineDataLogic {
	return &OfflineDataLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

const GetAndDeleteOfflineDataLua = `
	local sessionKey = KEYS[1]
	local countKey = KEYS[2]
	
	-- 获取两个 hash 的所有字段（返回格式为 [k1, v1, k2, v2...])
	local sessionData = redis.call('HGETALL', sessionKey)
	local countData = redis.call('HGETALL', countKey)
	
	-- 删除两个 key
	redis.call('DEL', sessionKey, countKey)
	
	-- 返回嵌套数组{{k,v,k,v}, {k,v,k,v}}
	return {sessionData, countData}
`

func (l *OfflineDataLogic) OfflineData(req *types.OfflineDataRequest) (resp []types.OfflineDataResponse, err error) {
	sessionSnapshotKey := fmt.Sprintf("user:%d", req.UserId)
	sessionCountKey := fmt.Sprintf("user:%d:count", req.UserId)

	script := redis.NewScript(GetAndDeleteOfflineDataLua)
	val, err := l.svcCtx.Redis.ScriptRunCtx(l.ctx, script, []string{sessionSnapshotKey, sessionCountKey})
	if err != nil {
		common.Errorf("执行Lua脚本失败：%s", err.Error())
		return []types.OfflineDataResponse{}, common.WrapError(common.RedisError, err)
	}

	if val == nil {
		return []types.OfflineDataResponse{}, nil
	}

	outerList, ok := val.([]interface{})
	if !ok || len(outerList) < 2 {
		return []types.OfflineDataResponse{}, nil
	}
	// outerList[0] 是 sessionData，outerList[1] 是 countData。将切片转化为map
	sessionList, ok := outerList[0].([]interface{})
	if !ok {
		common.Errorf("离线数据格式异常：sessionData 类型错误")
		return []types.OfflineDataResponse{}, nil
	}
	countList, ok := outerList[1].([]interface{})
	if !ok {
		common.Errorf("离线数据格式异常：countData 类型错误")
		return []types.OfflineDataResponse{}, nil
	}

	sessionMap := flatListToMap(sessionList)
	countMap := flatListToMap(countList)

	var responses []types.OfflineDataResponse

	// 组转数据 遍历出来的就是 field value
	for sessionId, jsonStr := range sessionMap {
		var data map[string]string
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue // 容错处理
		}

		unreadCount := "0"
		if c, exists := countMap[sessionId]; exists {
			unreadCount = c
		}

		// 类型安全转换
		sId, _ := strconv.ParseInt(sessionId, 10, 64)
		uType, _ := strconv.Atoi(data["type"]) // Atoi:字符串数字转整形的数字
		sType, _ := strconv.Atoi(data["sessionType"])
		cnt, _ := strconv.Atoi(unreadCount)
		senderId, _ := strconv.ParseInt(data["senderId"], 10, 64)

		responses = append(responses, types.OfflineDataResponse{
			Type:           int32(uType),
			SessionType:    int32(sType),
			SessionId:      sId,
			SenderId:       senderId,
			Avatar:         data["avatar"],
			Name:           data["name"],
			LastMsgContent: data["lastMsgContent"],
			LastMsgTime:    data["lastMsgTime"],
			Count:          int32(cnt),
		})
	}

	return responses, nil
}

// 将 [k1, v1, k2, v2] 转换为 map
func flatListToMap(list []interface{}) map[string]string {
	res := make(map[string]string, len(list))
	for i := 0; i < len(list); i += 2 {
		if i+1 >= len(list) {
			break
		}
		key, ok := list[i].(string)
		if !ok {
			continue
		}
		val, ok := list[i+1].(string)
		if !ok {
			continue
		}
		if key != "" {
			res[key] = val
		}
	}
	return res
}
