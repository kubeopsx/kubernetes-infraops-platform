package rpc

import (
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/toolkits/pkg/logger"
)

func (r *Client) Sync(hostname string) []*model.LogStrategy {
	var result []*model.LogStrategy
	err := r.Get()
	if err != nil {
		level.Error(r.Logger).Log("msg", "get client error", "serverAddr", r.ServerAddress, "err", err)
		return nil
	}
	err = r.BRPCClient.Call("HTTPServer.Sync", hostname, &result)
	if err != nil {
		r.Close()
		level.Error(r.Logger).Log("msg", "server client error", "serverAddr", r.ServerAddress, "err", err)
		return nil
	}
	logger.Infof("sync result:%+v", result)
	return result
}
