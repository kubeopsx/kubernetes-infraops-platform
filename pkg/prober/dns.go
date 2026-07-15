package prober

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
)

type DNSProber struct{}

func (DNSProber) Name() string { return "dns" }

func (DNSProber) Probe(ctx context.Context, target Target) ([]Sample, error) {
	target = target.Normalize()
	resolver := net.DefaultResolver
	if server := target.Options["server"]; server != "" {
		server = addressWithDefaultPort(server, "53")
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: timeoutFor(ctx, target.Timeout)}).DialContext(ctx, network, server)
			},
		}
	}

	start := time.Now()
	answers, err := resolver.LookupHost(ctx, targetHost(target.Address))
	duration := milliseconds(time.Since(start))
	samples := []Sample{
		NewSample(common.MetricsNameDNSLookupDurationMilliseconds, duration),
		NewSample(common.MetricsNameDNSLookupSuccess, 0),
		NewSample(common.MetricsNameDNSAnswerCount, float64(len(answers))),
	}
	if err != nil {
		return samples, fmt.Errorf("probe DNS target %q: %w", target.Address, err)
	}
	samples[1].Value = 1
	return samples, nil
}
