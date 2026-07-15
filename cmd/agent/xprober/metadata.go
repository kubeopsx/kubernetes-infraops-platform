package xprober

import (
	"context"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/toolkits/pkg/logger"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	LocalRegion string
	LocalIp     string
)

func GetLocalRegionByEc2(logger log.Logger) bool {
	addr := "http://169.254.169.254/latest/meta-data/placement/availability-zone"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
	if err != nil {
		level.Error(logger).Log("msg", "create EC2 region metadata request", "err", err)
		return false
	}
	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		level.Error(logger).Log("msg", "GetLocalRegionByEc2 http", "error", err)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		level.Error(logger).Log("msg", "GetLocalRegionByEc2 http not 200", "resp.StatusCode", resp.StatusCode)
		return false
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		level.Error(logger).Log("msg", "GetLocalRegionByEc2 http read response body", "err", err)
		return false
	}
	dataStr := string(respBytes)
	if len(dataStr) < 2 {
		level.Error(logger).Log("msg", "invalid EC2 availability zone", "value", dataStr)
		return false
	}
	region := dataStr[:len(dataStr)-1]
	LocalRegion = region
	return true
}

func GetLocalIp() string {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		logger.Errorf("get local ip err:%+v", err)
		return ""
	}
	l := strings.Split(conn.LocalAddr().String(), ":")[0]
	conn.Close()
	return l
}
