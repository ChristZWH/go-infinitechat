package constants

import "regexp"

var (
	//手机号的正则表达式
	PhoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

	//邮箱的正则表达式
	EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9_+&*-]+(?:\.[a-zA-Z0-9_+&*-]+)*@(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$`)

	//密码的盐值
	PasswordSalt = "USER"

	//登录验证码的Redis键前缀，邮箱
	LoginCodeEmailPrefix = "login_code_email:"

	//登录验证码的Redis键前缀，手机号
	LoginCodePhonePrefix = "login_code_phone:"
)
