// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	// REST 服务基础配置
	rest.RestConf

	// Redis 连接配置
	Redis redis.RedisConf

	//go-zero 自带的缓存配置
	Cache cache.CacheConf `json:",optional"`

	// WebSocket 服务配置，独立于 REST 服务，监听在不同端口（9101）
	WebSocket struct {
		// WebSocket 监听端口，客户端通过 ws://ip:9101/ws/chat 连接
		Port int
		// WebSocket 路径
		Path string
	}

	// Kafka 配置
	Kafka struct {
		// Kafka Broker 地址列表
		Brokers []string `json:",optional"`
		// 消息推送消费者组
		MessageConsumerGroup string `json:",default=infinite-chat-push-group-9"`
		// 系统通知消费者组
		NotificationConsumerGroup string `json:",default=system-notification-consumer-group"`
	}

	// Etcd 注册中心的配置
	Etcd struct {
		// Etcd 集群地址列表
		Endpoints []string
		// 服务注册的 key 前缀
		Prefix string
		// 当前节点的公网 IP，写入 etcd 和 Redis 时用，让其他服务和客户端能通过此 IP 连接过来
		PublicIP string
	}

	// User RPC 客户端配置
	UserRpc zrpc.RpcClientConf
}
