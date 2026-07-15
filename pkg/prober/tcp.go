package prober

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
)

type TCPProber struct{}

func (TCPProber) Name() string { return "tcp" }

func (TCPProber) Probe(ctx context.Context, target Target) ([]Sample, error) {
	target = target.Normalize()
	start := time.Now()
	dialer := net.Dialer{Timeout: timeoutFor(ctx, target.Timeout)}
	conn, err := dialer.DialContext(ctx, "tcp", target.Address)
	duration := milliseconds(time.Since(start))
	samples := []Sample{
		NewSample(common.MetricsNameTCPConnectDurationMilliseconds, duration),
		NewSample(common.MetricsNameTCPConnectSuccess, 0),
	}
	if err != nil {
		return samples, fmt.Errorf("probe TCP target %q: %w", target.Address, err)
	}
	_ = conn.Close()
	samples[1].Value = 1
	return samples, nil
}
