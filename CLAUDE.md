# 待办
- validate 标签校验未实现：go-zero v1.10.3 的 httpx.Parse 不自动执行 api 文件里的 validate/msg 标签，
  需在 main.go 注册 httpx.SetValidator 自定义校验器（或改在 logic 兜底），稍后实现
  
-  validator.go 内部实现 → 等学到"反射"专题再回来看，当前阶段先跳过（黑盒子）

-  手机验证码登录暂为占位：loginlogic.go 的 code 分支里手机号写死 code == "123456"（发短信是 TODO），
   本项目是个人学习项目，暂时跳过；注意发送侧已把手机验证码存入 Redis（login_code_phone:<账号>），
   接入真实短信后把登录侧改成与邮箱分支一致：查 Redis 比对 + 成功后 Del