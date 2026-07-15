package prober

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
)

type TLSProber struct{}

func (TLSProber) Name() string { return "tls" }

func (TLSProber) Probe(ctx context.Context, target Target) ([]Sample, error) {
	target = target.Normalize()
	address := addressWithDefaultPort(target.Address, "443")
	serverName := target.Options["server_name"]
	if serverName == "" {
		serverName = targetHost(address)
	}
	config := &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: boolOption(target.Options, "insecure_skip_verify", false),
		MinVersion:         tls.VersionTLS12,
	}
	dialer := tls.Dialer{
		NetDialer: &net.Dialer{Timeout: timeoutFor(ctx, target.Timeout)},
		Config:    config,
	}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", address)
	duration := milliseconds(time.Since(start))
	samples := []Sample{
		NewSample(common.MetricsNameTLSHandshakeDurationMilliseconds, duration),
		NewSample(common.MetricsNameTLSConnectSuccess, 0),
		NewSample(common.MetricsNameTLSCertificateValid, 0),
		NewSample(common.MetricsNameTLSCertificateExpirySeconds, 0),
	}
	if err != nil {
		return samples, fmt.Errorf("probe TLS target %q: %w", address, err)
	}
	defer conn.Close()
	samples[1].Value = 1
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return samples, fmt.Errorf("probe TLS target %q returned unexpected connection type %T", address, conn)
	}
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return samples, fmt.Errorf("probe TLS target %q returned no peer certificate", address)
	}
	leaf := state.PeerCertificates[0]
	now := time.Now()
	if now.After(leaf.NotBefore) && now.Before(leaf.NotAfter) {
		samples[2].Value = 1
	}
	samples[3].Value = leaf.NotAfter.Sub(now).Seconds()
	return samples, nil
}
