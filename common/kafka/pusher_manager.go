package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"go-infinitechat/common/common"

	"github.com/zeromicro/go-queue/kq"
)

type PusherManager struct {
	pushers map[string]*kq.Pusher
}

func NewPusherManager(brokers []string, topics []string) *PusherManager {
	pm := &PusherManager{
		pushers: make(map[string]*kq.Pusher),
	}

	for _, topic := range topics {
		pusher := kq.NewPusher(brokers, topic)
		pm.pushers[topic] = pusher
	}

	return pm
}

// 发送消息到指定topic（无key）
func (pm *PusherManager) Push(ctx context.Context, topic string, message interface{}) error {
	pusher, ok := pm.pushers[topic]
	if !ok {
		common.Errorf("Kafka Pusher 未找到, topic: %s", topic)
		return fmt.Errorf("Kafka Pusher 未找到, topic: %s", topic)
	}

	jsonBytes, err := json.Marshal(message)
	if err != nil {
		common.Errorf("消息序列化失败: %s", err.Error())
		return err
	}

	if err := pusher.Push(ctx, string(jsonBytes)); err != nil {
		common.Errorf("Kafka消息发送失败, topic: %s, error: %s", topic, err.Error())
		return err
	}

	// common.Infof("Kafka消息发送成功, topic: %s", topic)
	return nil
}

// 发送带Key的消息到指定topic。使用key保证同一个key的消息发送到同一个分区，保证顺序性
func (pm *PusherManager) KPush(ctx context.Context, topic string, key string, message interface{}) error {
	pusher, ok := pm.pushers[topic]
	if !ok {
		common.Errorf("Kafka Pusher 未找到, topic: %s", topic)
		return fmt.Errorf("Kafka Pusher 未找到, topic: %s", topic)
	}

	jsonBytes, err := json.Marshal(message)
	if err != nil {
		common.Errorf("消息序列化失败: %s", err.Error())
		return err
	}

	if err := pusher.KPush(ctx, key, string(jsonBytes)); err != nil {
		common.Errorf("Kafka消息发送失败, topic: %s, key: %s, error: %s", topic, key, err.Error())
		return err
	}

	// 成功暂不入日志
	// common.Infof("Kafka消息发送成功, topic: %s, key: %s", topic, key)
	return nil
}

// 关闭所有Pusher
func (pm *PusherManager) Close() {
	for topic, pusher := range pm.pushers {
		if err := pusher.Close(); err != nil {
			common.Errorf("关闭Kafka Pusher失败, topic: %s, error: %s", topic, err.Error())
		}
	}
}
