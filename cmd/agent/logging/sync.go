package logging

import (
	"context"
	"encoding/json"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/rpc"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/config/agent"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/toolkits/pkg/logger"
	"time"
)

func TickerLoggingSync(client *rpc.Client, ctx context.Context, syncChan chan []*Logging, localConfig []*Logging, metricsMap map[string]*prometheus.GaugeVec, hostname string) error {
	ticker := time.NewTicker(5 * time.Second)
	doSync(client, syncChan, localConfig, metricsMap, hostname)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			doSync(client, syncChan, localConfig, metricsMap, hostname)
		}
	}
}

func doSync(client *rpc.Client, syncChan chan []*Logging, localConfig []*Logging, metricsMap map[string]*prometheus.GaugeVec, hostname string) {
	result := client.Sync(hostname)
	ls := []*model.LogStrategy{}
	for _, i := range result {
		i := i
		m := map[string]string{}
		json.Unmarshal([]byte(i.TagJson), &m)
		i.Tags = m
		labels := []string{}
		for k := range i.Tags {
			labels = append(labels, k)
		}
		me := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: i.MetricName,
			Help: i.MetricHelp,
		}, labels)
		// 为了动态注册，防止重复注册，用map
		if _, loaded := metricsMap[i.MetricName]; !loaded {
			prometheus.MustRegister(me)
			metricsMap[i.MetricName] = me
		}
		ls = append(ls, i)
		logger.Infof("doSync rpc result num:%d result:%+v tags:%+v", len(result), i, i.Tags)
	}
	newLs := agent.SetLogRegs(ls)
	for _, i := range newLs {
		j := &Logging{
			S: i,
		}
		localConfig = append(localConfig, j)
	}
	syncChan <- localConfig
}
