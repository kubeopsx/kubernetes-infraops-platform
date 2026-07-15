package prober

import (
	"context"
	"fmt"
	"time"

	"github.com/go-ping/ping"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
)

type ICMPProber struct{}

func (ICMPProber) Name() string { return "icmp" }

func (ICMPProber) Probe(ctx context.Context, target Target) ([]Sample, error) {
	target = target.Normalize()
	host := targetHost(target.Address)
	pinger, err := ping.NewPinger(host)
	if err != nil {
		return icmpFailure(), fmt.Errorf("create ICMP pinger for %q: %w", host, err)
	}
	pinger.Count = intOption(target.Options, "count", 3)
	pinger.Interval = 200 * time.Millisecond
	pinger.Timeout = timeoutFor(ctx, target.Timeout)
	pinger.SetPrivileged(boolOption(target.Options, "privileged", false))

	done := make(chan error, 1)
	go func() { done <- pinger.Run() }()
	select {
	case <-ctx.Done():
		pinger.Stop()
		return icmpFailure(), ctx.Err()
	case err = <-done:
		if err != nil {
			return icmpFailure(), fmt.Errorf("probe ICMP target %q: %w", host, err)
		}
	}

	stats := pinger.Statistics()
	success := 0.0
	if stats.PacketsRecv > 0 {
		success = 1
	}
	return []Sample{
		NewSample(common.MetricsNamePingTargetSuccess, success),
		NewSample(common.MetricsNamePingPacketDrop, stats.PacketLoss),
		NewSample(common.MetricsNamePingLatency, milliseconds(stats.AvgRtt)),
	}, nil
}

func icmpFailure() []Sample {
	return []Sample{
		NewSample(common.MetricsNamePingTargetSuccess, 0),
		NewSample(common.MetricsNamePingPacketDrop, 100),
	}
}
