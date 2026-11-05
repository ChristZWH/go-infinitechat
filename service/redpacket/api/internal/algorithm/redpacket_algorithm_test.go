package algorithm

import (
	"testing"
)

// sum 计算总金额，供各用例断言"总额分毫不差"
func sum(amounts []int64) int64 {
	var s int64
	for _, a := range amounts {
		s += a
	}
	return s
}

func TestAllocateNormalRedPacket(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		count     int
		wantErr   bool
		wantTotal int64
	}{
		{name: "整除", total: 1000, count: 10, wantTotal: 1000},
		{name: "有余数（前 remainder 份各多 1 分）", total: 1003, count: 10, wantTotal: 1003},
		{name: "每份 1 分", total: 10, count: 10, wantTotal: 10},
		{name: "金额不足", total: 9, count: 10, wantErr: true},
		{name: "金额为 0", total: 0, count: 1, wantErr: true},
		{name: "数量为 0", total: 100, count: 0, wantErr: true},
		{name: "大额", total: 1000000_00, count: 2, wantTotal: 1000000_00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amounts, err := AllocateNormalRedPacket(tt.total, tt.count)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望返回错误，实际返回金额 %v", amounts)
				}
				return
			}
			if err != nil {
				t.Fatalf("非预期错误: %v", err)
			}
			if len(amounts) != tt.count {
				t.Fatalf("份数 = %d, 期望 %d", len(amounts), tt.count)
			}
			if got := sum(amounts); got != tt.wantTotal {
				t.Errorf("总额 = %d, 期望 %d", got, tt.wantTotal)
			}
			for i, a := range amounts {
				if a < MinAmount {
					t.Errorf("第 %d 份金额 %d < 最小金额 %d", i, a, MinAmount)
				}
			}
		})
	}
}

func TestAllocateNormalRedPacketDistributesRemainder(t *testing.T) {
	// 1003 分 / 10 份 = 100 余 3：前 3 份 101，其余 100
	amounts, err := AllocateNormalRedPacket(1003, 10)
	if err != nil {
		t.Fatalf("非预期错误: %v", err)
	}
	for i := 0; i < 3; i++ {
		if amounts[i] != 101 {
			t.Errorf("第 %d 份 = %d, 期望 101", i, amounts[i])
		}
	}
	for i := 3; i < 10; i++ {
		if amounts[i] != 100 {
			t.Errorf("第 %d 份 = %d, 期望 100", i, amounts[i])
		}
	}
}

func TestAllocateRandomRedPacket(t *testing.T) {
	tests := []struct {
		name    string
		total   int64
		count   int
		wantErr bool
	}{
		{name: "常规拼手气", total: 10000, count: 10},
		{name: "只有 1 份", total: 500, count: 1},
		{name: "每份恰好 1 分（remainAmount == 0 的退化场景）", total: 10, count: 10},
		{name: "余量不足以做切点（remainAmount < count，曾会死循环）", total: 15, count: 10},
		{name: "两份", total: 3, count: 2},
		{name: "金额不足", total: 5, count: 10, wantErr: true},
		{name: "数量为 0", total: 100, count: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 拼手气有随机性，重复多轮确保不会偶发错误
			for round := 0; round < 50; round++ {
				amounts, err := AllocateRandomRedPacket(tt.total, tt.count)
				if tt.wantErr {
					if err == nil {
						t.Fatalf("期望返回错误，实际返回金额 %v", amounts)
					}
					return
				}
				if err != nil {
					t.Fatalf("非预期错误: %v", err)
				}
				if len(amounts) != tt.count {
					t.Fatalf("份数 = %d, 期望 %d", len(amounts), tt.count)
				}
				if got := sum(amounts); got != tt.total {
					t.Fatalf("第 %d 轮总额 = %d, 期望 %d, 明细 %v", round, got, tt.total, amounts)
				}
				for i, a := range amounts {
					if a < MinAmount {
						t.Fatalf("第 %d 轮第 %d 份金额 %d < 最小金额 %d", round, i, a, MinAmount)
					}
				}
			}
		})
	}
}

func TestAllocateRandomRedPacketSingle(t *testing.T) {
	amounts, err := AllocateRandomRedPacket(777, 1)
	if err != nil {
		t.Fatalf("非预期错误: %v", err)
	}
	if len(amounts) != 1 || amounts[0] != 777 {
		t.Fatalf("单份红包 = %v, 期望 [777]", amounts)
	}
}
