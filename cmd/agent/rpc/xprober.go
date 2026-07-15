package rpc

import (
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/toolkits/pkg/logger"
)

func (r *Client) ProberTargetSync(req common.TargetGetRequest) *common.TargetGetResponse {
	var res *common.TargetGetResponse
	err := r.Get()
	if err != nil {
		level.Error(r.Logger).Log("msg", "get cli error", "ServerAddress", r.ServerAddress, "err", err)
		return nil
	}
	err = r.BRPCClient.Call("HTTPServer.GetTargets", req, &res)
	if err != nil {
		r.Close()
		level.Error(r.Logger).Log("msg", "HTTPServer.GetTargets.error", "ServerAddress", r.ServerAddress, "err", err)
		return nil
	}
	logger.Infof("GetTargets result:%+v", res)
	return res
}

func (r *Client) PushResults(req common.ResultPushRequest) *common.ResultPushResponse {
	var res *common.ResultPushResponse
	err := r.Get()
	if err != nil {
		level.Error(r.Logger).Log("msg", "get cli error", "ServerAddress", r.ServerAddress, "err", err)
		return nil
	}
	err = r.BRPCClient.Call("HTTPServer.PushResults", req, &res)
	if err != nil {
		r.Close()
		level.Error(r.Logger).Log("msg", "HTTPServer.PushResults.error", "ServerAddress", r.ServerAddress, "err", err)
		return nil
	}
	logger.Infof("PushResults.res:%+v req:%+v", res, req.Results[0])
	return res
}
