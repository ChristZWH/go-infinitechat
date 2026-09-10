// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"context"
	"fmt"
	"go-infinitechat/common/common"
	"go-infinitechat/common/model/constants"
	"go-infinitechat/service/realtime/internal/channelmgr"
	"go-infinitechat/service/realtime/internal/config"
	"go-infinitechat/service/user/rpc/userrpc"
	"time"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type ServiceContext struct {
	Config config.Config
	// Redis 客户端
	Redis *redis.Redis

	// WebSocket 连接管理器（全局单例）,维护 userId ↔ WebSocket连接 的双向映射
	ChannelManager *channelmgr.ChannelManger

	// Kafka 生产者 - store-topic
	// 用途：将用户发的消息发送到 "store-topic"，由 OfflineDataService 消费并存入 MySQL
	StorePusher *kq.Pusher

	// Kafka 生产者 - message-topic
	// 用途：将用户发的消息发送到 "message-topic"，由Consumer 消费并推送给在线用户
	// 使用 sessionId 作为 key 保证同一会话的消息有序
	MessagePusher *kq.Pusher

	// Kafka 生产者 - store-notification-topic
	// 用途：当系统通知的接收者离线时，将通知转发到此 topic 进行持久化
	// 用户下次上线时，客户端会拉取离线通知
	NotificationStorePusher *kq.Pusher

	// Etcd 客户端
	// 用途：服务注册（把本节点地址写入 etcd，让 UserService 能发现并分配用户连接）
	EtcdClient *clientv3.Client

	// User RPC 客户端
	UserRpc userrpc.UserRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	// ============ 1. Redis ============
	rds := redis.MustNewRedis(c.Redis)

	// Kafks 生产者
	// 创建三个Pusher，分别对应三个 topic：
	// 		store-topic：消息持久化（写入 Mysql）
	// 		message-topic：消息推送（发给在线用户）
	// 		store-notification-topic：离线消息通知持久化（用户离线时转发通知到这里）
	var storePusher, messagePusher, notificationPusher *kq.Pusher
	if len(c.Kafka.Brokers) > 0 {
		storePusher = kq.NewPusher(c.Kafka.Brokers, constants.KafkaMessageTopicStore)
		messagePusher = kq.NewPusher(c.Kafka.Brokers, constants.KafkaMessageTopicPush)
		notificationPusher = kq.NewPusher(c.Kafka.Brokers, constants.KafkaSystemNotificationTopic)
		common.Info("Kafka Pushers 初始化完成")
	} else {
		common.Warn("Kafka Brokers 未配置，消息将无法持久化和推送")
	}

	// Etcd 服务注册
	var etcdClient *clientv3.Client
	if len(c.Etcd.Endpoints) > 0 {
		eli, err := clientv3.New(clientv3.Config{Endpoints: c.Etcd.Endpoints})
		if err != nil {
			common.Errorf("Etcd 连接失败: %s", err.Error())
		} else {
			etcdClient = eli
			registerService(etcdClient, c)
		}
	}

	// User RPC 客户端
	// go-zero 从 etcd 自动发现 user.rpc 地址，支持负载均衡
	userRpc := userrpc.NewUserRpc(zrpc.MustNewClient(c.UserRpc))
	common.Info("User RPC 客户端初始化成功......")

	return &ServiceContext{
		Config:                  c,
		Redis:                   rds,
		ChannelManager:          channelmgr.GetManager(),
		StorePusher:             storePusher,
		MessagePusher:           messagePusher,
		NotificationStorePusher: notificationPusher,
		EtcdClient:              etcdClient,
		UserRpc:                 userRpc,
	}
}

// 将当前 WebSocket 节点注册到 etcd
// 为什么要注册？
//
//	当部署多个 RealTimeService 实例时（如 3 个节点），UserService 需要知道
//	有哪些 WS 节点可用，然后通过一致性哈希算法决定某个用户应该连接到哪个节点。
//	用户登录时，UserService 从 etcd 读取节点列表 → 哈希选节点 → 返回 ws://ip:port/ws/chat 给客户端
//
// 注册信息：
//
//	Key:   /services/realtimeservice/ip:9101
//	Value: ip:9101
//
// 保活机制：
//
//	 创建一个 30 秒 TTL 的租约，并开启自动续约（KeepAlive）。
//	如果本节点宕机，30 秒后 etcd 自动删除该 key，其他服务就知道这个节点不可用了。
func registerService(cli *clientv3.Client, c config.Config) {
	// 拼接节点地址 -> 127.0.0.1:9101
	addr := fmt.Sprintf("%s:%d", c.Etcd.PublicIP, c.WebSocket.Port)
	// 拼接 etcd key -> /services/realtimeservice/127.0.0.1:9101
	key := fmt.Sprintf("%s/%s", c.Etcd.Prefix, addr)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建租约：30秒 TTL，到期自动删除（除非续约）
	lease, err := cli.Grant(ctx, 30)
	if err != nil {
		common.Errorf("Etcd 创建租约失败：%s", err.Error())
		return
	}

	// 写入 etcd，绑定租约
	_, err = cli.Put(ctx, key, addr, clientv3.WithLease(lease.ID))
	if err != nil {
		common.Errorf("Etcd 注册服务失败: %s", err.Error())
		return
	}

	ch, err := cli.KeepAlive(ctx, lease.ID)
	if err != nil {
		common.Errorf("Etcd KeepAlive 失败: %s", err.Error())
		return
	}

	// 必须消费 KeepAlive 返回的 channel，否则会阻塞导致续约停止
	go func() {
		for range ch {
			// 续约成功，静默处理（不打日志，避免每 10 秒刷一行）
		}
		// channel 关闭说明 etcd 连接断了，服务注册可能已失效
		common.Warn("Etcd KeepAlive channel 关闭，服务注册可能已失效")
	}()

	common.Infof("Etcd 服务注册成功: key=%s, value=%s", key, addr)
}
