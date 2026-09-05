package utils

import (
	"go-infinitechat/common/common"
	"sync"

	"github.com/bwmarrin/snowflake"
)

const (
	WorkerID     = 1
	DataCenterID = 1
)

var (
	node *snowflake.Node
	once sync.Once
)

func initNode() {
	once.Do(func() {
		nodeID := (WorkerID << 5) | DataCenterID
		var err error
		node, err = snowflake.NewNode(int64(nodeID))
		if err != nil {
			common.Errorf("初始化 Snowflake 节点失败：%v", err)
		} else {
			common.Infof("Snowflake 节点初始化完成")
		}
	})
}

func NextInt() int64 {
	initNode()
	return node.Generate().Int64()
}

func NextString() string {
	initNode()
	return node.Generate().String()
}
