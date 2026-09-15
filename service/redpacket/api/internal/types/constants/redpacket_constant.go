package constants

const (
	//未领完（红包仍有剩余份数可领取）
	StatusNotCompleted = 0

	//已领完（红包所有份数都已被领取）
	StatusCompleted = 1

	//已过期（红包超过24小时未领完，剩余金额已退还发送者）
	StatusExpired = 2

	//红包不存在
	StatusNotExist = -1
)

const (
	//普通红包（等额红包，每人领取金额相同）
	TypeNormal = 0

	//拼手气红包（随机金额，每人领取金额不同）
	TypeRandom = 1
)

const (
	//红包过期时间（毫秒），24小时
	ExpireTimeMs = 24 * 60 * 60 * 1000

	//Redis 缓存过期时间（小时），比红包过期时间多1小时，确保过期处理完成后缓存才失效
	RedisCacheExpireHours = 25
)

const (
	//单个红包最大金额（单位：元）
	MaxSingleAmountYuan = 200

	//元转分的乘数（1元 = 100分）
	YuanToFenMultiplier = 100
)

const (
	//用户已领取过该红包
	AlreadyReceived = -1

	// 抢红包内 Lua 验证红包过期
	AlreadyExpired = -2

	//红包金额池已空
	EmptyPool = -3

	//红包尚未被领完
	CompletionFlagNotCompleted = 0

	//红包刚好被领完（本次领取是最后一份）
	CompletionFlagCompleted = 1
)

const (
	//发红包（扣减余额）
	BalanceLogTypeSend = 0

	//领红包（增加余额）
	BalanceLogTypeReceive = 1

	//红包过期退款（增加余额）
	BalanceLogTypeRefund = 2
)

const (
	//群聊
	SessionTypeGroup = 0

	//单聊
	SessionTypeSignal = 1
)

func IsValidType(redPacketType int) bool {
	return redPacketType == TypeNormal || redPacketType == TypeRandom
}

func IsCompleted(completedFlag int64) bool {
	return completedFlag == CompletionFlagCompleted
}
