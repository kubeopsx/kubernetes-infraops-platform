package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/metrics"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/config/server"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/inverted"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/inverted/index"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/inverted/labels"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HostIndex struct {
	Ir      *inverted.HeadIndexReader
	Logger  log.Logger
	Modulus int // 静态分片模
	Num     int
}

type ResourceIndexer interface {
	FlushIndex()                               // 刷新索引方法
	GetIndexReader() *inverted.HeadIndexReader // 获取内置索引 reader
	GetLogger() log.Logger
}

// 接口容器和对应注册方法
var indexContainer = make(map[string]ResourceIndexer)

func indexRegister(name string, ri ResourceIndexer) {
	indexContainer[name] = ri
}

// 判断索引存在
func ResourceIndexExists(name string) bool {
	_, ok := indexContainer[name]
	return ok
}

func (hi *HostIndex) FlushIndex() {
	r := new(model.ResourceHost)
	total := int(r.Count())
	ids := ""
	start := time.Now()
	metrics.ResourceNumCount.With(prometheus.Labels{common.LABEL_RESOURCE_TYPE: common.RESOURCE_HOST}).Set(float64(total))
	for i := 1; i <= total+1; i++ {
		if hi.Modulus == 0 {
			ids += fmt.Sprintf("%d,", i)
			continue
		}
		if i%hi.Modulus == hi.Num {
			ids += fmt.Sprintf("%d,", i)
			continue
		}
	}
	ids = strings.TrimRight(ids, ",")
	sql := fmt.Sprintf("id in (%s)", ids)
	obj, err := model.GetResourceHostMultiple(sql)
	if err != nil {
		return
	}
	thisH := inverted.NewHeadReader()

	thisGPAS := map[string]struct{}{}
	for _, item := range obj {
		m := make(map[string]string)
		m["hash"] = item.Hash
		tags := make(map[string]string)
		prIps := []string{}
		puIps := []string{}

		m["uid"] = item.Uid
		m["name"] = item.Name
		m["cloud_provider"] = item.CloudProvider
		m["charging_mode"] = item.ChargingMode
		m["region"] = item.Region
		m["instance_type"] = item.InstanceType
		m["avaliability_zone"] = item.AvailabilityZone
		m["vpc_id"] = item.VpcId
		m["subnet_id"] = item.SubnetId
		m["status"] = item.Status
		m["account_id"] = strconv.FormatInt(item.AccountId, 10)

		json.Unmarshal([]byte(item.PrivateIps), &prIps)
		json.Unmarshal([]byte(item.PublicIps), &puIps)

		m["stree_group"] = item.StreeGroup
		m["stree_product"] = item.StreeProduct
		m["stree_app"] = item.StreeApp

		thisGPAS[fmt.Sprintf("%s.%s.%s", item.StreeGroup, item.StreeProduct, item.StreeApp)] = struct{}{}

		thisH.GetOrCreateWithID(uint64(item.Id), item.Hash, mapTolsets(m))
		thisH.GetOrCreateWithID(uint64(item.Id), item.Hash, mapTolsets(tags))

		for _, i := range prIps {
			mp := map[string]string{
				"private_ip": i,
			}
			thisH.GetOrCreateWithID(uint64(item.Id), item.Hash, mapTolsets(mp))
		}
		for _, i := range puIps {
			mp := map[string]string{
				"public_ip": i, // 这里可以区分是 public_ip 还是 private_ip
			}
			thisH.GetOrCreateWithID(uint64(item.Id), item.Hash, mapTolsets(mp))
		}
	}
	hi.Ir.Reset(thisH)
	level.Debug(hi.Logger).Log("msg", "FlushIndex time took", "took", time.Since(start).Seconds())
	go func() {
		level.Info(hi.Logger).Log("msg", "FlushIndex Add GPA To PATH",
			"num", len(thisGPAS),
		)
		for node := range thisGPAS {
			inputs := common.NodeRequest{
				Node: node,
			}
			model.StreeAdd(&inputs, hi.Logger)
		}
	}()
}

func (hi *HostIndex) GetIndexReader() *inverted.HeadIndexReader {
	return hi.Ir
}

func (hi *HostIndex) GetLogger() log.Logger {
	return hi.Logger
}

func GetResourceIndexReader(name string) (bool, ResourceIndexer) {
	ri, ok := indexContainer[name]
	return ok, ri
}

func GetAllResourceIndexReader() (make map[string]ResourceIndexer) {
	return indexContainer
}

func mapTolsets(m map[string]string) labels.Labels {
	var lset labels.Labels
	for k, v := range m {
		l := labels.Label{
			Name:  k,
			Value: v,
		}
		lset = append(lset, l)
	}
	return lset
}

type MemPostings struct {
	mtx     sync.RWMutex
	m       map[string]map[string][]uint64
	ordered bool
}

type HeadIndexReader struct {
	postings *MemPostings
}

func New(logger log.Logger, ims []*server.InvertedIndexConfig) {
	loadNum := 0
	loadResource := make([]string, 0)
	for _, i := range ims {
		if !i.Enabled {
			continue
		}
		level.Info(logger).Log("msg", "index new", "name", i.ResourceName)
		loadNum += 1
		loadResource = append(loadResource, i.ResourceName)
		switch i.ResourceName {
		case common.RESOURCE_HOST:
			mi := &HostIndex{
				Ir:      inverted.NewHeadReader(),
				Logger:  logger,
				Modulus: i.Modulus,
				Num:     i.Num,
			}
			indexRegister(i.ResourceName, mi)
		case common.RESOURCE_RDS:
			mi := &HostIndex{
				Ir:      inverted.NewHeadReader(),
				Logger:  logger,
				Modulus: i.Modulus,
				Num:     i.Num,
			}
			indexRegister(i.ResourceName, mi)
		}
	}
	level.Info(logger).Log("msg", "index new summary", "loadNum", loadNum, "detail", strings.Join(loadResource, " "))
}

func GetMatchIdsByIndex(req common.ResourceQueryRequest) (matchIds []uint64) {
	ri, ok := indexContainer[req.ResourceType]
	if !ok {
		return
	}
	matcher := common.FormatLabelMatcher(req.Labels)

	p, err := inverted.PostingsForMatchers(ri.GetIndexReader(), matcher...)
	if err != nil {
		level.Error(ri.GetLogger()).Log("msg", "inverted PostingsForMatchers error", "ResourceType", req.ResourceType)
		return
	}
	matchIds, err = index.ExpandPostings(p)
	fmt.Println(matchIds)
	if err != nil {
		level.Error(ri.GetLogger()).Log("msg", "index ExpandPostings error", "ResourceType", req.ResourceType)
		return
	}
	return
}

func RevertedIndexSyncManager(ctx context.Context, logger log.Logger) error {
	level.Error(logger).Log("msg", "run reverted index sync manager", "resource_num", len(indexContainer))
	ticker := time.NewTicker(15 * time.Second)
	doIndexFlush()
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			doIndexFlush()
		}
	}
}

func doIndexFlush() {
	var wg sync.WaitGroup
	wg.Add(len(indexContainer))
	for name, ir := range indexContainer {
		name := name
		ir := ir
		go func() {
			defer wg.Done()
			start := time.Now()
			ir.FlushIndex()
			// 遍历调用各个resource的flushindex方法时可以在外层及时算刷新耗时
			metrics.IndexFlushDuration.With(prometheus.Labels{common.LABEL_RESOURCE_TYPE: name}).Set(float64(time.Since(start).Seconds()))
		}()
	}
	wg.Wait()
}
