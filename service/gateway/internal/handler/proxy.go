package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go-infinitechat/common/common"
	"go-infinitechat/service/gateway/internal/config"
	"go-infinitechat/service/gateway/internal/discovery"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// 反向代理路由器
//
// 支持两种上游模式（由 Upstream 的 scheme 决定）：
//   - http://host:port     静态单实例，沿用原来的行为
//   - etcd:///services/xxx 从 etcd 订阅实例列表，轮询负载均衡
//
// 匹配策略：按 Prefix 从上到下匹配，命中即停

// RouteEntry 一条路由规则
type RouteEntry struct {
	Id       string
	Prefixes []string

	// StaticUpstream 		http:// 模式时有值
	StaticUpstream *url.URL
	// Resolver 					etcd:// 模式时有值
	Resolver *discovery.Resolver
	// Proxy 							两种模式都用同一个 RecoverProxy,Director 里面做分支
	Proxy *httputil.ReverseProxy
}

// ProxyRouter 反向代理路由器
type ProxyRouter struct {
	routes  []*RouteEntry
	etcdCli *clientv3.Client // 可以为 nil（没配 etcd 时）
}

// NewProxyRouter 构建路由器
// etcdCli 可以为 nil，但那样 etcd:// 开头的路由会跳过
func NewProxyRouter(routeConfs []config.RouteConfig, timeoutConf config.TimeoutConfig, etcdCli *clientv3.Client) *ProxyRouter {
	pr := &ProxyRouter{etcdCli: etcdCli}
	for _, rc := range routeConfs {
		entry, err := buildRoute(rc, timeoutConf, etcdCli)
		if err != nil {
			common.Errorf("[Gateway] 路由构建失败: id=%s, upstream=%s, err=%s", rc.Id, rc.Upstream, err.Error())
			continue
		}
		pr.routes = append(pr.routes, entry)
		common.Warnf("[Gateway] 注册路由: id=%s, upstream=%s, prefixes=%v", rc.Id, rc.Upstream, rc.Prefixes)
	}
	return pr
}

// buildRoute 根据单条配置构建 RouteEntry
func buildRoute(rc config.RouteConfig, timeoutConf config.TimeoutConfig, etcdCli *clientv3.Client) (*RouteEntry, error) {
	entry := &RouteEntry{
		Id:       rc.Id,
		Prefixes: rc.Prefixes,
	}

	// 公共 Transport: 所有路由共享同一套连接池参数
	// 但是实例是各自独立的，四条路由 - 四个连接池管理器，持有 TCP 连接
	transport := &http.Transport{
		ResponseHeaderTimeout: time.Duration(timeoutConf.Response) * time.Millisecond,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
	}

	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		common.Errorf("[Gateway]代理失败: id = %s, path = %s, err = %s", rc.Id, r.URL.Path, err.Error())
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":502,"message":"网关错误，上有服务不可用"}`))
	}

	switch {
	case strings.HasPrefix(rc.Upstream, "etcd://"):
		// 动态模式
		if etcdCli == nil {
			return nil, fmt.Errorf("etcd 未配置，无法处理 etcd:// 路由")
		}
		// "etcd:///services/user.api" → prefix = "/services/user.api"
		prefix := strings.TrimPrefix(rc.Upstream, "etcd://")
		resolver, err := discovery.NewResolver(etcdCli, prefix)
		if err != nil {
			return nil, err
		}
		entry.Resolver = resolver

		entry.Proxy = &httputil.ReverseProxy{
			Transport:    transport,
			ErrorHandler: errorHandler,
			// 每次请求都从 resolver 拿一个新的 upstream
			// Director 是每个请求转发前的回调，只做一件事：改写请求的去向
			Director: func(req *http.Request) {
				u := resolver.Next() // 挑一个上游地址（轮询）
				if u == nil {
					// 没有可用实例，把 URL 置空触发 RoundTrip 失败 → ErrorHandler
					req.URL.Scheme = ""
					req.URL.Host = ""
					req.Host = ""
					return
				}
				req.URL.Scheme = u.Scheme
				req.URL.Host = u.Host
				req.Host = u.Host
			},
		}
	default:
		// 静态模式
		// 兜底：没写 scheme 的给一个友好提示
		if !strings.HasPrefix(rc.Upstream, "http://") && !strings.HasPrefix(rc.Upstream, "https://") {
			return nil, fmt.Errorf("upstream scheme 不支持: %s（只支持 http/https/etcd）", rc.Upstream)
		}
		upstream, err := url.Parse(rc.Upstream)
		if err != nil {
			return nil, err
		}
		entry.StaticUpstream = upstream

		proxy := httputil.NewSingleHostReverseProxy(upstream)
		proxy.Transport = transport
		proxy.ErrorHandler = errorHandler
		defaultDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			defaultDirector(req)
			req.Host = upstream.Host
		}
		entry.Proxy = proxy
	}

	return entry, nil
}

// ServeHTTP 处理请求转发
func (pr *ProxyRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := pr.FindRoutes(r.URL.Path)
	if route == nil {
		common.Warnf("[Gateway] 未匹配到路由: path=%s", r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"未找到对应的服务路由"}`))
		return
	}

	// 动态模式下提前检查实例数，直接返回 503（更清晰的错误语义）
	if route.Resolver != nil && !route.Resolver.HasInstance() {
		common.Errorf("[Gateway] 服务无可用实例: id=%s, path=%s", route.Id, r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":503,"message":"服务暂时不可用"}`))
		return
	}

	common.Infof("[Gateway] 路由匹配: path=%s -> %s", r.URL.Path, route.Id)
	route.Proxy.ServeHTTP(w, r)
}

// FindRoute 按路径前缀查找路由
func (pr *ProxyRouter) FindRoutes(path string) *RouteEntry {
	for _, r := range pr.routes {
		for _, prefix := range r.Prefixes {
			if strings.HasPrefix(path, prefix) {
				return r
			}
		}
	}
	return nil
}

// Close 释放所有 resolver 资源
func (pr *ProxyRouter) Close() {
	for _, r := range pr.routes {
		if r.Resolver != nil {
			r.Resolver.Close()
		}
	}
}
