package channelmgr

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	ws "github.com/gorilla/websocket"
)

// newTestClientConn 起一个最小 WebSocket 服务端并拨号，返回服务端侧的 ClientConn。
// 真实的 *ws.Conn 是必需的：AddUserConn 踢旧连接时会调用 Close()。
func newTestClientConn(t *testing.T) (*ClientConn, func()) {
	t.Helper()

	connCh := make(chan *ws.Conn, 1)
	releaseCh := make(chan struct{})
	var once sync.Once

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := ws.Upgrader{}
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		connCh <- c
		<-releaseCh
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("拨号失败: %v", err)
	}

	serverConn := <-connCh

	cleanup := func() {
		once.Do(func() {
			close(releaseCh)
			_ = client.Close()
			serverConn.Close()
			srv.Close()
		})
	}
	return NewClientConn(serverConn), cleanup
}

func newTestManager() *ChannelManger {
	return &ChannelManger{
		userConnMap: make(map[string]*ClientConn),
		connUserMap: make(map[*ClientConn]string),
	}
}

func TestAddAndGetUserConn(t *testing.T) {
	cm := newTestManager()
	conn, cleanup := newTestClientConn(t)
	defer cleanup()

	cm.AddUserConn("1001", conn)

	if got := cm.GetConnByUserId("1001"); got != conn {
		t.Fatalf("GetConnByUserId 返回 %v, 期望同一连接", got)
	}
	if got := cm.GetUserIdByConn(conn); got != "1001" {
		t.Fatalf("GetUserIdByConn = %q, 期望 1001", got)
	}
	if got := cm.OnlineCount(); got != 1 {
		t.Fatalf("OnlineCount = %d, 期望 1", got)
	}
	if got := cm.GetConnByUserId("not-exist"); got != nil {
		t.Fatalf("不存在的用户应返回 nil, 实际 %v", got)
	}
}

func TestAddUserConnKicksOldConnection(t *testing.T) {
	cm := newTestManager()
	oldConn, oldCleanup := newTestClientConn(t)
	defer oldCleanup()
	newConn, newCleanup := newTestClientConn(t)
	defer newCleanup()

	cm.AddUserConn("2001", oldConn)
	cm.AddUserConn("2001", newConn)

	if got := cm.GetConnByUserId("2001"); got != newConn {
		t.Fatal("新连接未生效")
	}
	// 旧连接必须被踢掉：既不在反向映射里，也已被关闭
	if got := cm.GetUserIdByConn(oldConn); got != "" {
		t.Errorf("旧连接的反向映射未清理，仍指向 %q", got)
	}
	if err := oldConn.WriteMessage(ws.TextMessage, []byte("ping")); err == nil {
		t.Error("旧连接的底层连接应已被关闭")
	}
	if got := cm.OnlineCount(); got != 1 {
		t.Errorf("OnlineCount = %d, 期望 1（同一用户只保留一条连接）", got)
	}
}

func TestRemoveByConnIsIdempotent(t *testing.T) {
	cm := newTestManager()
	conn, cleanup := newTestClientConn(t)
	defer cleanup()

	cm.AddUserConn("3001", conn)

	if got := cm.RemoveByConn(conn); got != "3001" {
		t.Fatalf("首次 RemoveByConn = %q, 期望 3001", got)
	}
	// 被新连接顶掉后再断开：反向映射已删，必须返回空串而不是误删新连接
	if got := cm.RemoveByConn(conn); got != "" {
		t.Fatalf("重复 RemoveByConn = %q, 期望空串", got)
	}
	if got := cm.OnlineCount(); got != 0 {
		t.Fatalf("OnlineCount = %d, 期望 0", got)
	}
}

func TestRemoveByUserId(t *testing.T) {
	cm := newTestManager()
	conn, cleanup := newTestClientConn(t)
	defer cleanup()

	cm.AddUserConn("4001", conn)
	cm.RemoveByUserId("4001")

	if cm.GetConnByUserId("4001") != nil {
		t.Error("RemoveByUserId 后仍能查到连接")
	}
	if cm.GetUserIdByConn(conn) != "" {
		t.Error("RemoveByUserId 未清理反向映射")
	}
	if cm.OnlineCount() != 0 {
		t.Error("RemoveByUserId 后在线数不为 0")
	}
}

// 用 -race 跑这个用例：并发读写 map 必须被 RWMutex 保护
func TestChannelManagerConcurrentAccess(t *testing.T) {
	cm := newTestManager()
	const users = 50

	conns := make([]*ClientConn, users)
	cleanups := make([]func(), users)
	for i := 0; i < users; i++ {
		conns[i], cleanups[i] = newTestClientConn(t)
	}
	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < users; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			userId := strings.Repeat("u", 1) + "-" + string(rune('a'+i%26))
			cm.AddUserConn(userId, conns[i])
			_ = cm.GetConnByUserId(userId)
			_ = cm.OnlineCount()
		}()
	}
	wg.Wait()

	for i := 0; i < users; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.RemoveByConn(conns[i])
			_ = cm.OnlineCount()
		}()
	}
	wg.Wait()
}
