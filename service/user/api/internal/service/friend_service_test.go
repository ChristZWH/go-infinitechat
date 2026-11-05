package service

import (
	"database/sql"
	"testing"
)

func TestNullToEmpty(t *testing.T) {
	if got := nullToEmpty(sql.NullString{Valid: false}); got != "" {
		t.Fatalf("NULL 应转成空串, 得到 %q", got)
	}
	if got := nullToEmpty(sql.NullString{String: "alice", Valid: true}); got != "alice" {
		t.Fatalf("有效值应原样返回, 得到 %q", got)
	}
}

// evictFriendCache 在 rds 为 nil 时（本地开发未接 Redis）不能 panic
func TestEvictFriendCacheNilRedis(t *testing.T) {
	fs := &FriendService{}
	fs.evictFriendCache(1, 2) // 只断言不 panic
}
