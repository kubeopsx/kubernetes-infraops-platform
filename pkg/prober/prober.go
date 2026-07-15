package prober

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	DefaultInterval = 15 * time.Second
	DefaultTimeout  = 10 * time.Second
)

type Target struct {
	Type         string
	Address      string
	SourceRegion string
	TargetRegion string
	Interval     time.Duration
	Timeout      time.Duration
	Options      map[string]string
}

func (t Target) Normalize() Target {
	t.Type = strings.ToLower(strings.TrimSpace(t.Type))
	t.Address = strings.TrimSpace(t.Address)
	if t.Interval <= 0 {
		t.Interval = DefaultInterval
	}
	if t.Timeout <= 0 {
		t.Timeout = DefaultTimeout
	}
	if t.Options == nil {
		t.Options = map[string]string{}
	}
	return t
}

type Sample struct {
	MetricName string
	Value      float64
	Timestamp  time.Time
}

func NewSample(name string, value float64) Sample {
	return Sample{MetricName: name, Value: value, Timestamp: time.Now()}
}

type Prober interface {
	Name() string
	Probe(context.Context, Target) ([]Sample, error)
}

type Registry struct {
	mu      sync.RWMutex
	probers map[string]Prober
}

func NewRegistry() *Registry {
	return &Registry{probers: make(map[string]Prober)}
}

func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.MustRegister(ICMPProber{})
	r.MustRegister(HTTPProber{})
	r.MustRegister(TCPProber{})
	r.MustRegister(DNSProber{})
	r.MustRegister(TLSProber{})
	return r
}

func (r *Registry) Register(p Prober) error {
	if p == nil {
		return fmt.Errorf("register nil prober")
	}
	name := strings.ToLower(strings.TrimSpace(p.Name()))
	if name == "" {
		return fmt.Errorf("register prober with empty name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.probers[name]; exists {
		return fmt.Errorf("prober %q already registered", name)
	}
	r.probers[name] = p
	return nil
}

func (r *Registry) MustRegister(p Prober) {
	if err := r.Register(p); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(name string) (Prober, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.probers[strings.ToLower(strings.TrimSpace(name))]
	return p, ok
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.probers))
	for name := range r.probers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) Probe(ctx context.Context, target Target) ([]Sample, error) {
	target = target.Normalize()
	p, ok := r.Get(target.Type)
	if !ok {
		return nil, fmt.Errorf("unsupported probe type %q", target.Type)
	}
	return p.Probe(ctx, target)
}
