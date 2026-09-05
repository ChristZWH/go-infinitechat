package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	virtualNodes = 160
	wsServerUri  = "/ws/chat"
)

type ringNode struct {
	hash uint32
	addr string
}

type WsServerLocator struct {
	mu   sync.RWMutex
	ring []ringNode
}

func NewWsServerLocator(endpints []string, prefix string) (*WsServerLocator, error) {
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpints})
	if err != nil {
		return nil, err
	}

	loc := &WsServerLocator{}

	// 首次拉取节点列表
	resp, err := cli.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var addrs []string
	for _, kv := range resp.Kvs {
		addrs = append(addrs, string(kv.Value))
	}
	loc.buildRing(addrs)

	// Watch 节点变化，动态重建哈希环
	go func() {
		watchCh := cli.Watch(context.Background(), prefix, clientv3.WithPrefix())
		for range watchCh {
			resp, err := cli.Get(context.Background(), prefix, clientv3.WithPrefix())
			if err != nil {
				continue
			}
			var newAddrs []string
			for _, kv := range resp.Kvs {
				newAddrs = append(newAddrs, string(kv.Value))
			}
			loc.buildRing(newAddrs)
		}
	}()

	return loc, nil
}

func (l *WsServerLocator) buildRing(addrs []string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var nodes []ringNode
	for _, addr := range addrs {
		for i := 0; i < virtualNodes; i++ {
			h := hashStr(fmt.Sprintf("%s#%d", addr, i))
			nodes = append(nodes, ringNode{hash: h, addr: addr})
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].hash < nodes[j].hash
	})
	l.ring = nodes
}

func (l *WsServerLocator) GetWsServerUri(userId string) string {
	l.mu.RLock()
	defer l.mu.Unlock()

	if len(l.ring) == 0 {
		return ""
	}

	h := hashStr(userId)
	idx := sort.Search(len(l.ring), func(i int) bool {
		return l.ring[i].hash >= h
	})
	return fmt.Sprintf("ws://%s%s", l.ring[idx].addr, wsServerUri)
}

func hashStr(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
