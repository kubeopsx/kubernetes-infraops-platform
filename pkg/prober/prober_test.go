package prober

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
)

func TestDefaultRegistryProtocols(t *testing.T) {
	registry := NewDefaultRegistry()
	want := []string{"dns", "http", "icmp", "tcp", "tls"}
	got := registry.Names()
	if len(got) != len(want) {
		t.Fatalf("protocol count = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("protocol[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTCPProber(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = conn.Close()
		}
	}()

	samples, err := (TCPProber{}).Probe(context.Background(), Target{Address: listener.Addr().String(), Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	assertSampleValue(t, samples, common.MetricsNameTCPConnectSuccess, 1)
}

func TestHTTPProber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	samples, err := (HTTPProber{}).Probe(context.Background(), Target{Address: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	assertSampleValue(t, samples, common.MetricsNameHTTPInterfaceSuccess, 1)
}

func TestDNSProber(t *testing.T) {
	samples, err := (DNSProber{}).Probe(context.Background(), Target{Address: "localhost", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	assertSampleValue(t, samples, common.MetricsNameDNSLookupSuccess, 1)
}

func TestTLSProber(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()

	address := server.Listener.Addr().String()
	samples, err := (TLSProber{}).Probe(context.Background(), Target{
		Address: address,
		Timeout: time.Second,
		Options: map[string]string{"insecure_skip_verify": "true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertSampleValue(t, samples, common.MetricsNameTLSConnectSuccess, 1)
}

type concurrencyProber struct {
	current atomic.Int32
	maximum atomic.Int32
	release <-chan struct{}
	wg      *sync.WaitGroup
}

func (p *concurrencyProber) Name() string { return "concurrency" }

func (p *concurrencyProber) Probe(context.Context, Target) ([]Sample, error) {
	current := p.current.Add(1)
	for {
		maximum := p.maximum.Load()
		if current <= maximum || p.maximum.CompareAndSwap(maximum, current) {
			break
		}
	}
	<-p.release
	p.current.Add(-1)
	p.wg.Done()
	return []Sample{NewSample("test_value", 1)}, nil
}

func TestSchedulerBoundsConcurrency(t *testing.T) {
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	fake := &concurrencyProber{release: release, wg: &wg}
	registry := NewRegistry()
	registry.MustRegister(fake)
	scheduler := NewScheduler(1, registry)
	defer scheduler.StopWait()

	callback := func([]Sample, error) {}
	if err := scheduler.Submit(context.Background(), Target{Type: fake.Name()}, callback); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Submit(context.Background(), Target{Type: fake.Name()}, callback); err != nil {
		t.Fatal(err)
	}
	close(release)
	wg.Wait()
	if got := fake.maximum.Load(); got != 1 {
		t.Fatalf("maximum concurrency = %d, want 1", got)
	}
}

func assertSampleValue(t *testing.T, samples []Sample, name string, want float64) {
	t.Helper()
	for _, sample := range samples {
		if sample.MetricName == name {
			if sample.Value != want {
				t.Fatalf("sample %s = %v, want %v", name, sample.Value, want)
			}
			return
		}
	}
	t.Fatalf("sample %s not found in %+v", name, samples)
}
