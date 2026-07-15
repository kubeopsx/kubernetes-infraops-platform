package xprober

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
)

func TestTargetCatalogDeduplicatesAndPropagatesOptions(t *testing.T) {
	resetTargetCatalog()
	t.Cleanup(resetTargetCatalog)
	path := filepath.Join(t.TempDir(), "server.yaml")
	config := []byte(`probe:
  - type: http
    region: external
    target: ["https://example.com", "https://example.com"]
    interval_seconds: 20
    timeout_seconds: 4
    options:
      method: HEAD
  - type: icmp
    region: source
    target: ["127.0.0.1"]
`)
	if err := os.WriteFile(path, config, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewTargetFlushManager(log.NewNopLogger(), path)
	if err := manager.refresh(); err != nil {
		t.Fatal(err)
	}
	groups := GetTargetsByRegion("source")
	if len(groups) != 1 {
		t.Fatalf("target groups = %d, want 1: %+v", len(groups), groups)
	}
	group := groups[0]
	if group.Type != "http" || len(group.Target) != 1 || group.IntervalSeconds != 20 || group.TimeoutSeconds != 4 {
		t.Fatalf("unexpected target group: %+v", group)
	}
	if group.Options["method"] != "HEAD" {
		t.Fatalf("method option = %q", group.Options["method"])
	}
}

func resetTargetCatalog() {
	targetCatalog.Lock()
	targetCatalog.configured = nil
	targetCatalog.agents = make(map[string]agentRegistration)
	targetCatalog.Unlock()
}
