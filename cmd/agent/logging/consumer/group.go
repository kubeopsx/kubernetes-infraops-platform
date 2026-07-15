package consumer

import (
	"fmt"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/toolkits/pkg/logger"
)

type Group struct {
	ConsumerNum int
	Consumers   []*Consumer
}

func NewGroup(filePath string, stream chan string, s *model.LogStrategy, cq chan *AnalystPoint) *Group {
	consumerNum := common.LogConsumerNum
	cg := &Group{
		ConsumerNum: consumerNum,
		Consumers:   make([]*Consumer, 0),
	}
	logger.Infof("new worker group, [file:%s][num:%d]", filePath, consumerNum)

	for i := 0; i < consumerNum; i++ {
		mark := fmt.Sprintf("[consumer][file:%s][num:%d/%d]", filePath, i+1, consumerNum)
		c := &Consumer{
			FilePath:     filePath,
			Stream:       stream,
			S:            s,
			Mark:         mark,
			CounterQueue: cq,
			Close:        make(chan struct{}),
		}
		cg.Consumers = append(cg.Consumers, c)
	}
	return cg
}

func (g *Group) Start() {
	for i := 0; i < g.ConsumerNum; i++ {
		g.Consumers[i].Start()
	}
}

func (g *Group) Stop() {
	for i := 0; i < g.ConsumerNum; i++ {
		g.Consumers[i].Stop()
	}
}
