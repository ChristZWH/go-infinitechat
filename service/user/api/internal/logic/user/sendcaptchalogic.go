// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"
	"errors"

	"go-infinitechat/common/common"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/constants"
	utils2 "go-infinitechat/service/user/api/internal/utils"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

type SendCaptchaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendCaptchaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendCaptchaLogic {
	return &SendCaptchaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SendCaptchaLogic) SendCaptcha(req *types.SendCaptchaRequest) (resp string, err error) {
	code, err := utils.RandomNumber(6) // 生成6位随机验证码
	if err != nil {
		common.Errorf("生成验证码失败：%s", err.Error())
		return "", common.WrapError(common.SystemError, err)
	}

	var codeKey string
	if constants.PhoneRegex.MatchString(req.Account) {
		codeKey = constants.LoginCodePhonePrefix + req.Account
	} else if constants.EmailRegex.MatchString(req.Account) {
		codeKey = constants.LoginCodeEmailPrefix + req.Account
	}

	codeStr, err := l.svcCtx.Redis.Get(codeKey)
	if err != nil && !errors.Is(err, redis.Nil) {
		common.Errorf("从Redis获取验证码失败：%s", err.Error())
		return "", common.WrapError(common.RedisError, err)
	}

	if constants.PhoneRegex.MatchString(req.Account) { //手机号
		if codeStr != "" {
			return constants.SmsSendFrequentlyMsg, nil
		}
		//TODO 发送手机验证码

	} else if constants.EmailRegex.MatchString(req.Account) { //邮箱
		if codeStr != "" {
			return constants.SmsSendFrequentlyMsg, nil
		}
		//发送邮箱验证码（有底层 err → 记日志 + WrapError 返回）
		// http://localhost:8104/api/user/sendCaptcha?account=3463991617@qq.com
		// account 就是收信人地址
		if emailErr := utils2.SendEmailCode(req.Account, code); emailErr != nil {
			common.Errorf("发送邮件验证码失败：%s", emailErr)
			return "", common.WrapError(common.SystemError, emailErr)
		}
	} else {
		// 没有明确 error，直接 panic
		common.Throw(common.PhoneEmailError)
	}
	//保存验证码到redis（有底层 err → 记日志 + WrapError 返回）
	if redisErr := l.svcCtx.Redis.Setex(codeKey, code, constants.SmsExpireTime); redisErr != nil {
		common.Errorf("保存验证码失败：%s", redisErr)
		return "", common.WrapError(common.SystemError, redisErr)
	}

	common.Infof("发送验证码，账号：%s, 验证码：%s", req.Account, code[:3]+"***") // 打印日志(验证码脱敏)
	return constants.SmsSendSuccessMsg, nil
}
