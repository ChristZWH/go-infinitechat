// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"go-infinitechat/common/common"
	commonconstants "go-infinitechat/common/model/constants"
	"go-infinitechat/common/model/txctx"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/constants"
	userModel "go-infinitechat/service/user/model/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.UserRegisterRequest) (resp *types.LoginResponse, err error) {
	loginCodeKey := ""
	if constants.PhoneRegex.MatchString(req.Account) {
		code := "123456" //写死123456
		if code != req.Code {
			return nil, common.LoginError
		}
	} else if constants.EmailRegex.MatchString(req.Account) {
		loginCodeKey = constants.LoginCodeEmailPrefix + req.Account
		code, _ := l.svcCtx.Redis.Get(loginCodeKey)
		if code == "" || code != req.Code {
			return nil, common.LoginError
		}
	} else {
		common.Throw(common.PhoneEmailError)
	}

	user, err := l.svcCtx.UserService.GetUser(req.Account)
	common.ThrowIfWithMsg(user != nil, common.UserAlreadyExists, common.UserAlreadyExists.Message, err)

	//判断两次密码是否一致
	if req.Password != req.ConfirmPassword {
		return nil, common.PasswordMismatchError
	}

	//雪花算法生成userId
	userId := utils.NextInt()

	newUser := &userModel.User{UserId: userId}
	newUser.Avatar = ""
	if constants.PhoneRegex.MatchString(req.Account) {
		newUser.Phone = sql.NullString{String: req.Account, Valid: true}
	} else if constants.EmailRegex.MatchString(req.Account) {
		newUser.Email = sql.NullString{String: req.Account, Valid: true}
	}

	encryptedPassword := l.svcCtx.UserService.EncryptPassword(req.Password)
	newUser.Password = sql.NullString{String: encryptedPassword, Valid: true}
	newUser.Nickname = sql.NullString{String: req.Nickname, Valid: true}
	newUser.CreatedTime = time.Now()
	newUser.UpdatedTime = time.Now()

	err = txctx.WithTransaction(l.ctx, l.svcCtx.SqlConn, func(ctx context.Context) error {
		_, err2 := l.svcCtx.UserModel.InsertTx(ctx, newUser)
		common.ThrowIfWithMsg(err2 != nil, common.OperationError, "注册失败", err2)

		robotId := int64(123)
		robotUser, _ := l.svcCtx.UserService.GetUserById(robotId)
		common.ThrowIfWithMsg(robotUser == nil, common.SystemError, "机器人用户不存在，robotId:"+strconv.FormatInt(robotId, 10))

		addFriendResp := l.svcCtx.FriendService.AddFriend(ctx, newUser, robotId)
		common.Infof("用户注册成功，已添加机器人为好友，userId: %d, robotId: %d, sessionId: %d", userId, robotId, addFriendResp.SessionId)

		return nil
	})
	// 事务失败
	common.ThrowIf(err != nil, common.SystemError, err)

	twoToken := l.svcCtx.UserService.GetTwoToken(userId)
	wsServiceUri := l.svcCtx.WsServerLocator.GetWsServerUri(strconv.FormatInt(userId, 10))
	l.svcCtx.Redis.Hset(commonconstants.RedisWsServerUri, strconv.FormatInt(userId, 10), wsServiceUri)

	return &types.LoginResponse{
		UserId:       newUser.UserId,
		Account:      req.Account,
		Nickname:     newUser.Nickname.String,
		Avatar:       newUser.Avatar,
		Gender:       int64(newUser.Gender),
		Description:  newUser.Description.String,
		Token:        twoToken.AccessToken,
		RefreshToken: twoToken.RefreshToken,
		WeServiceURL: wsServiceUri,
	}, nil
}
