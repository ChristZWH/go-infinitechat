package etcd

import (
	"context"
	"fmt"
	"go-infinitechat/common/common"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// 通用的 HTTP 服务注册器
//
// 设计目的：
//   让 user.api / offline.api / redpacket.api 等 HTTP 服务能够像
//   user.rpc / realtimeservice 一样注册到 etcd，供网关做服务发现。
//
// 使用方式：
//   reg, err := etcd.RegisterHTTPService(etcd.RegisterOptions{
//       Endpoints: cfg.Etcd.Endpoints,
//       Key:       "/services/user.api",
//       Addr:      "10.0.0.5:8104",
//   })
//   defer reg.Close()  // 进程退出时主动注销

// RegisterOptions 注册参数
type RegisterOptions struct {
	// etcd 集群地址
	Endpoints []string
	// Key 服务的 key 前缀（例如/services/user.api）
	// 最终在 etcd 里写入 的 key 为 {Key}/{Addr}
	Key string
	// Addr 本实验的对外访问地址（host:port 或 ip:port）
	// 	注意：不要写 0.0.0.0，必须是其他进程能访问到的地址
	Addr string
	// TTL 租约存活时间（秒），默认 30
	TTL int64
}

// Register 服务注册句柄，进程退出前调用 Close() 可以主动下线
type Register struct {
	client  *clientv3.Client
	leaseID clientv3.LeaseID
	fullKey string
	cancel  context.CancelFunc
}

// RegisterHTTPService 将当前服务注册到 etcd
//
// 工作流程：
//  1. 连接 etcd
//  2. 创建 TTL 租约
//  3. 写入 {Key}/{Addr} = {Addr}，绑定租约
//  4. 后台 KeepAlive 持续续约
//
// 如果 Endpoints 为空：不报错、不注册，直接返回 (nil, nil)
// 这样本地开发时没启 etcd 也不会阻塞启动
func RegisterHTTPService(opts RegisterOptions) (*Register, error) {
	if len(opts.Endpoints) == 0 {
		common.Warn("[etcd-register] Endpoints 未配置，跳过服务注册")
		return nil, nil
	}
	if opts.Addr == "" {
		return nil, fmt.Errorf("addr 不能为空")
	}
	if opts.Key == "" {
		return nil, fmt.Errorf("key 不能为空")
	}
	if opts.TTL <= 0 {
		opts.TTL = 30
	}

	// 1. 连接 etcd
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   opts.Endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd 连接失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 2. 创建租约
	leaseResp, err := cli.Grant(ctx, opts.TTL)
	if err != nil {
		cancel()
		// 关闭连接
		_ = cli.Close()
		return nil, fmt.Errorf("etcd 创建租约失败: %w", err)
	}

	// 3. 写入 key，绑定租约
	fullKey := fmt.Sprintf("%s/%s", opts.Key, opts.Addr)
	if _, err := cli.Put(ctx, fullKey, opts.Addr, clientv3.WithLease(leaseResp.ID)); err != nil {
		cancel()
		_ = cli.Close()
		return nil, fmt.Errorf("etcd 注册失败: %w", err)
	}

	// 4. 启动 KeepAlive
	ch, err := cli.KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		cancel()
		_ = cli.Close()
		return nil, fmt.Errorf("etcd KeepAlive 启动失败: %w", err)
	}

	// 必须持续消费 KeepAlive channel，否则底层阻塞导致续约暂停
	go func() {
		for range ch {
			// 续约成功，静默
		}
		common.Warnf("[etcd-register] KeepAlive channel 关闭，注册可能已失效: key=%s", fullKey)
	}()

	common.Infof("[etcd-register] 注册成功: key=%s, addr=%s, ttl=%ds", fullKey, opts.Addr, opts.TTL)

	return &Register{
		client:  cli,
		leaseID: leaseResp.ID,
		fullKey: fullKey,
		cancel:  cancel,
	}, nil
}

func (r *Register) Close() {
	if r == nil || r.client == nil {
		return
	}
	if r.cancel != nil {
		r.cancel()
	}

	// 主动撤销租约
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// 撤销
	_, _ = r.client.Revoke(ctx, r.leaseID)
	_ = r.client.Close()
	common.Infof("[etcd-register] 已注销: key=%s", r.fullKey)
}
