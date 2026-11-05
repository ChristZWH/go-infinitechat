package service

import "testing"

func TestYuanToFen(t *testing.T) {
	tests := []struct {
		name    string
		yuan    string
		want    int64
		wantErr bool
	}{
		{"整数元", "10", 1000, false},
		{"两位小数", "10.50", 1050, false},
		{"一位小数", "0.5", 50, false},
		{"一分钱", "0.01", 1, false},
		{"零", "0", 0, false},
		{"三位小数应报错", "1.234", 0, true},
		{"非数字", "abc", 0, true},
		// 负数能被解析出来（-100 分），但调用方必须自己拒绝：
		// sendredpacketlogic.validateSendRequest 里有 `totalAmountFen <= 0` 的校验
		{"负数", "-1", -100, false},
		{"空串", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := YuanToFen(tt.yuan)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际得到 %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("非预期错误: %v", err)
			}
			if got != tt.want {
				t.Errorf("YuanToFen(%q) = %d, 期望 %d", tt.yuan, got, tt.want)
			}
		})
	}
}

func TestConvertFenToYuan(t *testing.T) {
	tests := []struct {
		fen  int64
		want string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{10, "0.10"},
		{100, "1.00"},
		{1050, "10.50"},
		{1000000, "10000.00"},
	}

	for _, tt := range tests {
		if got := ConvertFenToYuan(tt.fen); got != tt.want {
			t.Errorf("ConvertFenToYuan(%d) = %q, 期望 %q", tt.fen, got, tt.want)
		}
	}
}

func TestYuanFenRoundTrip(t *testing.T) {
	// 元 → 分 → 元 必须无损，否则展示金额和实际扣款会对不上
	for _, yuan := range []string{"0.01", "0.99", "1.00", "88.88", "200.00"} {
		fen, err := YuanToFen(yuan)
		if err != nil {
			t.Fatalf("YuanToFen(%q): %v", yuan, err)
		}
		if got := ConvertFenToYuan(fen); got != yuan {
			t.Errorf("往返转换 %q → %d 分 → %q，金额不一致", yuan, fen, got)
		}
	}
}

func TestToInt64(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want int64
	}{
		{"int64", int64(42), 42},
		{"字符串数字", "123", 123},
		{"非数字字符串", "abc", 0},
		{"nil", nil, 0},
		{"bool 不支持", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToInt64(tt.in); got != tt.want {
				t.Errorf("ToInt64(%v) = %d, 期望 %d", tt.in, got, tt.want)
			}
		})
	}
}
