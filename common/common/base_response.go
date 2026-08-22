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

func NewBaseResponse[T any](code int, data T, msg string) *BaseResponse[T] {
	return &BaseResponse[T]{Code: code, Data: data, Message: msg}
}

func NewBaseResponseFromErrorCode(err ErrorCode) *BaseErrorResponse {
	return &BaseErrorResponse{Code: err.Code, Message: err.Message}
}

func NewBaseResponseFromMessage(code int, msg string) *BaseErrorResponse {
	return &BaseErrorResponse{Code: code, Message: msg}
}
