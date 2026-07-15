package rpc

import (
	"bufio"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/toolkits/pkg/net/gobrpc"
	"github.com/ugorji/go/codec"
	"io"
	"net"
	"net/rpc"
	"reflect"
	"time"
)

type Client struct {
	BRPCClient    *gobrpc.RPCClient
	ServerAddress string
	Logger        log.Logger
}

func NewClient(serverAddr string, logger log.Logger) *Client {
	r := &Client{
		ServerAddress: serverAddr,
		Logger:        logger,
	}
	return r
}

func (r *Client) Get() error {
	if r.BRPCClient != nil {
		return nil
	}
	conn, err := net.DialTimeout("tcp", r.ServerAddress, time.Second*5)
	if err != nil {
		level.Error(r.Logger).Log("msg", "dial server failed", "serverAddress", r.ServerAddress, "err", err)
		return err
	}
	var connBuf = struct {
		io.Closer
		*bufio.Reader
		*bufio.Writer
	}{conn, bufio.NewReader(conn), bufio.NewWriter(conn)}

	var mh codec.MsgpackHandle
	mh.MapType = reflect.TypeOf(map[string]interface{}(nil))

	rpcCodec := codec.MsgpackSpecRpc.ClientCodec(connBuf, &mh)
	client := rpc.NewClientWithCodec(rpcCodec)
	r.BRPCClient = gobrpc.NewRPCClient(r.ServerAddress, client, 5*time.Second)
	return nil
}

func (r *Client) Close() {
	if r.BRPCClient != nil {
		defer r.BRPCClient.Close()
		r.BRPCClient = nil
	}
}
