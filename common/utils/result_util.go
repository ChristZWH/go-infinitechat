package utils

import "go-infinitechat/common/common"

// 真正的响应调用这里，而不是通过 common
// Success 成功响应：code 固定 200，message 固定 ok，约定只在此处维护
func Success[T any](data T) *common.BaseResponse[T] {
	return common.NewBaseResponse(200, data, "ok")
}

// Fail 错误响应：用于已封装的 ErrorCode
func Fail(err common.ErrorCode) *common.BaseErrorResponse {
	return common.NewBaseErrorResponseFromErrorCode(err)
}

// FailWithCode 错误响应：用于没有 ErrorCode 封装的场景，code、message 由调用方指定
func FailWithCode(code int, message string) *common.BaseErrorResponse {
	return common.NewBaseErrorResponseFromMessage(code, message)
}
