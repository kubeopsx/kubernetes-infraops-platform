package xprober

import (
	"testing"
	"time"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_model/go"
)

func TestCollectorPublishesGenericAndProtocolMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	collector, err := newCollector(registry)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stored := collector.Store([]*common.Result{{
		WorkerName:    "agent-1",
		MetricsName:   common.MetricsNameTCPConnectSuccess,
		TargetAddress: "127.0.0.1:8080",
		SourceRegion:  "source",
		TargetRegion:  "target",
		Type:          "tcp",
		TimeStamp:     now.Unix(),
		Value:         1,
	}})
	if stored != 1 {
		t.Fatalf("stored = %d, want 1", stored)
	}
	collector.Publish(now)

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	assertMetricValue(t, families, "infraops_probe_value", 1)
	assertMetricValue(t, families, common.MetricsNameTCPConnectSuccess, 1)
}

func TestCollectorExpiresOldResults(t *testing.T) {
	registry := prometheus.NewRegistry()
	collector, err := newCollector(registry)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	collector.Store([]*common.Result{{
		WorkerName: "agent-1", MetricsName: common.MetricsNameDNSLookupSuccess,
		TargetAddress: "example.com", Type: "dns", TimeStamp: now.Add(-resultTTL - time.Second).Unix(), Value: 1,
	}})
	collector.Publish(now)
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "infraops_probe_value" && len(family.Metric) != 0 {
			t.Fatalf("expired generic metric was still published")
		}
	}
}

func assertMetricValue(t *testing.T, families []*io_prometheus_client.MetricFamily, name string, want float64) {
	t.Helper()
	for _, family := range families {
		if family.GetName() != name || len(family.Metric) == 0 {
			continue
		}
		if got := family.Metric[0].GetGauge().GetValue(); got != want {
			t.Fatalf("metric %s = %v, want %v", name, got, want)
		}
		return
	}
	t.Fatalf("metric %s not found", name)
}
