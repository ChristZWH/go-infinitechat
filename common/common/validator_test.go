package common

import "testing"

func TestParseValidationMsg(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		tag  string
		want string
	}{
		{"空消息", "", "required", ""},
		{"单条 = 格式", "required=用户ID不能为空", "required", "用户ID不能为空"},
		{"多段命中第二个", "required=不能为空; min=太短了", "min", "太短了"},
		{"通配符 *", "*=通用提示", "required", "通用提示"},
		{"冒号格式", "min:3", "min", "3"},
		{"无键值对原样返回", "长度必须大于0", "length", "长度必须大于0"},
		{"单对不校验 tag 直接返回值", "required=不能为空", "min", "不能为空"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseValidationMsg(tt.msg, tt.tag); got != tt.want {
				t.Fatalf("parseValidationMsg(%q, %q) = %q, 期望 %q", tt.msg, tt.tag, got, tt.want)
			}
		})
	}
}
