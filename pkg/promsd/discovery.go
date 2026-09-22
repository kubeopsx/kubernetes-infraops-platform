package promsd

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
)

const (
	DefaultJob     = "infraops-service-tree"
	DefaultNetwork = "private"
	DefaultPort    = 9100
)

var invalidLabelCharacter = regexp.MustCompile(`[^a-zA-Z0-9_]`)

type Options struct {
	Job     string
	Network string
	Port    int
}

type TargetGroup struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

func Build(hosts []model.ResourceHost, options Options) ([]TargetGroup, []error) {
	options = withDefaults(options)
	groups := make([]TargetGroup, 0, len(hosts))
	var warnings []error

	for _, host := range hosts {
		tags, err := decodeTags(host.Tags)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("resource_host id=%d tags: %w", host.Id, err))
		}
		if !scrapeEnabled(tags) {
			continue
		}

		port, err := targetPort(tags, options.Port)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("resource_host id=%d: %w", host.Id, err))
			continue
		}
		addresses, errs := targetAddresses(host, options.Network, port)
		for _, err := range errs {
			warnings = append(warnings, fmt.Errorf("resource_host id=%d: %w", host.Id, err))
		}
		if len(addresses) == 0 {
			continue
		}

		labels := hostLabels(host, tags, options.Job)
		groups = append(groups, TargetGroup{Targets: addresses, Labels: labels})
	}

	sort.Slice(groups, func(i, j int) bool {
		left := groups[i].Labels["service_tree"] + "\x00" + groups[i].Labels["resource_name"]
		right := groups[j].Labels["service_tree"] + "\x00" + groups[j].Labels["resource_name"]
		if left == right {
			return strings.Join(groups[i].Targets, ",") < strings.Join(groups[j].Targets, ",")
		}
		return left < right
	})
	return groups, warnings
}

func withDefaults(options Options) Options {
	if options.Job == "" {
		options.Job = DefaultJob
	}
	if options.Network == "" {
		options.Network = DefaultNetwork
	}
	if options.Port == 0 {
		options.Port = DefaultPort
	}
	return options
}

func decodeTags(raw json.RawMessage) (map[string]string, error) {
	tags := make(map[string]string)
	if len(raw) == 0 || string(raw) == "null" {
		return tags, nil
	}
	if err := json.Unmarshal(raw, &tags); err != nil {
		return tags, err
	}
	return tags, nil
}

func scrapeEnabled(tags map[string]string) bool {
	value := firstTag(tags, "prometheus.io/scrape", "prometheus_scrape")
	if value == "" {
		return true
	}
	enabled, err := strconv.ParseBool(value)
	return err == nil && enabled
}

func targetPort(tags map[string]string, fallback int) (int, error) {
	value := firstTag(tags, "prometheus.io/port", "prometheus_port")
	if value == "" {
		if fallback < 1 || fallback > 65535 {
			return 0, fmt.Errorf("default port %d is outside 1..65535", fallback)
		}
		return fallback, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("prometheus port %q is invalid", value)
	}
	return port, nil
}

func targetAddresses(host model.ResourceHost, network string, port int) ([]string, []error) {
	var rawSets []json.RawMessage
	switch network {
	case "private":
		rawSets = append(rawSets, host.PrivateIps)
	case "public":
		rawSets = append(rawSets, host.PublicIps)
	case "all":
		rawSets = append(rawSets, host.PrivateIps, host.PublicIps)
	default:
		return nil, []error{fmt.Errorf("network %q is invalid", network)}
	}

	seen := make(map[string]struct{})
	var addresses []string
	var errs []error
	for _, raw := range rawSets {
		values, err := decodeAddresses(raw)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, value := range values {
			address, err := addPort(value, port)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if _, ok := seen[address]; ok {
				continue
			}
			seen[address] = struct{}{}
			addresses = append(addresses, address)
		}
	}
	sort.Strings(addresses)
	return addresses, errs
}

func decodeAddresses(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err == nil {
		return values, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("addresses must be a JSON string or string array")
	}
	return []string{value}, nil
}

func addPort(value string, port int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("empty target address")
	}
	if strings.ContainsAny(value, "/ \t\r\n") || strings.Contains(value, "://") {
		return "", fmt.Errorf("target address %q is invalid", value)
	}
	if host, existingPort, err := net.SplitHostPort(value); err == nil {
		if host == "" || existingPort == "" {
			return "", fmt.Errorf("target address %q is invalid", value)
		}
		return net.JoinHostPort(host, existingPort), nil
	}
	if ip := net.ParseIP(value); ip != nil {
		return net.JoinHostPort(value, strconv.Itoa(port)), nil
	}
	if strings.Contains(value, ":") {
		return "", fmt.Errorf("target address %q is invalid", value)
	}
	return net.JoinHostPort(value, strconv.Itoa(port)), nil
}

func hostLabels(host model.ResourceHost, tags map[string]string, defaultJob string) map[string]string {
	job := firstTag(tags, "prometheus.io/job", "prometheus_job")
	if job == "" {
		job = defaultJob
	}
	scheme := firstTag(tags, "prometheus.io/scheme", "prometheus_scheme")
	if scheme == "" {
		scheme = "http"
	}
	metricsPath := firstTag(tags, "prometheus.io/path", "prometheus_metrics_path")
	if metricsPath == "" {
		metricsPath = "/metrics"
	}

	labels := map[string]string{
		"job":               job,
		"resource_type":     "resource_host",
		"resource_id":       strconv.FormatInt(host.Id, 10),
		"resource_uid":      host.Uid,
		"resource_name":     host.Name,
		"stree_group":       host.StreeGroup,
		"stree_product":     host.StreeProduct,
		"stree_app":         host.StreeApp,
		"service_tree":      strings.Join([]string{host.StreeGroup, host.StreeProduct, host.StreeApp}, "."),
		"region":            host.Region,
		"cloud_provider":    host.CloudProvider,
		"instance_type":     host.InstanceType,
		"availability_zone": host.AvailabilityZone,
		"__scheme__":        scheme,
		"__metrics_path__":  metricsPath,
	}
	for key, value := range tags {
		if isControlTag(key) || value == "" {
			continue
		}
		label := "tag_" + sanitizeLabelName(key)
		if label == "tag_" {
			continue
		}
		if _, exists := labels[label]; !exists {
			labels[label] = value
		}
	}
	for key, value := range labels {
		if value == "" {
			delete(labels, key)
		}
	}
	return labels
}

func firstTag(tags map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(tags[key]); value != "" {
			return value
		}
	}
	return ""
}

func isControlTag(key string) bool {
	switch key {
	case "prometheus.io/scrape", "prometheus_scrape",
		"prometheus.io/port", "prometheus_port",
		"prometheus.io/job", "prometheus_job",
		"prometheus.io/scheme", "prometheus_scheme",
		"prometheus.io/path", "prometheus_metrics_path":
		return true
	default:
		return false
	}
}

func sanitizeLabelName(value string) string {
	value = invalidLabelCharacter.ReplaceAllString(value, "_")
	return strings.Trim(value, "_")
}
