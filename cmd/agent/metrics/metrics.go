package metrics

import (
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

func Create(obj []*model.LogStrategy) map[string]*prometheus.GaugeVec {
	mmap := make(map[string]*prometheus.GaugeVec)
	for _, s := range obj {
		labels := []string{}
		for k := range s.Tags {
			labels = append(labels, k)
		}
		m := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: s.MetricName,
			Help: s.MetricHelp,
		}, labels)
		mmap[s.MetricName] = m
	}
	return mmap
}

func Start(addr string) error {
	http.Handle("/metrics", promhttp.Handler())
	srv := http.Server{Addr: addr}
	err := srv.ListenAndServe()
	return err
}
