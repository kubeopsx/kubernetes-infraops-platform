package xprober

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/rpc"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/prober"
	"github.com/prometheus/client_golang/prometheus"
)

const DefaultMaxProbeWorkers = 32

var (
	Registry  *prober.Registry
	Scheduler *prober.Scheduler
	LTM       *LocalTargetManager
	Results   = newResultBuffer()

	registerMetricsOnce sync.Once
	LocalProbeValue     = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "infraops_agent_probe_value",
		Help: "Latest protocol probe sample produced by this infraops agent.",
	}, []string{"metric_name", "probe_type", "target_address", "source_region", "target_region"})
)

type resultBuffer struct {
	mu   sync.Mutex
	data map[string][]*common.Result
}

func newResultBuffer() *resultBuffer {
	return &resultBuffer{data: make(map[string][]*common.Result)}
}

func (b *resultBuffer) Store(key string, values []*common.Result) {
	b.mu.Lock()
	b.data[key] = values
	b.mu.Unlock()
}

func (b *resultBuffer) Drain() map[string][]*common.Result {
	b.mu.Lock()
	defer b.mu.Unlock()
	drained := b.data
	b.data = make(map[string][]*common.Result)
	return drained
}

func (b *resultBuffer) Restore(values map[string][]*common.Result) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for key, result := range values {
		if _, newer := b.data[key]; !newer {
			b.data[key] = result
		}
	}
}

type LocalTargetManager struct {
	logger    log.Logger
	mu        sync.RWMutex
	targets   map[string]*LocalTarget
	scheduler *prober.Scheduler
	registry  *prober.Registry
	ctx       context.Context
	cancel    context.CancelFunc
	stopOnce  sync.Once
}

func NewLocalTargetManager(logger log.Logger, registry *prober.Registry, scheduler *prober.Scheduler) *LocalTargetManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &LocalTargetManager{
		logger:    logger,
		targets:   make(map[string]*LocalTarget),
		scheduler: scheduler,
		registry:  registry,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (ltm *LocalTargetManager) refresh(targets *common.TargetGetResponse) {
	desired := make(map[string]prober.Target)
	if targets != nil {
		for _, group := range targets.Targets {
			if group == nil {
				continue
			}
			probeType := strings.ToLower(strings.TrimSpace(group.Type))
			if _, exists := ltm.registry.Get(probeType); !exists {
				level.Warn(ltm.logger).Log("msg", "ignore unsupported probe type", "type", probeType)
				continue
			}
			interval := time.Duration(group.IntervalSeconds) * time.Second
			timeout := time.Duration(group.TimeoutSeconds) * time.Second
			for _, address := range group.Target {
				target := (prober.Target{
					Type:         probeType,
					Address:      address,
					SourceRegion: LocalRegion,
					TargetRegion: group.Region,
					Interval:     interval,
					Timeout:      timeout,
					Options:      cloneOptions(group.Options),
				}).Normalize()
				if target.Address == "" {
					continue
				}
				desired[targetUID(target)] = target
			}
		}
	}

	var start []*LocalTarget
	var stop []*LocalTarget
	ltm.mu.Lock()
	for uid, current := range ltm.targets {
		target, found := desired[uid]
		if !found || !sameTarget(current.target, target) {
			stop = append(stop, current)
			delete(ltm.targets, uid)
		}
	}
	for uid, target := range desired {
		if _, found := ltm.targets[uid]; found {
			continue
		}
		worker := newLocalTarget(ltm.ctx, ltm.logger, ltm.scheduler, target)
		ltm.targets[uid] = worker
		start = append(start, worker)
	}
	ltm.mu.Unlock()

	for _, worker := range stop {
		worker.Stop()
	}
	for _, worker := range start {
		worker.Start()
	}
	level.Info(ltm.logger).Log("msg", "probe targets reconciled", "desired", len(desired), "started", len(start), "stopped", len(stop))
}

func (ltm *LocalTargetManager) Stop() {
	ltm.stopOnce.Do(func() {
		ltm.cancel()
		ltm.mu.Lock()
		targets := make([]*LocalTarget, 0, len(ltm.targets))
		for _, target := range ltm.targets {
			targets = append(targets, target)
		}
		ltm.targets = make(map[string]*LocalTarget)
		ltm.mu.Unlock()
		for _, target := range targets {
			target.Stop()
		}
		ltm.scheduler.StopWait()
	})
}

type LocalTarget struct {
	logger    log.Logger
	target    prober.Target
	scheduler *prober.Scheduler
	ctx       context.Context
	cancel    context.CancelFunc
	stopOnce  sync.Once
	running   atomic.Bool
}

func newLocalTarget(parent context.Context, logger log.Logger, scheduler *prober.Scheduler, target prober.Target) *LocalTarget {
	ctx, cancel := context.WithCancel(parent)
	return &LocalTarget{logger: logger, target: target, scheduler: scheduler, ctx: ctx, cancel: cancel}
}

func (lt *LocalTarget) Start() {
	go func() {
		lt.schedule()
		ticker := time.NewTicker(lt.target.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-lt.ctx.Done():
				return
			case <-ticker.C:
				lt.schedule()
			}
		}
	}()
}

func (lt *LocalTarget) schedule() {
	if !lt.running.CompareAndSwap(false, true) {
		level.Warn(lt.logger).Log("msg", "skip overlapping probe", "target", lt.target.Address, "type", lt.target.Type)
		return
	}
	err := lt.scheduler.Submit(lt.ctx, lt.target, func(samples []prober.Sample, err error) {
		defer lt.running.Store(false)
		if lt.ctx.Err() != nil {
			return
		}
		if err != nil {
			level.Warn(lt.logger).Log("msg", "protocol probe failed", "target", lt.target.Address, "type", lt.target.Type, "err", err)
		}
		if len(samples) == 0 {
			return
		}
		results := make([]*common.Result, 0, len(samples))
		for _, sample := range samples {
			timestamp := sample.Timestamp
			if timestamp.IsZero() {
				timestamp = time.Now()
			}
			result := &common.Result{
				WorkerName:    LocalIp,
				MetricsName:   sample.MetricName,
				TargetAddress: lt.target.Address,
				SourceRegion:  lt.target.SourceRegion,
				TargetRegion:  lt.target.TargetRegion,
				Type:          lt.target.Type,
				TimeStamp:     timestamp.Unix(),
				Value:         float32(sample.Value),
			}
			results = append(results, result)
			LocalProbeValue.WithLabelValues(sample.MetricName, lt.target.Type, lt.target.Address, lt.target.SourceRegion, lt.target.TargetRegion).Set(sample.Value)
		}
		Results.Store(targetUID(lt.target), results)
	})
	if err != nil {
		lt.running.Store(false)
		level.Error(lt.logger).Log("msg", "submit protocol probe failed", "target", lt.target.Address, "type", lt.target.Type, "err", err)
	}
}

func (lt *LocalTarget) Stop() {
	lt.stopOnce.Do(lt.cancel)
}

func New(logger log.Logger, region string) {
	Registry = prober.NewDefaultRegistry()
	Scheduler = prober.NewScheduler(DefaultMaxProbeWorkers, Registry)
	if region != "" {
		LocalRegion = region
	} else if !GetLocalRegionByEc2(logger) {
		LocalRegion = "unknown"
	}
	LocalIp = GetLocalIp()
	if LocalIp == "" {
		LocalIp = common.GetHostName()
	}
	registerMetricsOnce.Do(func() { prometheus.MustRegister(LocalProbeValue) })
	LTM = NewLocalTargetManager(logger, Registry, Scheduler)
	level.Info(logger).Log("msg", "multi-protocol probe engine initialized", "protocols", strings.Join(Registry.Names(), ","), "workers", DefaultMaxProbeWorkers)
}

func TickerPushResults(client *rpc.Client, ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pushProbeResults(client)
		}
	}
}

func pushProbeResults(client *rpc.Client) {
	drained := Results.Drain()
	results := make([]*common.Result, 0)
	for _, values := range drained {
		results = append(results, values...)
	}
	if len(results) == 0 {
		return
	}
	response := client.PushResults(common.ResultPushRequest{Results: results})
	if response == nil || int(response.SuccessNum) != len(results) {
		Results.Restore(drained)
	}
}

func TickerGetTargets(client *rpc.Client, ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	request := common.TargetGetRequest{LocalRegion: LocalRegion, LocalIp: LocalIp}
	syncTargets := func() {
		response := client.ProberTargetSync(request)
		if response != nil {
			LTM.refresh(response)
		}
	}
	syncTargets()
	for {
		select {
		case <-ctx.Done():
			LTM.Stop()
			return nil
		case <-ticker.C:
			syncTargets()
		}
	}
}

func targetUID(target prober.Target) string {
	return fmt.Sprintf("%s|%s|%s", target.Type, target.TargetRegion, target.Address)
}

func sameTarget(left, right prober.Target) bool {
	return left.Type == right.Type &&
		left.Address == right.Address &&
		left.SourceRegion == right.SourceRegion &&
		left.TargetRegion == right.TargetRegion &&
		left.Interval == right.Interval &&
		left.Timeout == right.Timeout &&
		reflect.DeepEqual(left.Options, right.Options)
}

func cloneOptions(options map[string]string) map[string]string {
	clone := make(map[string]string, len(options))
	for key, value := range options {
		clone[key] = value
	}
	return clone
}
