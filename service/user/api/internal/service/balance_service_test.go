package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"go-infinitechat/common/common"
	"go-infinitechat/service/user/model/user_balance"
)

// fakeUserBalanceModel 只实现 FindOne / InsertTx，其余由内嵌接口兜底
type fakeUserBalanceModel struct {
	user_balance.UserBalanceModel
	findOneRes *user_balance.UserBalance
	findOneErr error
	insertErr  error
}

func (f *fakeUserBalanceModel) FindOne(_ context.Context, _ int64) (*user_balance.UserBalance, error) {
	return f.findOneRes, f.findOneErr
}

func (f *fakeUserBalanceModel) InsertTx(_ context.Context, _ *user_balance.UserBalance) (sql.Result, error) {
	return nil, f.insertErr
}

func assertPanicCode(t *testing.T, want common.ErrorCode, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("期望 panic(ErrorCode %d), 实际没有", want.Code)
		}
		got, ok := r.(common.ErrorCode)
		if !ok {
			t.Fatalf("panic 类型 = %T, 期望 ErrorCode", r)
		}
		if got.Code != want.Code {
			t.Fatalf("错误码 = %d, 期望 %d", got.Code, want.Code)
		}
	}()
	fn()
}

func TestInitializaBalance(t *testing.T) {
	ctx := context.Background()

	t.Run("首次初始化成功", func(t *testing.T) {
		bs := &BalanceService{UserBalanceModel: &fakeUserBalanceModel{
			findOneRes: nil, findOneErr: sql.ErrNoRows,
		}}
		bs.InitializaBalance(ctx, 1, 0) // 不应 panic
	})

	t.Run("余额为负", func(t *testing.T) {
		bs := &BalanceService{UserBalanceModel: &fakeUserBalanceModel{}}
		assertPanicCode(t, common.ParamsError, func() { bs.InitializaBalance(ctx, 1, -1) })
	})

	t.Run("已存在", func(t *testing.T) {
		bs := &BalanceService{UserBalanceModel: &fakeUserBalanceModel{
			findOneRes: &user_balance.UserBalance{UserId: 1},
		}}
		assertPanicCode(t, common.OperationError, func() { bs.InitializaBalance(ctx, 1, 0) })
	})

	t.Run("数据库故障", func(t *testing.T) {
		bs := &BalanceService{UserBalanceModel: &fakeUserBalanceModel{
			findOneErr: errors.New("db down"),
		}}
		assertPanicCode(t, common.SystemError, func() { bs.InitializaBalance(ctx, 1, 0) })
	})
}

func TestGetUserBalance(t *testing.T) {
	t.Run("成功，分转元", func(t *testing.T) {
		bs := &BalanceService{UserBalanceModel: &fakeUserBalanceModel{
			findOneRes: &user_balance.UserBalance{UserId: 1, Balance: 12345},
		}}
		resp := bs.GetUserBalance(1)
		if resp.Balance.String() != "123.45" {
			t.Fatalf("Balance = %s, 期望 123.45", resp.Balance.String())
		}
	})

	t.Run("用户ID非法", func(t *testing.T) {
		bs := &BalanceService{UserBalanceModel: &fakeUserBalanceModel{}}
		assertPanicCode(t, common.ParamsError, func() { bs.GetUserBalance(0) })
	})

	t.Run("余额记录不存在", func(t *testing.T) {
		bs := &BalanceService{UserBalanceModel: &fakeUserBalanceModel{
			findOneErr: sql.ErrNoRows,
		}}
		assertPanicCode(t, common.NotFoundError, func() { bs.GetUserBalance(1) })
	})

	t.Run("数据库故障不误报 404", func(t *testing.T) {
		// 曾经条件写反：数据库故障被当成"余额记录不存在"
		bs := &BalanceService{UserBalanceModel: &fakeUserBalanceModel{
			findOneErr: errors.New("db down"),
		}}
		assertPanicCode(t, common.SystemError, func() { bs.GetUserBalance(1) })
	})
}
