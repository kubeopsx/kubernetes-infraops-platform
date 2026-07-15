package rpc

import (
	"context"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"time"
)

func (c *Client) HostReport(collect model.Collect) {
	var msg string
	err := c.Get()
	if err != nil {
		level.Error(c.Logger).Log("msg", "get client error", "serverAddress", c.ServerAddress)
		return
	}
	err = c.BRPCClient.Call("HTTPServer.HostReport", collect, &msg)
	if err != nil {
		defer c.Close()
		level.Error(c.Logger).Log("msg", "HTTPServer.HostReport error", "serverAddress", c.ServerAddress, "err", err)
		return
	}
}

func CollectBase(client *Client, logger log.Logger) {
	var (
		err  error
		sn   string
		cpu  string
		mem  string
		disk string
	)
	// https://uzzju.com/post/47.java
	snCloudHost := `curl -s http://169.254.169.254/latest/meta-data/instance-id`
	snHost := `dmidecode -s system-serial-number | tail -n 1 | tr -d "\n"`

	cpuCmd := `cat /proc/cpuinfo | grep processor | wc -l | tr -d "\n"`
	memCmd := `cat /proc/meminfo | grep MemTotal | awk '{printf "%d",$2/1024/1024}'`
	diskCmd := `df -m | grep '/dev/' | grep -v '/var/lib' | grep -v tmpfs | awk '{sum += $2}; END {printf "%d", sum / 1024}'`

	sn, err = common.GetCommand(snCloudHost)
	if err != nil || sn == "" {
		sn, err = common.GetCommand(snHost)
		if err != nil {
			level.Error(logger).Log("msg", "sn host.error", "shell", snHost, "error", err)
		}
	}
	level.Info(logger).Log("msg", "collect base", "sn", sn)

	cpu, err = common.GetCommand(cpuCmd)
	if err != nil {
		level.Error(logger).Log("msg", "cpu command error", "shell", cpuCmd, "error", err)
	}
	level.Info(logger).Log("msg", "collect base", "cpu", cpu)

	mem, err = common.GetCommand(memCmd)
	if err != nil {
		level.Error(logger).Log("msg", "mem command error", "shell", memCmd, "error", err)
	}
	level.Info(logger).Log("msg", "Collect Base", "mem", mem)

	disk, err = common.GetCommand(diskCmd)
	if err != nil {
		level.Error(logger).Log("msg", "disk command error", "shell", diskCmd, "error", err)
	}
	level.Info(logger).Log("msg", "Collect Base", "disk", disk)

	ip := common.GetLocalIP()
	hostname := common.GetHostName()

	hostObj := model.Collect{
		SN:       sn,
		CPU:      cpu,
		Mem:      mem,
		Disk:     disk,
		IpAddr:   ip,
		HostName: hostname,
	}
	client.HostReport(hostObj)
}

func CollectWithReport(client *Client, ctx context.Context, logger log.Logger) error {
	ticker := time.NewTicker(5 * time.Second)
	level.Info(logger).Log("msg", "collect with report enabled")
	CollectBase(client, logger)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			level.Info(logger).Log("msg", "receive quit signal and quit")
			return nil
		case <-ticker.C:
			CollectBase(client, logger)
		}
	}
}
