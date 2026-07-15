package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/csync"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/indexer"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/metrics"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/metrics/statistic"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/rpc"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/task"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/xprober"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/config/server"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/db"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/web"
	"github.com/oklog/run"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "configFile", "server.yaml", "")
	flag.Parse()

	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = level.NewFilter(logger, level.AllowInfo())
	logger = log.With(logger, "ts", log.DefaultTimestamp, "caller", log.DefaultCaller)

	cfg, err := server.LoadFile(configFile)
	if err != nil {
		level.Error(logger).Log("msg", "config load file err", "err", err)
		return
	}
	level.Info(logger).Log("msg", "config load file success", "path", configFile, "mysql count", len(cfg.Mysql))

	if err := db.InitialDatabase(cfg.Mysql); err != nil {
		level.Error(logger).Log("msg", "initial database failed", "err", err)
		return
	}
	level.Info(logger).Log("msg", "initial database success", "db count", len(db.Database))

	// model.AddTest(logger)
	// model.QueryCase1Test(logger)
	// model.QueryCase2Test(logger)
	// model.QueryCase3Test(logger)
	// model.DeleteTest(logger)

	// 初始化倒排索引
	indexer.New(logger, cfg.InvertedIndex)

	task.CacheNew()

	xprober.New()

	// 注册相关metrics
	metrics.New()

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
	g.Add(func() error {
		errChan := make(chan error, 1)
		go func() {
			errChan <- rpc.Run(cfg.RPCAddress, logger)
		}()
		select {
		case err := <-errChan:
			level.Error(logger).Log("msg", "rpc server error", "file.path", "err", err)
			return err
		case <-ctx.Done():
			level.Error(logger).Log("msg", "rpc server error", "file.path", "err", err)
			return nil
		}
	}, func(err error) {
		cancel()
	})
	g.Add(func() error {
		errChan := make(chan error, 1)
		go func() {
			errChan <- web.Run(cfg.HTTPAddress, logger)
		}()
		select {
		case err := <-errChan:
			level.Error(logger).Log("msg", "web server error", "file.path", "err", err)
			return err
		case <-ctx.Done():
			level.Error(logger).Log("msg", "web server error", "file.path", "err", err)
			return nil
		}
	}, func(err error) {
		cancel()
	})
	if cfg.PublicCloudAssetSync.Enabled {
		csync.New(logger)
		g.Add(func() error {
			err := csync.Manager(ctx, logger)
			if err != nil {
				level.Error(logger).Log("msg", "cloud sync manager,err", "err", err)
			}
			return err
		}, func(err error) {
			cancel()
		})
	}
	g.Add(func() error {
		err := indexer.RevertedIndexSyncManager(ctx, logger)
		if err != nil {
			level.Error(logger).Log("msg", "reverted index sync manager,err", "err", err)

		}
		return err
	}, func(err error) {
		cancel()
	},
	)
	g.Add(func() error {
		err := statistic.TreeStatisticManager(ctx, logger)
		if err != nil {
			level.Error(logger).Log("msg", "tree statistic manager err", "err", err)
		}
		return err
	}, func(err error) {
		cancel()
	},
	)
	g.Add(func() error {
		err := task.SyncTaskManager(ctx, logger)
		if err != nil {
			level.Error(logger).Log("msg", "sync task manager err", "err", err)
		}
		return err
	}, func(err error) {
		cancel()
	},
	)
	tfm := xprober.NewTargetFlushManager(logger, configFile)
	g.Add(func() error {
		err := tfm.Run(ctx)
		return err
	}, func(err error) {
		cancel()
	})
	g.Add(func() error {
		err := xprober.DataProcess(ctx, logger)
		return err
	}, func(err error) {
		cancel()
	})
	g.Run()
}
