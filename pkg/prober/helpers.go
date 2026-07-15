package prober

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func timeoutFor(ctx context.Context, configured time.Duration) time.Duration {
	if configured <= 0 {
		configured = DefaultTimeout
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < configured {
			return remaining
		}
	}
	return configured
}

func boolOption(options map[string]string, key string, fallback bool) bool {
	raw, ok := options[key]
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func intOption(options map[string]string, key string, fallback int) int {
	raw, ok := options[key]
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func targetHost(address string) string {
	if parsed, err := url.Parse(address); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	if host, _, err := net.SplitHostPort(address); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(address, "[]")
}

func addressWithDefaultPort(address, defaultPort string) string {
	if _, _, err := net.SplitHostPort(address); err == nil {
		return address
	}
	if parsed, err := url.Parse(address); err == nil && parsed.Hostname() != "" {
		if parsed.Port() != "" {
			return net.JoinHostPort(parsed.Hostname(), parsed.Port())
		}
		return net.JoinHostPort(parsed.Hostname(), defaultPort)
	}
	host := targetHost(address)
	return net.JoinHostPort(host, defaultPort)
}

func milliseconds(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(d) / float64(time.Millisecond)
}
