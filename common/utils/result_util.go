package utils

import "go-infinitechat/common/common"

func Success[T any](data T) *common.BaseResponse[T] {
	return common.NewBaseResponse(200, data, "ok")
}

func FailWithErrorCode(err common.ErrorCode) *common.BaseErrorResponse {
	return common.NewBaseResponseFromErrorCode(err.Code, err)
}

func FailWithMessage(err common.ErrorCode, message string) *common.BaseErrorResponse {
	return common.NewBaseResponseFromMessage(err.Code, message)
}
