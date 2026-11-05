package logic

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"go-infinitechat/service/user/model/user"
	"go-infinitechat/service/user/model/user_session"
	"go-infinitechat/service/user/rpc/internal/svc"
	userpb "go-infinitechat/service/user/rpc/user"
)

// fakeUserModel 只实现被测试代码用到的 FindOne，其余方法由内嵌的 nil 接口兜底。
// 这样不用连 MySQL 就能验证 logic 的控制流。
type fakeUserModel struct {
	user.UserModel
	users map[int64]*user.User
	err   error
}

func (f *fakeUserModel) FindOne(_ context.Context, id int64) (*user.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	u, ok := f.users[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

type fakeUserSessionModel struct {
	user_session.UserSessionModel
	members []int64
	err     error
}

func (f *fakeUserSessionModel) FindMemberIdsBySessionId(_ context.Context, _ int64) ([]int64, error) {
	return f.members, f.err
}

func newUser(nickname string) *user.User {
	return &user.User{Nickname: sql.NullString{String: nickname, Valid: true}}
}

// BatchGetUserInfos 曾经把条件写反（err == nil 也 continue），导致接口恒返回空。
// 这个用例锁住"查询成功必须出现在结果里"。
func TestBatchGetUserInfosReturnsFoundUsers(t *testing.T) {
	u1 := newUser("alice")
	u1.UserId = 1
	u2 := newUser("bob")
	u2.UserId = 2

	svcCtx := &svc.ServiceContext{
		UserModel: &fakeUserModel{users: map[int64]*user.User{1: u1, 2: u2}},
	}
	l := NewBatchGetUserInfosLogic(context.Background(), svcCtx)

	resp, err := l.BatchGetUserInfos(&userpb.BatchGetUserInfosReq{UserIds: []int64{1, 2, 3}})
	if err != nil {
		t.Fatalf("非预期错误: %v", err)
	}
	if len(resp.Users) != 2 {
		t.Fatalf("返回用户数 = %d, 期望 2（不存在的 3 应被跳过，1 和 2 必须返回）", len(resp.Users))
	}

	got := map[int64]string{}
	for _, u := range resp.Users {
		got[u.UserId] = u.Nickname
	}
	if got[1] != "alice" || got[2] != "bob" {
		t.Errorf("返回内容不正确: %+v", got)
	}
}

func TestBatchGetUserInfosEmptyInput(t *testing.T) {
	svcCtx := &svc.ServiceContext{UserModel: &fakeUserModel{users: map[int64]*user.User{}}}
	l := NewBatchGetUserInfosLogic(context.Background(), svcCtx)

	resp, err := l.BatchGetUserInfos(&userpb.BatchGetUserInfosReq{})
	if err != nil {
		t.Fatalf("空入参不应报错: %v", err)
	}
	if resp == nil || len(resp.Users) != 0 {
		t.Fatalf("空入参应返回空结果，实际 %+v", resp)
	}
}

func TestBatchGetUserInfosSkipsDBError(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		UserModel: &fakeUserModel{err: errors.New("db down")},
	}
	l := NewBatchGetUserInfosLogic(context.Background(), svcCtx)

	resp, err := l.BatchGetUserInfos(&userpb.BatchGetUserInfosReq{UserIds: []int64{1, 2}})
	if err != nil {
		t.Fatalf("非预期错误: %v", err)
	}
	if len(resp.Users) != 0 {
		t.Fatalf("查询失败时应跳过，实际返回 %d 条", len(resp.Users))
	}
}

func TestGetGroupMemberIds(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		UserSessionModel: &fakeUserSessionModel{members: []int64{10, 20, 30}},
	}
	l := NewGetGroupMemberIdsLogic(context.Background(), svcCtx)

	resp, err := l.GetGroupMemberIds(&userpb.SessionIdReq{SessionId: 9001})
	if err != nil {
		t.Fatalf("非预期错误: %v", err)
	}
	if len(resp.UserIds) != 3 {
		t.Fatalf("成员数 = %d, 期望 3", len(resp.UserIds))
	}
	if resp.UserIds[0] != 10 || resp.UserIds[2] != 30 {
		t.Errorf("成员列表顺序/内容不正确: %v", resp.UserIds)
	}
}

func TestGetGroupMemberIdsPropagatesError(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		UserSessionModel: &fakeUserSessionModel{err: errors.New("db down")},
	}
	l := NewGetGroupMemberIdsLogic(context.Background(), svcCtx)

	if _, err := l.GetGroupMemberIds(&userpb.SessionIdReq{SessionId: 1}); err == nil {
		t.Fatal("底层查询失败时 logic 必须把错误抛出去（否则群消息会被静默丢弃）")
	}
}
