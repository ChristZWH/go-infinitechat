package common

type ErrorCode struct {
	Code    int
	Message string
	Cause   error
}

// 实现 error 接口
func (e ErrorCode) Error() string {
	return e.Message
}

// 拿到真实原因
func (e ErrorCode) Unwrap() error {
	return e.Cause
}

func WrapError(code ErrorCode, cause error) ErrorCode {
	return ErrorCode{Code: code.Code, Message: code.Message, Cause: cause}
}

func newErrorCode(code int, message string) ErrorCode {
	return ErrorCode{Code: code, Message: message}
}

// ============ 通用错误码（40xxx：请求/客户端/通用业务错误）============
var (
	ParamsError       = newErrorCode(40000, "请求参数错误")
	NotLoginError     = newErrorCode(40100, "未登录")
	NoAuthError       = newErrorCode(40101, "无权限")
	TokenMissing      = newErrorCode(40102, "令牌缺失")
	TokenExpired      = newErrorCode(40103, "令牌已过期")
	TokenInvalid      = newErrorCode(40104, "令牌无效")
	TokenMismatch     = newErrorCode(40105, "令牌与用户不匹配")
	PleaseLogin       = newErrorCode(40106, "请先登录") // 原 50004：业务错误，不属于系统故障
	ForbiddenError    = newErrorCode(40300, "禁止访问")
	NotFoundError     = newErrorCode(40400, "请求数据不存在")
	SameLoginConflict = newErrorCode(40900, "账号已在其他地方登录") // 原 50005：409 冲突语义
)

// ============ 系统错误码（50xxx：内部故障，对外只给笼统提示，细节只进日志）============
var (
	SystemError         = newErrorCode(50000, "系统内部异常")
	OperationError      = newErrorCode(50001, "操作失败") // 系统级操作失败（如写库失败）的兜底
	RedisError          = newErrorCode(50002, "Redis启动错误")
	MysqlError          = newErrorCode(50003, "Mysql启动错误")
	SystemBusy          = newErrorCode(50008, "系统繁忙，请稍后重试")
	PusherNotFoundError = newErrorCode(50009, "Pusher 未找到")
	MarshalFailedError  = newErrorCode(50010, "序列化失败")
	PushFailedError     = newErrorCode(50011, "Kafka 消息推送失败")
)

// ============ 用户相关错误码（70xxx）============
var (
	PhoneEmailError       = newErrorCode(70000, "手机号/邮箱格式错误")
	UserAlreadyExists     = newErrorCode(70001, "用户已存在")
	UserNotExists         = newErrorCode(70002, "用户不存在")
	RegisterError         = newErrorCode(70003, "注册失败")
	CaptchaError          = newErrorCode(70004, "验证码错误") // 原 LoginErrorCode：原名与语义不符
	LoginError            = newErrorCode(70005, "登录失败, 用户名或密码错误")
	PasswordMismatchError = newErrorCode(70006, "两次密码不一致") // 原 LoginPasswordError：出现于注册/改密场景
)

// ============ WebSocket 业务错误码（90xxx）============
var (
	SignalTypeError    = newErrorCode(90000, "单聊消息必须指定接收者")
	GroupTypeError     = newErrorCode(90001, "群聊消息不需要指定接收者")
	InvalidToken       = newErrorCode(90003, "无效token，请重新登录")
	UserEmailListEmpty = newErrorCode(90004, "用户邮箱列表为空，请检查用户服务是否正常或没有用户注册")
)

func ThrowIf(condition bool, err ErrorCode, cause ...error) {
	if condition {
		var c error
		if len(cause) > 0 {
			c = cause[0]
		}
		panic(ErrorCode{Code: err.Code, Message: err.Message, Cause: c})
	}
}

func ThrowIfWithMsg(condition bool, err ErrorCode, msg string, cause ...error) {
	if condition {
		var c error
		if len(cause) > 0 {
			c = cause[0]
		}
		panic(ErrorCode{Code: err.Code, Message: msg, Cause: c})
	}
}
func ThrowWithMsg(err ErrorCode, msg string, cause ...error) {
	var c error
	if len(cause) > 0 {
		c = cause[0]
	}
	panic(ErrorCode{Code: err.Code, Message: msg, Cause: c})
}

func Throw(err ErrorCode, cause ...error) {
	var c error
	if len(cause) > 0 {
		c = cause[0]
	}
	panic(ErrorCode{Code: err.Code, Message: err.Message, Cause: c})
}
