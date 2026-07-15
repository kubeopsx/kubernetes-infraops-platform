package xprober

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	MetricsCollectInterval     = 15 * time.Second
	TargetFlushManagerInterval = 60 * time.Second
	resultTTL                  = 5 * time.Minute
)

type metricDefinition struct {
	help       string
	labelNames []string
	labels     func(*common.Result) []string
}

var metricDefinitions = map[string]metricDefinition{
	common.MetricsNamePingLatency: {
		help: "Average ICMP round-trip latency in milliseconds.", labelNames: []string{"source_region", "target_region"},
		labels: func(r *common.Result) []string { return []string{r.SourceRegion, r.TargetRegion} },
	},
	common.MetricsNamePingPacketDrop: {
		help: "ICMP packet loss percentage.", labelNames: []string{"source_region", "target_region"},
		labels: func(r *common.Result) []string { return []string{r.SourceRegion, r.TargetRegion} },
	},
	common.MetricsNamePingTargetSuccess: {
		help: "Whether at least one ICMP response was received.", labelNames: []string{"source_region", "target_region"},
		labels: func(r *common.Result) []string { return []string{r.SourceRegion, r.TargetRegion} },
	},
	common.MetricsNameHTTPInterfaceSuccess:             targetMetric("Whether the HTTP probe succeeded."),
	common.MetricsNameHTTPResolveDurationMilliseconds:  targetMetric("HTTP DNS resolution duration in milliseconds."),
	common.MetricsNameHTTPTLSDurationMilliseconds:      targetMetric("HTTP TLS handshake duration in milliseconds."),
	common.MetricsNameHTTPConnectDurationMilliseconds:  targetMetric("HTTP connection duration in milliseconds."),
	common.MetricsNameHTTPProcessDurationMilliseconds:  targetMetric("HTTP server processing duration in milliseconds."),
	common.MetricsNameHTTPTransferDurationMilliseconds: targetMetric("HTTP response transfer duration in milliseconds."),
	common.MetricsNameTCPConnectDurationMilliseconds:   targetMetric("TCP connection duration in milliseconds."),
	common.MetricsNameTCPConnectSuccess:                targetMetric("Whether the TCP connection succeeded."),
	common.MetricsNameDNSLookupDurationMilliseconds:    targetMetric("DNS lookup duration in milliseconds."),
	common.MetricsNameDNSLookupSuccess:                 targetMetric("Whether the DNS lookup succeeded."),
	common.MetricsNameDNSAnswerCount:                   targetMetric("Number of addresses returned by the DNS lookup."),
	common.MetricsNameTLSHandshakeDurationMilliseconds: targetMetric("TLS connection and handshake duration in milliseconds."),
	common.MetricsNameTLSConnectSuccess:                targetMetric("Whether the TLS connection succeeded."),
	common.MetricsNameTLSCertificateValid:              targetMetric("Whether the peer certificate is currently valid."),
	common.MetricsNameTLSCertificateExpirySeconds:      targetMetric("Seconds until the peer certificate expires."),
}

func targetMetric(help string) metricDefinition {
	return metricDefinition{
		help:       help,
		labelNames: []string{"source_region", "target_region", "target_address"},
		labels: func(r *common.Result) []string {
			return []string{r.SourceRegion, r.TargetRegion, r.TargetAddress}
		},
	}
}

type aggregate struct {
	sum   float64
	count int
}

type Collector struct {
	mu       sync.Mutex
	results  map[string]*common.Result
	generic  *prometheus.GaugeVec
	specific map[string]*prometheus.GaugeVec
}

func newCollector(registerer prometheus.Registerer) (*Collector, error) {
	collector := &Collector{
		results: make(map[string]*common.Result),
		generic: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "infraops_probe_value",
			Help: "Latest protocol probe value; metric_name identifies the protocol-specific sample.",
		}, []string{"metric_name", "worker", "probe_type", "target_address", "source_region", "target_region"}),
		specific: make(map[string]*prometheus.GaugeVec, len(metricDefinitions)),
	}
	if err := registerer.Register(collector.generic); err != nil {
		return nil, fmt.Errorf("register generic probe metric: %w", err)
	}
	for name, definition := range metricDefinitions {
		metric := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: definition.help}, definition.labelNames)
		if err := registerer.Register(metric); err != nil {
			return nil, fmt.Errorf("register probe metric %q: %w", name, err)
		}
		collector.specific[name] = metric
	}
	return collector, nil
}

func (c *Collector) Store(results []*common.Result) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	stored := 0
	for _, result := range results {
		if result == nil || strings.TrimSpace(result.MetricsName) == "" {
			continue
		}
		copyResult := *result
		if copyResult.TimeStamp <= 0 {
			copyResult.TimeStamp = time.Now().Unix()
		}
		c.results[resultUID(&copyResult)] = &copyResult
		stored++
	}
	return stored
}

func (c *Collector) Publish(now time.Time) {
	c.mu.Lock()
	current := make([]*common.Result, 0, len(c.results))
	for uid, result := range c.results {
		if now.Sub(time.Unix(result.TimeStamp, 0)) > resultTTL {
			delete(c.results, uid)
			continue
		}
		copyResult := *result
		current = append(current, &copyResult)
	}
	c.mu.Unlock()

	c.generic.Reset()
	for _, metric := range c.specific {
		metric.Reset()
	}
	aggregates := make(map[string]*aggregate)
	aggregateLabels := make(map[string][]string)
	for _, result := range current {
		c.generic.WithLabelValues(
			result.MetricsName,
			result.WorkerName,
			result.Type,
			result.TargetAddress,
			result.SourceRegion,
			result.TargetRegion,
		).Set(float64(result.Value))

		definition, known := metricDefinitions[result.MetricsName]
		if !known {
			continue
		}
		labels := definition.labels(result)
		key := result.MetricsName + "\xff" + strings.Join(labels, "\xff")
		if aggregates[key] == nil {
			aggregates[key] = &aggregate{}
			aggregateLabels[key] = labels
		}
		aggregates[key].sum += float64(result.Value)
		aggregates[key].count++
	}
	for key, value := range aggregates {
		metricName := strings.SplitN(key, "\xff", 2)[0]
		c.specific[metricName].WithLabelValues(aggregateLabels[key]...).Set(value.sum / float64(value.count))
	}
}

func resultUID(result *common.Result) string {
	return strings.Join([]string{
		result.WorkerName,
		result.MetricsName,
		result.SourceRegion,
		result.TargetRegion,
		result.Type,
		result.TargetAddress,
	}, "\xff")
}

var (
	defaultCollector     *Collector
	defaultCollectorOnce sync.Once
	defaultCollectorErr  error
)

func New() {
	defaultCollectorOnce.Do(func() {
		defaultCollector, defaultCollectorErr = newCollector(prometheus.DefaultRegisterer)
	})
	if defaultCollectorErr != nil {
		panic(defaultCollectorErr)
	}
}

func StoreResults(results []*common.Result) int {
	New()
	return defaultCollector.Store(results)
}

func DataProcess(ctx context.Context, logger log.Logger) error {
	New()
	ticker := time.NewTicker(MetricsCollectInterval)
	defer ticker.Stop()
	level.Info(logger).Log("msg", "run multi-protocol data process manager")
	defaultCollector.Publish(time.Now())
	for {
		select {
		case <-ticker.C:
			defaultCollector.Publish(time.Now())
		case <-ctx.Done():
			return nil
		}
	}
}
