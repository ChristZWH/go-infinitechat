package constants

const (
	SmsAccessKeyId     = ""
	SmsAccessKeySecret = ""                 // 前两个是阿里云账号的密钥对
	SmsSigName         = "自己学习案例"           // 短信签名,用户收到的短信开头会显示 【无夕教育科技】
	SmSTemplateCode    = "SMS_684558841457" // 在阿里云控制台申请，申请通过之后拿到

	SmsSendSuccessMsg        = "验证码发送成功"
	SmsSendFrequentlyMsg     = "发送太频繁，请稍后再试"
	SmsUpdatePasswordSuccess = "修改密码成功"
	EmailHostName            = "smtp.qq.com"
	EmailUserName            = "3463991617@qq.com"
	EmailPassword            = "vfsbsupvdgywcjac"
	EmailName                = "个人学习案例" // 发件人显示名，收件人看到的名字
	EmailSubject             = "注册验证码"  // 邮件主题
	SmsExpireTime            = 5 * 60   // 300 秒
)
