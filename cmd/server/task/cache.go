package task

import (
	"context"
	"encoding/json"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/toolkits/pkg/logger"
	"sync"
	"time"
)

var Caches *Cache

type Cache struct {
	sync.RWMutex
	M map[string][]*model.TaskMeta
}

func CacheNew() {
	Caches = &Cache{
		M: make(map[string][]*model.TaskMeta),
	}
}

func SyncTaskManager(ctx context.Context, logger log.Logger) error {

	level.Info(logger).Log("msg", "run sync task manager")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			Caches.doSyncTask()
		}
	}
}

func (tc *Cache) doSyncTask() {
	tasks, err := model.UnDoTaskMeta()
	if err != nil {
		logger.Errorf("err:%+v", err)
		return
	}
	m := make(map[string][]*model.TaskMeta)
	for _, t := range tasks {
		err := json.Unmarshal([]byte(t.HostsRaw), &t.Hosts)
		if err != nil {
			logger.Errorf("err:%+v", err)
			continue
		}
		logger.Debugf("t:%+v", t)
		if len(t.Hosts) == 0 {
			continue
		}

		for _, host := range t.Hosts {
			tasks, loaded := m[host]
			if !loaded {
				tasks = make([]*model.TaskMeta, 0)
			}
			if t.Action == "" {
				t.Action = "start"
			}

			tasks = append(tasks, t)
			m[host] = tasks
		}
	}
	tc.Lock()
	defer tc.Unlock()
	tc.M = m
	logger.Debugf("m:%+v", tc.M)
}

func (tc *Cache) GetTasksByIp(ip string) []*model.TaskMeta {
	tc.Lock()
	defer tc.Unlock()
	result, loaded := tc.M[ip]
	if !loaded {
		result = make([]*model.TaskMeta, 0)
	}
	return result
}
