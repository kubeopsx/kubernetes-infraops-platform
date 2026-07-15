package rpc

import (
	"github.com/go-kit/log/level"
)

func (r *Client) Ping() {
	var msg string
	err := r.Get()
	if err != nil {
		level.Error(r.Logger).Log("msg", "get cli error", "serverAddress", r.ServerAddress, "err", err)
		return
	}
	err = r.BRPCClient.Call("HTTPServer.Ping", "agent", &msg)
	if err != nil {
		level.Error(r.Logger).Log("msg", "server ping error", "serverAddress", r.ServerAddress, "err", err)
		return
	}
	level.Info(r.Logger).Log("msg", "server ping success", "serverAddress", r.ServerAddress, "msg", msg)

}
