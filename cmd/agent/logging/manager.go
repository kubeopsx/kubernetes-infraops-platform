package logging

import (
	"context"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/consumer"
	"github.com/toolkits/pkg/logger"
	"sync"
)

type Manager struct {
	TargetMtx    sync.Mutex
	activeTarget map[string]*Logging
	c            chan *consumer.AnalystPoint
}

func NewManager(c chan *consumer.AnalystPoint) *Manager {
	m := make(map[string]*Logging)
	return &Manager{
		activeTarget: m,
		c:            c,
	}
}

func (l *Manager) SyncManager(ctx context.Context, syncChan chan []*Logging) error {
	for {
		select {
		case <-ctx.Done():
			l.SyncStop()
			return nil
		case jobs := <-syncChan:
			l.Sync(jobs)
		}

	}
}

func (l *Manager) Sync(jobs []*Logging) {
	logger.Infof("run logging manager [num:%d][res:%+v]", len(jobs), jobs)
	NewTargets := make(map[string]*Logging)
	AllTargets := make(map[string]*Logging)

	l.TargetMtx.Lock()
	for _, t := range jobs {
		hash := t.hash()
		AllTargets[hash] = t
		if _, v := l.activeTarget[hash]; !v {
			NewTargets[hash] = t
			l.activeTarget[hash] = t
		}
	}
	// 停止旧的
	for hash, t := range l.activeTarget {
		if _, v := AllTargets[hash]; !v {
			logger.Infof("[stop:%+v] [s:%+v]", t, t.S)
			t.stop()
			delete(l.activeTarget, hash)
		}
	}
	l.TargetMtx.Unlock()

	for _, t := range NewTargets {
		t := t
		t.start(l.c)
	}
}

func (l *Manager) SyncStop() {
	l.TargetMtx.Lock()
	defer l.TargetMtx.Unlock()
	for _, v := range l.activeTarget {
		v.stop()
	}
}
