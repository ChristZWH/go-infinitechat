package dto

import "github.com/shopspring/decimal"

type TwoJWT struct {
	AccessToken  string
	RefreshToken string
}

type ModifyFriendApplicationResponse struct {
	UserId      string `json:"userId"`
	SessionId   string `json:"sessionId"`
	SessionType int32  `json:"sessionType"`
	SessionName string `json:"sessionName"`
	Avatar      string `json:"avatar"`
}

// 用户余额（单位：元）
type UserBalanceResponse struct {
	Balance decimal.Decimal `json:"balance"`
}

type BalanceLogDTO struct {
	//相关用户名称
	UserName string `json:"userName"`
	//类型（1:收入 2:支出）
	Type int32 `json:"type"`
	//金额（单位：元，数据库存储为分）
	Amount decimal.Decimal `json:"amount"`
	//时间（格式化为 MM月DD日 HH:mm）
	Time string `json:"time"`
}

type FriendDTO struct {
	UserId    string `json:"userId"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Signature string `json:"signature"`
	Status    int64  `json:"status"`
	SessionId string `json:"sessionId"`
}

type ApplyFriendDTO struct {
	UserId     string `json:"userId"`
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar"`
	Msg        string `json:"msg"`
	Status     int64  `json:"status"`
	Time       string `json:"time"`
	IsReceiver int    `json:"isReceiver"` // 0:我是发送者 1:我是接收者
}
