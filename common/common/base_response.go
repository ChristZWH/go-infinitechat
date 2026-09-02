package common

type BaseResponse[T any] struct {
	Code    int    `json:"code"`
	Data    T      `json:"data"`
	Message string `json:"message"`
}

type BaseErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewBaseResponse 通用成功响应构造器：code/data/message 由调用方指定
func NewBaseResponse[T any](code int, data T, msg string) *BaseResponse[T] {
	return &BaseResponse[T]{Code: code, Data: data, Message: msg}
}

// NewBaseResponseFromErrorCode 通用错误响应构造器：code/message 取自 ErrorCode
func NewBaseErrorResponseFromErrorCode(err ErrorCode) *BaseErrorResponse {
	return &BaseErrorResponse{Code: err.Code, Message: err.Message}
}

// NewBaseResponseFromMessage 通用错误响应构造器：code/message 由调用方指定
func NewBaseErrorResponseFromMessage(code int, msg string) *BaseErrorResponse {
	return &BaseErrorResponse{Code: code, Message: msg}
}
