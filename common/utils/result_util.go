package utils

import "go-infinitechat/common/common"

func Success[T any](data T) *common.BaseResponse[T] {
	return common.NewBaseResponse(200, data, "ok")
}

func Fail(err common.ErrorCode) *common.BaseErrorResponse {
	return common.NewBaseResponseFromErrorCode(err)
}

func FailWithCode(code int, message string) *common.BaseErrorResponse {
	return common.NewBaseResponseFromMessage(code, message)
}
