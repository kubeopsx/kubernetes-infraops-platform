package prober

import (
	"context"
	"fmt"
	"sync"

	"github.com/gammazero/workerpool"
)

type Callback func([]Sample, error)

type Scheduler struct {
	registry *Registry
	pool     *workerpool.WorkerPool
	mu       sync.RWMutex
	closed   bool
}

func NewScheduler(maxWorkers int, registry *Registry) *Scheduler {
	if maxWorkers <= 0 {
		maxWorkers = 32
	}
	if registry == nil {
		registry = NewDefaultRegistry()
	}
	return &Scheduler{
		registry: registry,
		pool:     workerpool.New(maxWorkers),
	}
}

func (s *Scheduler) Submit(parent context.Context, target Target, callback Callback) error {
	target = target.Normalize()
	if _, ok := s.registry.Get(target.Type); !ok {
		return fmt.Errorf("unsupported probe type %q", target.Type)
	}
	if callback == nil {
		return fmt.Errorf("probe callback is nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("probe scheduler is stopped")
	}
	s.pool.Submit(func() {
		ctx, cancel := context.WithTimeout(parent, timeoutFor(parent, target.Timeout))
		defer cancel()
		samples, err := s.registry.Probe(ctx, target)
		callback(samples, err)
	})
	return nil
}

func (s *Scheduler) StopWait() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	s.pool.StopWait()
}
