package service

import (
	"context"
	"errors"
	"testing"

	"go-infinitechat/common/common"
	"go-infinitechat/service/user/api/internal/types/constants"
	"go-infinitechat/service/user/model/user_session"
)

// fakeUserSessionModel 只实现 FindOne，其余由内嵌接口兜底
type fakeUserSessionModel struct {
	user_session.UserSessionModel
	findOne func(ctx context.Context, userId, sessionId int64) (*user_session.UserSession, error)
}

func (f *fakeUserSessionModel) FindOne(ctx context.Context, userId, sessionId int64) (*user_session.UserSession, error) {
	return f.findOne(ctx, userId, sessionId)
}

func newGroupServiceForAuth(m user_session.UserSessionModel) *GroupService {
	return &GroupService{UserSessionModel: m}
}

func TestEnsureGroupMember(t *testing.T) {
	member := &user_session.UserSession{Role: RoleGroupMember, Status: constants.StatusSuccess}

	tests := []struct {
		name    string
		findOne func(ctx context.Context, userId, sessionId int64) (*user_session.UserSession, error)
		wantErr *common.ErrorCode // nil 表示不 panic
	}{
		{
			name: "是成员",
			findOne: func(_ context.Context, _, _ int64) (*user_session.UserSession, error) {
				return member, nil
			},
			wantErr: nil,
		},
		{
			name: "不是成员",
			findOne: func(_ context.Context, _, _ int64) (*user_session.UserSession, error) {
				return nil, errors.New("sql: no rows in result set")
			},
			wantErr: &common.NoAuthError,
		},
		{
			name: "已退出（status != 0）",
			findOne: func(_ context.Context, _, _ int64) (*user_session.UserSession, error) {
				return &user_session.UserSession{Role: RoleGroupMember, Status: 1}, nil
			},
			wantErr: &common.NoAuthError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := newGroupServiceForAuth(&fakeUserSessionModel{findOne: tt.findOne})
			fn := func() {
				gs.EnsureGroupMember(context.Background(), 55, 1001)
			}

			if tt.wantErr == nil {
				// 不 panic 即通过
				fn()
				return
			}

			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("期望 panic(ErrorCode %d), 实际没有", tt.wantErr.Code)
				}
				got, ok := r.(common.ErrorCode)
				if !ok {
					t.Fatalf("panic 类型 = %T, 期望 ErrorCode", r)
				}
				if got.Code != tt.wantErr.Code {
					t.Fatalf("错误码 = %d, 期望 %d", got.Code, tt.wantErr.Code)
				}
			}()
			fn()
		})
	}
}
