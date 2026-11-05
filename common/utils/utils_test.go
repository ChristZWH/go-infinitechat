package utils

import (
	"testing"

	"go-infinitechat/common/common"
)

func TestSuccess(t *testing.T) {
	resp := Success("data")
	if resp.Code != 200 || resp.Message != "ok" || resp.Data != "data" {
		t.Fatalf("Success 响应不正确: %+v", resp)
	}
}

func TestFail(t *testing.T) {
	resp := Fail(common.ParamsError)
	if resp.Code != common.ParamsError.Code || resp.Message != common.ParamsError.Message {
		t.Fatalf("Fail 响应不正确: %+v", resp)
	}
}

func TestFailWithCode(t *testing.T) {
	resp := FailWithCode(40300, "无权访问")
	if resp.Code != 40300 || resp.Message != "无权访问" {
		t.Fatalf("FailWithCode 响应不正确: %+v", resp)
	}
}

func TestRandomNumber(t *testing.T) {
	for _, l := range []int{1, 4, 6, 10} {
		s, err := RandomNumber(l)
		if err != nil {
			t.Fatalf("RandomNumber(%d): %v", l, err)
		}
		if len(s) != l {
			t.Fatalf("RandomNumber(%d) 长度 = %d", l, len(s))
		}
		for _, c := range s {
			if c < '0' || c > '9' {
				t.Fatalf("RandomNumber(%d) 含非数字字符: %q", l, s)
			}
		}
	}
}

func TestSnowflakeNextIntUnique(t *testing.T) {
	seen := make(map[int64]bool)
	for i := 0; i < 1000; i++ {
		id := NextInt()
		if id <= 0 {
			t.Fatalf("雪花 ID 必须为正数, 得到 %d", id)
		}
		if seen[id] {
			t.Fatalf("雪花 ID 重复: %d", id)
		}
		seen[id] = true
	}
}

func TestSnowflakeNextString(t *testing.T) {
	s := NextString()
	if s == "" || s == "0" {
		t.Fatalf("雪花字符串 ID 异常: %q", s)
	}
	if NextString() == s {
		t.Fatal("连续两次雪花字符串 ID 不应相同")
	}
}
