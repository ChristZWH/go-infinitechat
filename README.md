"# go-infinitechat" 

logic 层必须把内部错误用 WrapError 包装成 ErrorCode 再 return/panic，禁止裸传原始 error（否则会落到 500 兜底，业务错误被误判成故障）。这条约定建议写进 error_code.go 顶部注释或 README