// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"
	"database/sql"
	"errors"

	"go-infinitechat/common/common"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/constants"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type UpdatePasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePasswordLogic {
	return &UpdatePasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePasswordLogic) UpdatePassword(req *types.UpdatePasswordRequest) (string, error) {
	common.ThrowIf(req.Password != req.ConfirmPassword, common.PasswordMismatchError)

	//判断验证码是否正确
	codeKey := ""
	if constants.PhoneRegex.MatchString(req.Account) {
		codeKey = constants.LoginCodePhonePrefix + req.Account
	} else if constants.EmailRegex.MatchString(req.Account) {
		codeKey = constants.LoginCodeEmailPrefix + req.Account
	} else {
		common.Throw(common.PhoneEmailError)
	}
	code, err := l.svcCtx.Redis.Get(codeKey)
	common.ThrowIf(err != nil, common.SystemError, err)
	common.ThrowIf(code == "" || code != req.Code, common.CaptchaError)

	user, err := l.svcCtx.UserService.GetUser(req.Account)
	common.ThrowIf(err != nil && !errors.Is(err, sqlx.ErrNotFound), common.SystemError, err)
	common.ThrowIf(user == nil, common.UserNotExists)

	//密码加密
	encryptedPassword := l.svcCtx.UserService.EncryptPassword(req.Password)
	user.Password = sql.NullString{String: encryptedPassword, Valid: true}
	l.svcCtx.UserService.UpdatePassword(l.ctx, user)

	// 验证码一次性使用：修改成功后删除（与 loginlogic 保持一致）
	l.svcCtx.Redis.Del(codeKey)

	return constants.SmsUpdatePasswordSuccess, nil
}
