package rpc

import (
	"context"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/task"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"time"
)

func TickerTaskReport(c *Client, ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	localIp := common.GetLocalIP()
	c.DoTaskReport(localIp)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.DoTaskReport(localIp)
		}
	}
}

func (c *Client) DoTaskReport(localIp string) {
	req := model.TaskReportRequest{
		AgentIp:     localIp,
		ReportTasks: task.Locals.ReportTasks(),
	}
	var resp model.TaskReportResponse
	err := c.Get()
	if err != nil {
		return
	}
	err = c.BRPCClient.Call("HTTPServer.TaskReport", req, &resp)
	if err != nil {
		c.Close()
		return
	}
	if resp.AssignTasks != nil {
		count := len(resp.AssignTasks)
		for i := 0; i < count; i++ {
			at := resp.AssignTasks[i]
			task.Locals.AssignTask(at)
		}
	}
}
