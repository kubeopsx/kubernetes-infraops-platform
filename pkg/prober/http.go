package prober

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
)

type HTTPProber struct{}

func (HTTPProber) Name() string { return "http" }

type httpTimings struct {
	mu              sync.Mutex
	dnsStart        time.Time
	dnsDone         time.Time
	connectStart    time.Time
	connectDone     time.Time
	tlsStart        time.Time
	tlsDone         time.Time
	gotConn         time.Time
	firstByte       time.Time
	responseBodyEnd time.Time
}

func (HTTPProber) Probe(ctx context.Context, target Target) ([]Sample, error) {
	target = target.Normalize()
	address := target.Address
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}

	timings := &httpTimings{}
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			timings.mu.Lock()
			timings.dnsStart = time.Now()
			timings.mu.Unlock()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			timings.mu.Lock()
			timings.dnsDone = time.Now()
			timings.mu.Unlock()
		},
		ConnectStart: func(_, _ string) {
			timings.mu.Lock()
			timings.connectStart = time.Now()
			timings.mu.Unlock()
		},
		ConnectDone: func(_, _ string, _ error) {
			timings.mu.Lock()
			timings.connectDone = time.Now()
			timings.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			timings.mu.Lock()
			timings.tlsStart = time.Now()
			timings.mu.Unlock()
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			timings.mu.Lock()
			timings.tlsDone = time.Now()
			timings.mu.Unlock()
		},
		GotConn: func(httptrace.GotConnInfo) {
			timings.mu.Lock()
			timings.gotConn = time.Now()
			timings.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			timings.mu.Lock()
			timings.firstByte = time.Now()
			timings.mu.Unlock()
		},
	}

	requestCtx, cancel := context.WithTimeout(ctx, timeoutFor(ctx, target.Timeout))
	defer cancel()
	method := strings.ToUpper(target.Options["method"])
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(requestCtx, trace), method, address, nil)
	if err != nil {
		return httpFailure(), fmt.Errorf("create HTTP request for %q: %w", address, err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: boolOption(target.Options, "insecure_skip_verify", false),
		MinVersion:         tls.VersionTLS12,
	}
	client := http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return httpFailureWithTimings(timings), fmt.Errorf("probe HTTP target %q: %w", address, err)
	}
	_, readErr := io.Copy(io.Discard, resp.Body)
	closeErr := resp.Body.Close()
	timings.mu.Lock()
	timings.responseBodyEnd = time.Now()
	timings.mu.Unlock()
	if readErr != nil {
		return httpFailureWithTimings(timings), fmt.Errorf("read HTTP response from %q: %w", address, readErr)
	}
	if closeErr != nil {
		return httpFailureWithTimings(timings), fmt.Errorf("close HTTP response from %q: %w", address, closeErr)
	}

	samples := httpSamples(timings)
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		samples[0].Value = 1
		return samples, nil
	}
	return samples, fmt.Errorf("probe HTTP target %q returned status %d", address, resp.StatusCode)
}

func httpFailure() []Sample {
	return []Sample{NewSample(common.MetricsNameHTTPInterfaceSuccess, 0)}
}

func httpFailureWithTimings(t *httpTimings) []Sample {
	samples := httpSamples(t)
	samples[0].Value = 0
	return samples
}

func httpSamples(t *httpTimings) []Sample {
	t.mu.Lock()
	defer t.mu.Unlock()
	return []Sample{
		NewSample(common.MetricsNameHTTPInterfaceSuccess, 0),
		NewSample(common.MetricsNameHTTPResolveDurationMilliseconds, milliseconds(t.dnsDone.Sub(t.dnsStart))),
		NewSample(common.MetricsNameHTTPConnectDurationMilliseconds, milliseconds(t.connectDone.Sub(t.connectStart))),
		NewSample(common.MetricsNameHTTPTLSDurationMilliseconds, milliseconds(t.tlsDone.Sub(t.tlsStart))),
		NewSample(common.MetricsNameHTTPProcessDurationMilliseconds, milliseconds(t.firstByte.Sub(t.gotConn))),
		NewSample(common.MetricsNameHTTPTransferDurationMilliseconds, milliseconds(t.responseBodyEnd.Sub(t.firstByte))),
	}
}
