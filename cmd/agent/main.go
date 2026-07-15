package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/consumer"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/logging/counter"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/metrics"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/rpc"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/task"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/agent/xprober"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/config/agent"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "configFile", "agent.yaml", "")
	flag.Parse()

	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = level.NewFilter(logger, level.AllowInfo())
	logger = log.With(logger, "ts", log.DefaultTimestamp, "caller", log.DefaultCaller)

	cfg, err := agent.LoadFile(configFile)
	if err != nil {
		level.Error(logger).Log("msg", "config load file err", "err", err)
		return
	}
	level.Info(logger).Log("msg", "config load file success")

	r := rpc.NewClient(cfg.RPCServerAddress, logger)
	r.Ping()

	metricsMap := metrics.Create(cfg.LogStrategies)
	for _, v := range metricsMap {
		prometheus.MustRegister(v)
	}

	task.NewLocals(cfg.Task.MetaDir)

	xprober.New(logger, cfg.Region)

	// 统计指标的同步queue
	cq := make(chan *consumer.AnalystPoint, common.CounterQueueSize)
	// 统计指标的管理器
	pm := counter.NewPointCounterManager(cq, metricsMap)
	// 日志job管理器
	loggingManager := logging.NewManager(cq)

	// 把配置文件中的log job 传入
	syncChan := make(chan []*logging.Logging, 1)
	jobs := make([]*logging.Logging, 0)
	for _, i := range cfg.LogStrategies {
		i := i
		j := &logging.Logging{
			S: i,
		}
		jobs = append(jobs, j)
	}
	syncChan <- jobs

	var g run.Group
	// 所有用到 ctx 的人，都会接收到 ctx.down，相当于共同进退
	ctx, cancel := context.WithCancel(context.Background())
	fmt.Println(ctx)
	{
		// 处理信号的 handler
		// 发送 term 信号 cancelCh 他能够接收到这个信号，接收到信号弄到 chan 里面
		// 然后在这里面监听这个 chan
		term := make(chan os.Signal, 1)
		signal.Notify(term, syscall.SIGTERM, syscall.SIGQUIT)
		cancelCh := make(chan struct{})
		g.Add(
			func() error {
				select {
				case <-term:
					// 有数据的时候有人 cancelCh 他了，那我就要退出 cancel()
					level.Warn(logger).Log("msg", "receive SIGTERM, exiting gracefully....")
					cancel()
					return nil
				// 如果是 term 拿到的说明是我这个人触发的这个group的退出，说明接收到了监听信号
				// 如果不能 cancelCh 的话，别人退出的，我就会一直阻塞在这里
				// cancelCh	给 close 用的
				// 很多人都在监听他，只要有人close了他所有监听的人都能收到消息，类似广播
				// 只要这个 cancelCh 被 close 了我就能拿到，会被后面的 func 执行
				case <-cancelCh:
					level.Warn(logger).Log("msg", "other cancel exiting")
					return nil
				}
			},
			func(err error) {
				close(cancelCh)
			},
		)
	}
	{
		g.Add(func() error {
			errChan := make(chan error)
			go func() {
				errChan <- metrics.Start(cfg.HTTPAddress)
			}()
			select {
			case err := <-errChan:
				return err
			case <-ctx.Done():
				return nil
			}
		}, func(err error) {
			cancel()
		})
	}

	if cfg.CollectAndReport {
		g.Add(func() error {
			err := rpc.CollectWithReport(r, ctx, logger)
			if err != nil {
				level.Error(logger).Log("msg", "CollectWithReport.error", "err", err)
				return err
			}
			return nil
		}, func(err error) {
			cancel()
		})
	}
	if cfg.Log {
		g.Add(func() error {
			// logging和server直接同步
			err := logging.TickerLoggingSync(r, ctx, syncChan, jobs, metricsMap, common.GetHostName())
			if err != nil {
				level.Error(logger).Log("msg", "SetMetricsManager.error", "err", err)
				return err
			}
			return nil
		}, func(err error) {
			cancel()
		})
		g.Add(func() error {
			// 增加增量同步策略
			err := loggingManager.SyncManager(ctx, syncChan)
			if err != nil {
				level.Error(logger).Log("msg", "SyncManager.error", "err", err)
				return err
			}
			return nil
		}, func(err error) {
			cancel()
		})
		g.Add(func() error {
			// 增加增量同步策略
			err := pm.Update(ctx)
			if err != nil {
				level.Error(logger).Log("msg", "Update.error", "err", err)
				return err
			}
			return nil
		}, func(err error) {
			cancel()
		})
		g.Add(func() error {
			// 统计任务实体转化为prometheus的metrics的任务
			err := pm.SetMetricsManager(ctx)
			if err != nil {
				level.Error(logger).Log("msg", "SetMetricsManager.error", "err", err)
				return err
			}
			return nil
		}, func(err error) {
			cancel()
		})
	}
	g.Add(func() error {
		err := rpc.TickerTaskReport(r, ctx)
		if err != nil {
			level.Error(logger).Log("msg", "TickerTaskReport.error", "err", err)
			return err
		}
		return nil
	}, func(err error) {
		cancel()
	})
	g.Add(func() error {
		err := xprober.TickerGetTargets(r, ctx)
		if err != nil {
			level.Error(logger).Log("msg", "xprober.TickerGetTargets.error", "err", err)
			return err
		}
		return err
	}, func(err error) {
		cancel()
	})
	g.Add(func() error {
		err := xprober.TickerPushResults(r, ctx)
		if err != nil {
			level.Error(logger).Log("msg", "xprober.TickerPushResults.error", "err", err)
			return err
		}
		return err
	}, func(err error) {
		cancel()
	})
	g.Run()
}
