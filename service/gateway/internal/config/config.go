// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	Host string
	Port int

	Redis redis.RedisConf

	// Etcd 服务发现配置（可选）
	// Etcdpoints 为空时，所有 etcd:// 开头的路由都无法解析
	// 只有 http://开头的静态路由能能工作
	Etcd EtcdConfig `json:",optional"`

	Auth    AuthConfig
	Cors    CorsConfig
	Routes  []RouteConfig
	Timeout TimeoutConfig
	rest.RestConf
}

// EtcdConfig etcd 配置
type EtcdConfig struct {
	Endpoints []string `json:",optional"`
}

// 认证配置
type AuthConfig struct {
	WhiteList []string
}

// 跨域配置
type CorsConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   string
	AllowCredentials bool
	MaxAge           int
}

// RouteConfig 路由配置
//
// Upstream 支持两种写法：
//
//  1. etcd://{RegisterKey} 动态服务发现
//     例："etcd://services/user.api"
//     网关会订阅 etcd 前缀 /services/user.api，扫描到所有实例，按轮询转发
//  2. http://{host}:{port}   静态单实例
//     例: "http://localhost:8886"
type RouteConfig struct {
	Id       string
	Upstream string
	Prefixes []string
}

// 超时配置（毫秒）
type TimeoutConfig struct {
	Connect  int
	Response int
}
