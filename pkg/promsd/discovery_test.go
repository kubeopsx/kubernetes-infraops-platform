package promsd

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
)

func TestBuildUsesServiceTreeAndPrometheusTags(t *testing.T) {
	hosts := []model.ResourceHost{{
		Id:               7,
		Uid:              "host-7",
		Name:             "node-a",
		PrivateIps:       json.RawMessage(`["10.0.0.2","10.0.0.1","10.0.0.1"]`),
		PublicIps:        json.RawMessage(`["203.0.113.1"]`),
		Tags:             json.RawMessage(`{"cluster":"prod-a","prometheus.io/port":"9200","prometheus.io/path":"/custom-metrics","prometheus.io/job":"node-custom"}`),
		CloudProvider:    "example-cloud",
		Region:           "us-east",
		InstanceType:     "c4",
		AvailabilityZone: "us-east-1a",
		StreeGroup:       "platform",
		StreeProduct:     "monitoring",
		StreeApp:         "prometheus",
	}}

	groups, warnings := Build(hosts, Options{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %#v", groups)
	}
	wantTargets := []string{"10.0.0.1:9200", "10.0.0.2:9200"}
	if !reflect.DeepEqual(groups[0].Targets, wantTargets) {
		t.Fatalf("targets = %#v, want %#v", groups[0].Targets, wantTargets)
	}
	labels := groups[0].Labels
	for key, want := range map[string]string{
		"job":              "node-custom",
		"service_tree":     "platform.monitoring.prometheus",
		"stree_group":      "platform",
		"stree_product":    "monitoring",
		"stree_app":        "prometheus",
		"resource_name":    "node-a",
		"tag_cluster":      "prod-a",
		"__metrics_path__": "/custom-metrics",
		"__scheme__":       "http",
	} {
		if got := labels[key]; got != want {
			t.Errorf("labels[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestBuildSupportsPublicAndIPv6Targets(t *testing.T) {
	hosts := []model.ResourceHost{{
		Id:        1,
		PublicIps: json.RawMessage(`["2001:db8::1","metrics.example.test:9300"]`),
		Tags:      json.RawMessage(`{}`),
	}}
	groups, warnings := Build(hosts, Options{Network: "public", Port: 9200})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	want := []string{"[2001:db8::1]:9200", "metrics.example.test:9300"}
	if !reflect.DeepEqual(groups[0].Targets, want) {
		t.Fatalf("targets = %#v, want %#v", groups[0].Targets, want)
	}
}

func TestBuildSkipsDisabledAndInvalidTargets(t *testing.T) {
	hosts := []model.ResourceHost{
		{Id: 1, PrivateIps: json.RawMessage(`["10.0.0.1"]`), Tags: json.RawMessage(`{"prometheus.io/scrape":"false"}`)},
		{Id: 2, PrivateIps: json.RawMessage(`["10.0.0.2"]`), Tags: json.RawMessage(`{"prometheus.io/port":"bad"}`)},
		{Id: 3, PrivateIps: json.RawMessage(`["https://bad.example"]`)},
	}
	groups, warnings := Build(hosts, Options{})
	if len(groups) != 0 {
		t.Fatalf("groups = %#v", groups)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v", warnings)
	}
}
