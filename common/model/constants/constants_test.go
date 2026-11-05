package constants

import "testing"

func TestFriendStatusCacheKey(t *testing.T) {
	// 读取端（user-rpc）和清理端（user-api）必须得到完全相同的 key，
	// 否则删缓存永远删不掉，拉黑/删好友后旧状态会一直生效。
	tests := []struct {
		userId   int64
		friendId int64
		want     string
	}{
		{1, 23, "msg:validate:friend:status:1:23"},
		{12, 3, "msg:validate:friend:status:12:3"},
		{100, 200, "msg:validate:friend:status:100:200"},
	}

	for _, tt := range tests {
		got := FriendStatusCacheKey(tt.userId, tt.friendId)
		if got != tt.want {
			t.Errorf("FriendStatusCacheKey(%d,%d) = %q, 期望 %q", tt.userId, tt.friendId, got, tt.want)
		}
	}
}

func TestFriendStatusCacheKeyIsUnique(t *testing.T) {
	// 曾经的 bug：key = prefix + userId + friendId（没有分隔符），
	// (1,23) 与 (12,3) 会拼成同一个 key，导致按别人的好友关系放行/拦截。
	cases := [][2]int64{
		{1, 23}, {12, 3},
		{1, 234}, {12, 34}, {123, 4},
		{11, 1}, {1, 11},
	}

	seen := make(map[string][2]int64, len(cases))
	for _, c := range cases {
		key := FriendStatusCacheKey(c[0], c[1])
		if prev, ok := seen[key]; ok {
			t.Fatalf("(%d,%d) 与 (%d,%d) 生成了相同的缓存 key: %s", prev[0], prev[1], c[0], c[1], key)
		}
		seen[key] = c
	}
}

func TestFriendStatusCacheKeyDirectional(t *testing.T) {
	// A→B 与 B→A 是两条不同的好友关系记录，key 必须不同
	if FriendStatusCacheKey(1, 2) == FriendStatusCacheKey(2, 1) {
		t.Fatal("双向好友关系不应共用同一个缓存 key")
	}
}
