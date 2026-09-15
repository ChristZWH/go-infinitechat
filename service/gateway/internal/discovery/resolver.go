package discovery

import (
	"context"
	"fmt"
	"go-infinitechat/common/common"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Resolver 监听 etcd 某个前缀下的所有服务实例

// 职责：
//   - 启动时全量加载一次实例列表
//   - 后台 watch 该前缀，感知服务上下线
//   - 对外提供 Next() 轮询选择一个实例

// 线程安全：实例列表用读写锁保护，读取路径无竞争

// 举例：
//   订阅 prefix = "/services/user.api"
//   etcd 里有两个 key:
//     /services/user.api/10.0.0.5:8104  =  "10.0.0.5:8104"
//     /services/user.api/10.0.0.6:8104  =  "10.0.0.6:8104"
//   Next() 会轮流返回 http://10.0.0.5:8104 和 http://10.0.0.6:8104

type Resolver struct {
	prefix string
	client *clientv3.Client

	mu       sync.RWMutex
	instance []*url.URL

	counter uint64 // 轮询计数器（原子累加）
	cancel  context.CancelFunc
}

// NewResolver 创建并初始化 Resolver
//
// 首次调用会阻塞等待 etcd 返回初始实例列表（5 秒超时），
// 之后 watch 在后台 goroutine 里运行
func NewResolver(cli *clientv3.Client, prefix string) (*Resolver, error) {
	if cli == nil {
		return nil, fmt.Errorf("etcd client 不能为 nil")
	}
	if prefix == "" {
		return nil, fmt.Errorf("prefix 不能为空")
	}

	r := &Resolver{
		prefix: prefix,
		client: cli,
	}

	// 1. 首次全量加载
	if err := r.loadAll(); err != nil {
		return nil, err
	}

	// 2. 启动 watch
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.watch(ctx)

	common.Infof("[Resolver] 订阅成功: prefix=%s, 初始实例数=%d", prefix, r.Size())
	return r, nil
}

// loadAll 全量拉取 prefix 下所有实例
func (r *Resolver) loadAll() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := r.client.Get(ctx, r.prefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("etcd 查询失败: prefix=%s, err=%w", r.prefix, err)
	}

	list := make([]*url.URL, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		if u := parseAddr(string(kv.Value)); u != nil {
			list = append(list, u)
		}
	}

	r.mu.Lock()
	r.instance = list
	r.mu.Unlock()
	return nil
}

// watch 后台监听 etcd 变更
// 为了实现简单，任何变更都触发一次全量 reload
// prefix 下的实例数通常是个位数，这点开销可以忽略
func (r *Resolver) watch(ctx context.Context) {
	ch := r.client.Watch(ctx, r.prefix, clientv3.WithPrefix())
	for resp := range ch {
		if err := resp.Err(); err != nil {
			common.Errorf("[Resolver] watch 错误: prefix=%s, err=%s", r.prefix, err.Error())
			continue
		}
		if err := r.loadAll(); err != nil {
			common.Errorf("[Resolver] 重新加载失败: %s", err.Error())
			continue
		}
		common.Infof("[Resolver] 实例列表更新: prefix=%s, 当前实例数=%d", r.prefix, r.Size())
	}
}

// HasInstances 是否存在可用实例
func (r *Resolver) HasInstance() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.instance) > 0
}

// Size 当前实例数
func (r *Resolver) Size() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.instance)
}

// Close 停止 watch（不关闭 etcd client，由调用方负责）
func (r *Resolver) Close() {
	if r != nil && r.cancel != nil {
		r.Close()
	}
}

// 将 "host:port" 或 "http(s)://host:port" 解析为 URL
//
// 没带 scheme 时默认补 http://
func parseAddr(addr string) *url.URL {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	if !strings.HasPrefix(addr, "http://") || !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	url, err := url.Parse(addr)
	if err != nil {
		common.Warnf("[Resolver] 解析地址失败: addr=%s, err=%s", addr, err.Error())
		return nil
	}
	return url
}

// Next 轮询选下一个实例
// 返回 nil 表示当前没有可用实例
func (r *Resolver) Next() *url.URL {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := len(r.instance)
	if n == 0 {
		return nil
	}

	idx := atomic.AddUint64(&r.counter, 1) - 1
	return r.instance[int(idx%uint64(n))]
}
