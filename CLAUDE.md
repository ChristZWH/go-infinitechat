# 待办
- validate 标签校验未实现：go-zero v1.10.3 的 httpx.Parse 不自动执行 api 文件里的 validate/msg 标签，
  需在 main.go 注册 httpx.SetValidator 自定义校验器（或改在 logic 兜底），稍后实现