package rpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/task"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/xprober"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"io"
	"net"
	"net/rpc"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/toolkits/pkg/logger"
	"github.com/ugorji/go/codec"
)

type Server int

func (*Server) Ping(input string, output *string) error {
	fmt.Println(input)
	*output = "Success"
	return nil
}

func (*Server) HostReport(input model.Collect, output *string) error {
	fmt.Println(input)
	*output = "Success"
	ips := []string{input.IpAddr}
	ipJson, _ := json.Marshal(ips)
	if input.SN == "" {
		input.SN = input.HostName
	}
	if input.SN == "" {
		*output = "sn empty"
		return nil
	}

	rh := model.ResourceHost{
		Uid:        input.SN,
		Name:       input.HostName,
		PrivateIps: ipJson,
		Cpu:        input.CPU,
		Mem:        input.Mem,
		Disk:       input.Disk,
	}

	hash := rh.GenHash()

	rhUid := model.ResourceHost{
		Uid: input.SN,
	}
	rhUidDataBase, err := rhUid.Get()
	if err != nil {
		*output = "db error"
		return nil
	}
	if rhUidDataBase == nil {
		rh.Hash = hash
		err = rh.Add()
		if err != nil {
			*output = fmt.Sprintf("db error %v", err)
		} else {
			*output = "insert success"
		}
		return nil
	}
	if rhUidDataBase.Hash != hash {
		rh.Hash = hash
		update, err := rh.Update()
		if err != nil {
			*output = "update error"
			return nil
		}
		if update {
			*output = "update success"
			return nil
		}
	}
	return nil
}

func (*Server) Sync(input string, output *[]*model.LogStrategy) error {
	obj, err := model.Gets("id>0")
	if err != nil {
		logger.Errorf("failed to get log strategies: %v", err)
		return err
	}
	*output = obj
	logger.Infof("sync call receive: %v %v %v", obj, input, output)
	return nil
}

func (*Server) TaskReport(args model.TaskReportRequest, reply *model.TaskReportResponse) error {
	toMarkDoneIds := make(map[int64]struct{})
	if len(args.ReportTasks) > 0 {
		for _, x := range args.ReportTasks {
			tRes := model.TaskResult{
				ID:     0,
				TaskID: x.Id,
				Host:   args.AgentIp,
				Status: x.Status,
				Stdout: x.Stdout,
				Stderr: x.Stderr,
			}
			err, added := tRes.Save()
			if err != nil {
				logger.Errorf("err:%+v", err)
				return err
			}
			if added {
				logger.Infof("agent ip:%+v res:%+v", args.AgentIp, tRes)
				toMarkDoneIds[x.Id] = struct{}{}
			}
		}

		for id, _ := range toMarkDoneIds {
			err := model.DoTaskMetaMark(id)
			if err != nil {
				logger.Errorf("err:%+v", err)
				return err
			} else {
				logger.Infof("agent ip:%+v id:%+v", args.AgentIp, id)
			}
		}
	}
	reply.AssignTasks = task.Caches.GetTasksByIp(args.AgentIp)
	return nil
}

func (*Server) GetTargets(req common.TargetGetRequest, res *common.TargetGetResponse) error {
	region := req.LocalRegion
	xprober.RecordAgent(req.LocalIp, req.LocalRegion)
	targets := xprober.GetTargetsByRegion(region)
	res.Targets = targets
	return nil
}

func (*Server) PushResults(req common.ResultPushRequest, res *common.ResultPushResponse) error {
	res.SuccessNum = int32(xprober.StoreResults(req.Results))
	return nil
}

func Run(rpcAddress string, logger log.Logger) error {
	srv := rpc.NewServer()
	if err := srv.RegisterName("HTTPServer", new(Server)); err != nil {
		return err
	}

	listen, err := net.Listen("tcp", rpcAddress)
	if err != nil {
		level.Error(logger).Log("msg", "failed to listen address", "rpcAddress", rpcAddress, "err", err)
		return err
	}
	level.Info(logger).Log("msg", "rpc server available at", "rpcAddress", rpcAddress)

	var mh codec.MsgpackHandle
	mh.MapType = reflect.TypeOf(map[string]interface{}(nil))

	for {
		conn, err := listen.Accept()
		if err != nil {
			level.Warn(logger).Log("msg", "listen accept err", "err", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		var connBuf = struct {
			io.Closer
			*bufio.Reader
			*bufio.Writer
		}{conn, bufio.NewReader(conn), bufio.NewWriter(conn)}
		go srv.ServeCodec(codec.MsgpackSpecRpc.ServerCodec(connBuf, &mh))
	}
}
