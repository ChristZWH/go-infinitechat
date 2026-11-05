package discovery

import (
	"net/url"
	"testing"
)

func TestParseAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string // 空串表示期望 nil
	}{
		{"裸 host:port 补 http", "10.0.0.5:8104", "http://10.0.0.5:8104"},
		{"带 http", "http://10.0.0.5:8104", "http://10.0.0.5:8104"},
		{"带 https", "https://10.0.0.5:8104", "https://10.0.0.5:8104"},
		{"首尾空格", "  10.0.0.5:8104  ", "http://10.0.0.5:8104"},
		{"空串", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAddr(tt.addr)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("parseAddr(%q) = %v, 期望 nil", tt.addr, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseAddr(%q) = nil, 期望 %s", tt.addr, tt.want)
			}
			if got.String() != tt.want {
				t.Fatalf("parseAddr(%q) = %q, 期望 %q", tt.addr, got.String(), tt.want)
			}
		})
	}
}

func TestNextRoundRobin(t *testing.T) {
	r := &Resolver{
		instance: []*url.URL{
			mustURL("http://a:1"),
			mustURL("http://b:2"),
			mustURL("http://c:3"),
		},
	}

	want := []string{"http://a:1", "http://b:2", "http://c:3", "http://a:1", "http://b:2", "http://c:3"}
	for i, w := range want {
		got := r.Next()
		if got == nil || got.String() != w {
			t.Fatalf("第 %d 次 Next() = %v, 期望 %s", i+1, got, w)
		}
	}
}

func TestNextEmpty(t *testing.T) {
	r := &Resolver{}
	if got := r.Next(); got != nil {
		t.Fatalf("无实例时 Next() = %v, 期望 nil", got)
	}
}

func TestHasInstanceAndSize(t *testing.T) {
	empty := &Resolver{}
	if empty.HasInstance() {
		t.Fatal("空 Resolver 不应有实例")
	}
	if empty.Size() != 0 {
		t.Fatalf("Size = %d, 期望 0", empty.Size())
	}

	full := &Resolver{instance: []*url.URL{mustURL("http://a:1"), mustURL("http://b:2")}}
	if !full.HasInstance() {
		t.Fatal("有实例时 HasInstance 应为 true")
	}
	if full.Size() != 2 {
		t.Fatalf("Size = %d, 期望 2", full.Size())
	}
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
