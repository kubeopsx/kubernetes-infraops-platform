package xprober

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/config/server"
)

const agentRegistrationTTL = 2 * time.Minute

type agentRegistration struct {
	region   string
	lastSeen time.Time
}

var targetCatalog = struct {
	sync.RWMutex
	configured []*common.Targets
	agents     map[string]agentRegistration
}{agents: make(map[string]agentRegistration)}

type TargetFlushManager struct {
	Logger     log.Logger
	ConfigFile string
}

func NewTargetFlushManager(logger log.Logger, configFile string) *TargetFlushManager {
	return &TargetFlushManager{Logger: logger, ConfigFile: configFile}
}

func (t *TargetFlushManager) Run(ctx context.Context) error {
	ticker := time.NewTicker(TargetFlushManagerInterval)
	defer ticker.Stop()
	level.Info(t.Logger).Log("msg", "run target flush manager")
	if err := t.refresh(); err != nil {
		level.Error(t.Logger).Log("msg", "initial probe target refresh failed", "err", err)
	}
	for {
		select {
		case <-ticker.C:
			if err := t.refresh(); err != nil {
				level.Error(t.Logger).Log("msg", "probe target refresh failed", "err", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (t *TargetFlushManager) refresh() error {
	config, err := server.LoadFile(t.ConfigFile)
	if err != nil {
		return fmt.Errorf("load server config %q: %w", t.ConfigFile, err)
	}
	configured := make([]*common.Targets, 0, len(config.Probe))
	for _, probe := range config.Probe {
		if probe == nil || strings.TrimSpace(probe.Type) == "" || len(probe.Target) == 0 {
			continue
		}
		configured = append(configured, &common.Targets{
			Type:            strings.ToLower(strings.TrimSpace(probe.Type)),
			Region:          probe.Region,
			Target:          deduplicateStrings(probe.Target),
			IntervalSeconds: probe.IntervalSeconds,
			TimeoutSeconds:  probe.TimeoutSeconds,
			Options:         cloneStringMap(probe.Options),
		})
	}
	targetCatalog.Lock()
	targetCatalog.configured = configured
	removeExpiredAgentsLocked(time.Now())
	targetCatalog.Unlock()
	level.Info(t.Logger).Log("msg", "probe target catalog refreshed", "groups", len(configured))
	return nil
}

func RecordAgent(ip, region string) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	targetCatalog.Lock()
	targetCatalog.agents[ip] = agentRegistration{region: region, lastSeen: time.Now()}
	removeExpiredAgentsLocked(time.Now())
	targetCatalog.Unlock()
}

func GetTargetsByRegion(sourceRegion string) []*common.Targets {
	targetCatalog.Lock()
	removeExpiredAgentsLocked(time.Now())
	configured := cloneTargetGroups(targetCatalog.configured)
	agents := make(map[string]agentRegistration, len(targetCatalog.agents))
	for ip, registration := range targetCatalog.agents {
		agents[ip] = registration
	}
	targetCatalog.Unlock()

	result := make([]*common.Targets, 0, len(configured)+len(agents))
	for _, group := range configured {
		if group.Type == "icmp" && group.Region == sourceRegion {
			continue
		}
		result = append(result, group)
	}
	regionAgents := make(map[string][]string)
	for ip, registration := range agents {
		if registration.region == sourceRegion {
			continue
		}
		regionAgents[registration.region] = append(regionAgents[registration.region], ip)
	}
	regions := make([]string, 0, len(regionAgents))
	for region := range regionAgents {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	for _, region := range regions {
		result = append(result, &common.Targets{
			Type:   "icmp",
			Region: region,
			Target: deduplicateStrings(regionAgents[region]),
		})
	}
	return mergeEquivalentTargetGroups(result)
}

func removeExpiredAgentsLocked(now time.Time) {
	for ip, registration := range targetCatalog.agents {
		if now.Sub(registration.lastSeen) > agentRegistrationTTL {
			delete(targetCatalog.agents, ip)
		}
	}
}

func mergeEquivalentTargetGroups(groups []*common.Targets) []*common.Targets {
	merged := make(map[string]*common.Targets)
	order := make([]string, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d|%d|%s", group.Type, group.Region, group.IntervalSeconds, group.TimeoutSeconds, optionsKey(group.Options))
		current, exists := merged[key]
		if !exists {
			current = &common.Targets{
				Type:            group.Type,
				Region:          group.Region,
				IntervalSeconds: group.IntervalSeconds,
				TimeoutSeconds:  group.TimeoutSeconds,
				Options:         cloneStringMap(group.Options),
			}
			merged[key] = current
			order = append(order, key)
		}
		current.Target = append(current.Target, group.Target...)
	}
	result := make([]*common.Targets, 0, len(order))
	for _, key := range order {
		group := merged[key]
		group.Target = deduplicateStrings(group.Target)
		result = append(result, group)
	}
	return result
}

func optionsKey(options map[string]string) string {
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+options[key])
	}
	return strings.Join(parts, "\xff")
}

func cloneTargetGroups(groups []*common.Targets) []*common.Targets {
	result := make([]*common.Targets, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		result = append(result, &common.Targets{
			Type:            group.Type,
			Region:          group.Region,
			Target:          append([]string(nil), group.Target...),
			IntervalSeconds: group.IntervalSeconds,
			TimeoutSeconds:  group.TimeoutSeconds,
			Options:         cloneStringMap(group.Options),
		})
	}
	return result
}

func cloneStringMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func deduplicateStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
