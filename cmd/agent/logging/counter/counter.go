package counter

import (
	"context"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/consumer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/toolkits/pkg/logger"
	"math"
	"sync"
	"time"
)

type PointCounter struct {
	sync.RWMutex
	Count           int64
	Sum             float64
	Max             float64
	Min             float64
	Ts              int64
	LogFunc         string
	MetricsName     string
	SortLabelString string
	LabelMap        map[string]string
}

func NewPointCounter(metricsName, sortLabel, logFunc string, labelMap map[string]string) *PointCounter {
	pc := &PointCounter{
		MetricsName:     metricsName,
		SortLabelString: sortLabel,
		LabelMap:        labelMap,
		LogFunc:         logFunc,
	}
	return pc
}

func (pc *PointCounter) Update(value float64) {
	pc.Lock()
	defer pc.Unlock()

	pc.Sum = pc.Sum + value

	if math.IsNaN(pc.Max) || value > pc.Max {
		pc.Max = value
	}
	if math.IsNaN(pc.Min) || value < pc.Min {
		pc.Min = value
	}

	pc.Count += 1
	pc.Ts = time.Now().Unix()
}

type PointCounterManager struct {
	sync.RWMutex
	TagStringMap map[string]*PointCounter
	CounterQueue chan *consumer.AnalystPoint
	MetricsMap   map[string]*prometheus.GaugeVec
}

func NewPointCounterManager(cq chan *consumer.AnalystPoint, m map[string]*prometheus.GaugeVec) *PointCounterManager {
	pcm := &PointCounterManager{
		TagStringMap: make(map[string]*PointCounter),
		CounterQueue: cq,
		MetricsMap:   m,
	}
	return pcm
}

func (pcm *PointCounterManager) GetPcByUniqueName(seriesId string) *PointCounter {
	pcm.Lock()
	defer pcm.Unlock()
	return pcm.TagStringMap[seriesId]
}

func (pcm *PointCounterManager) SetPc(seriesId string, pc *PointCounter) {
	pcm.Lock()
	defer pcm.Unlock()
	pcm.TagStringMap[seriesId] = pc
}

func (pcm *PointCounterManager) Update(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case ap := <-pcm.CounterQueue:
			p := pcm.GetPcByUniqueName(ap.MetricsName + ap.SortLabelString)
			if p == nil {
				p = NewPointCounter(ap.MetricsName, ap.SortLabelString, ap.LogFunc, ap.LabelMap)
				pcm.SetPc(ap.MetricsName+ap.SortLabelString, p)
			}
			p.Update(ap.Value)
		}
	}
}

func (pcm *PointCounterManager) SetMetrics() {
	pcm.Lock()
	defer pcm.Unlock()

	for _, p := range pcm.TagStringMap {
		p := p
		metric, loaded := pcm.MetricsMap[p.MetricsName]
		if !loaded {
			logger.Errorf("metrics not found [name:%v]", p.MetricsName)
			continue
		}
		logger.Infof("[set metrics][pc:%+v]", pcm)

		var value float64
		switch p.LogFunc {
		case common.LogFuncCnt:
			value = float64(p.Count)
		case common.LogFuncSum:
			value = float64(p.Sum)
		case common.LogFuncMax:
			value = float64(p.Max)
		case common.LogFuncMin:
			value = float64(p.Min)
		case common.LogFuncAvg:
			value = float64(p.Sum) / float64(p.Count)
		}
		metric.With(prometheus.Labels(p.LabelMap)).Set(value)
	}
}

func (pcm *PointCounterManager) SetMetricsManager(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pcm.SetMetrics()
		}
	}
}
