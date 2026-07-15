package logging

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/consumer"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/reader"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/toolkits/pkg/logger"
)

type Logging struct {
	r *reader.Reader     // 代表生产者
	c *consumer.Group    // 代表消费者
	S *model.LogStrategy // 策略
}

func (lj *Logging) hash() string {
	md5obj := md5.New()
	md5obj.Write([]byte(lj.S.MetricName))
	md5obj.Write([]byte(lj.S.FilePath))
	return hex.EncodeToString(md5obj.Sum(nil))
}

func (lj *Logging) start(cq chan *consumer.AnalystPoint) {
	filepath := lj.S.FilePath
	// stream
	stream := make(chan string, common.LogQueueSize)

	// reader
	r, err := reader.NewReader(filepath, stream)
	if err != nil {
		return
	}
	lj.r = r

	// consumer
	cg := consumer.NewGroup(filepath, stream, lj.S, cq)
	lj.c = cg
	// 启动 reader 和 consumer
	// 先消费者
	lj.c.Start()
	// 后生产者
	go r.Start()
	logger.Infof("[create logging start][s:%+v]", lj.S)
}

// 先停生产者后消费者
func (lj *Logging) stop() {
	lj.r.Stop()
	lj.c.Stop()
}
