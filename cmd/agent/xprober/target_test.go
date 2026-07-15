package xprober

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/prober"
)

func TestResultBufferDrainAndRestore(t *testing.T) {
	buffer := newResultBuffer()
	older := []*common.Result{{MetricsName: "older", Value: 1}}
	buffer.Store("target", older)
	drained := buffer.Drain()
	if len(drained) != 1 {
		t.Fatalf("drained entries = %d, want 1", len(drained))
	}
	newer := []*common.Result{{MetricsName: "newer", Value: 2}}
	buffer.Store("target", newer)
	buffer.Restore(drained)
	final := buffer.Drain()
	if got := final["target"][0].MetricsName; got != "newer" {
		t.Fatalf("restored value replaced newer result: %q", got)
	}
}

type adapterTestProber struct{}

func (adapterTestProber) Name() string { return "adapter" }

func (adapterTestProber) Probe(context.Context, prober.Target) ([]prober.Sample, error) {
	return []prober.Sample{prober.NewSample("adapter_value", 42)}, nil
}

func TestLocalTargetConvertsSamplesToRPCResults(t *testing.T) {
	registry := prober.NewRegistry()
	registry.MustRegister(adapterTestProber{})
	scheduler := prober.NewScheduler(1, registry)
	defer scheduler.StopWait()
	Results = newResultBuffer()
	LocalIp = "agent-1"
	target := prober.Target{
		Type: "adapter", Address: "target:1234", SourceRegion: "source", TargetRegion: "target", Timeout: time.Second,
	}
	local := newLocalTarget(context.Background(), log.NewNopLogger(), scheduler, target)
	local.schedule()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		drained := Results.Drain()
		if len(drained) != 0 {
			for _, results := range drained {
				if len(results) != 1 {
					t.Fatalf("result count = %d, want 1", len(results))
				}
				result := results[0]
				if result.WorkerName != "agent-1" || result.MetricsName != "adapter_value" || result.Value != 42 {
					t.Fatalf("unexpected converted result: %+v", result)
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("probe result was not produced")
}
