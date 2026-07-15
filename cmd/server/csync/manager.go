package csync

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/db"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"math/rand"
	"sync"
	"time"
)

type CloudAssetsResource interface {
	sync()
}

type AliyunCloud struct{}

type TencentCloud struct{}

type CloudHostSync struct {
	AliyunCloud
	TencentCloud
	TableName string
	Logger    log.Logger
}

func Manager(ctx context.Context, logger log.Logger) error {
	level.Info(logger).Log("msg", "run cloud assets manager", "resource_number", len(cloudAssetsResourceContainer))
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			doCloudAssetsSync(logger)
		}
	}
}

var cloudAssetsResourceContainer = make(map[string]CloudAssetsResource)

func New(logger log.Logger) {
	hs := &CloudHostSync{
		TableName: common.RESOURCE_HOST,
		Logger:    logger,
	}
	cloudAssetsRegister(common.RESOURCE_HOST, hs)
}

func cloudAssetsRegister(name string, car CloudAssetsResource) {
	cloudAssetsResourceContainer[name] = car
}

func doCloudAssetsSync(logger log.Logger) {
	var wg sync.WaitGroup
	wg.Add(len(cloudAssetsResourceContainer))
	for _, cloudAssets := range cloudAssetsResourceContainer {
		cloudAssets := cloudAssets
		go func() {
			defer wg.Done()
			cloudAssets.sync()
		}()
	}
	wg.Wait()
}

func (chs *CloudHostSync) sync() {
	startTime := time.Now()
	mock := getMockResourceHost()

	uh, err := getHostUIDAndHash()
	if err != nil {
		return
	}

	toAddSet := make([]model.ResourceHost, 0)
	toUpdateSet := make([]model.ResourceHost, 0)
	toDelIds := make([]string, 0)

	localUidSet := make(map[string]struct{})

	var toAddNum, toUpdateNum, toDelNum int
	var suAddNum, suUpdateNum, suDelNum int

	for _, data := range mock {
		localUidSet[data.Uid] = struct{}{}
		h, ok := uh[data.Uid]
		if !ok {
			toAddSet = append(toAddSet, data)
			toAddNum++
			continue
		}
		if h == data.Hash {
			continue
		}
		toUpdateSet = append(toUpdateSet, data)
		toUpdateNum++
	}

	for uid := range uh {
		if _, ok := localUidSet[uid]; !ok {
			toDelIds = append(toDelIds, uid)
			toDelNum++
		}
	}

	for _, h := range toAddSet {
		err := h.Add()
		if err != nil {
			level.Error(chs.Logger).Log("msg", "resource host add err", "err", err, "name", h.Name)
			continue
		}
		suAddNum++
	}
	for _, h := range toUpdateSet {
		update, err := h.UpdateByUID(h.Uid)
		if err != nil {
			level.Error(chs.Logger).Log("msg", "resource host sync.err", "err", err, "name", h.Name)
			continue
		}
		if update {
			suUpdateNum++
		}
		if len(toDelIds) > 0 {
			num, _ := model.BatchDeleteResource(common.RESOURCE_HOST, "uid", toDelIds)
			suDelNum = int(num)
		}
		timeTook := time.Since(startTime)
		level.Info(chs.Logger).Log("msg", "resource host sync result",
			"publicCloud", len(mock),
			"db", len(uh),
			"toAddNum", toAddNum,
			"toUpdateNum", toUpdateNum,
			"toDelNum", toDelNum,
			"suAddNum", suAddNum,
			"suUpdateNum", suUpdateNum,
			"suDelNum", suDelNum,
			"timeTook", timeTook.Seconds(),
		)
	}
}

func getMockResourceHost() []model.ResourceHost {
	rand.Seed(time.Now().UnixNano())

	randGs := []string{"infra", "ads", "web", "sys"}
	randPs := []string{"monitor", "cicd", "k8s", "mq"}
	randAs := []string{"kafka", "prometheus", "zookeeper", "es"}

	randCpus := []string{"4", "8", "16", "32", "64", "128"}
	randMems := []string{"8", "16", "32", "64", "128", "256", "512"}
	randDisks := []string{"128", "256", "512", "1024", "2048", "4096", "8192"}

	randMapKeys := []string{"arch", "idc", "os"}
	randMapValues := []string{"mac", "linux", "beijing", "shanghai", "windows", "arm64", "amd64", "darwin"}

	randRegions := []string{"beijing", "shanghai", "guangzhou", "tianjin", "shandong"}
	randCloudProvider := []string{"alibaba", "tencent", "aws", "huawei", "azure"}
	randClusters := []string{"bigdata", "infra", "middleware", "business"}
	randInts := []string{"4c8g", "4c16g", "8c32g", "16c64g"}

	frn := func(n int) int {
		rand.Seed(time.Now().UnixNano())
		return rand.Intn(n)
	}
	frNum := func() int {
		rand.Seed(time.Now().UnixNano())
		return int(rand.Int63n(380-65) + 165)
	}
	hs := make([]model.ResourceHost, 0)
	for i := 0; i < frNum(); i++ {
		randN := i
		name := fmt.Sprintf("genMockResouceHost_host_%d", randN)
		ips := []string{fmt.Sprintf("8.8.8.%d", randN)}
		ipJ, _ := json.Marshal(ips)
		h := model.ResourceHost{
			Name:       name,
			PrivateIps: ipJ,
			Cpu:        randCpus[frn(len(randCpus)-1)],
			Mem:        randMems[frn(len(randMems)-1)],
			Disk:       randDisks[frn(len(randDisks)-1)],

			StreeGroup:   randGs[frn(len(randGs)-1)],
			StreeProduct: randPs[frn(len(randPs)-1)],
			StreeApp:     randAs[frn(len(randAs)-1)],

			Region:        randRegions[frn(len(randRegions)-1)],
			CloudProvider: randCloudProvider[frn(len(randCloudProvider)-1)],
			InstanceType:  randInts[frn(len(randInts)-1)],
		}
		tagM := make(map[string]string)
		for _, i := range randMapKeys {
			tagM[i] = randMapValues[frn(len(randMapValues)-1)]
		}
		tagM["cluster"] = randClusters[frn(len(randClusters)-1)]
		tagMJ, _ := json.Marshal(tagM)
		h.Tags = tagMJ

		hash := h.GenHash()
		h.Hash = hash
		md5o := md5.New()
		md5o.Write([]byte(name))
		h.Uid = hex.EncodeToString(md5o.Sum(nil))
		hs = append(hs, h)
	}
	return hs
}

func getHostUIDAndHash() (map[string]string, error) {
	var obj []model.ResourceHost
	err := db.Database["stree"].Cols("uid", "hash").Find(&obj)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, h := range obj {
		m[h.Uid] = h.Hash
	}
	return m, nil
}
