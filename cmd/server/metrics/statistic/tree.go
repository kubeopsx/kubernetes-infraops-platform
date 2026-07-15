package statistic

import (
	"context"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/indexer"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/metrics"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"strings"
	"time"
)

// 根据倒排索引的分组统计指标
// 比如看到 host 在 region 、cloud_provider 等关键标签上的分布情况
// 也就是要调用各个资源索引的 group 接口，传入 region、cloud provider 等关键标签即可
func TreeStatisticManager(ctx context.Context, logger log.Logger) error {
	level.Info(logger).Log("msg", "run tree statistic manager")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			statisticWorker(logger)
		}
	}
}

func statisticWorker(logger log.Logger) {
	irs := indexer.GetAllResourceIndexReader()
	level.Info(logger).Log("msg", "statisticWorker start num", "num", len(irs))

	qReq := &common.NodeRequest{
		QueryType: 5,
	}
	allGPAS := model.StreeQuery(qReq, logger)
	metrics.GPACount.Set(float64(len(allGPAS)))

	for resourceType, ir := range irs {
		resourceType := resourceType
		ir := ir
		// 按 region 的分布
		go func() {
			// 全局
			regions := ir.GetIndexReader().GetGroupByLabel(common.LABEL_REGION)

			for _, i := range regions.Group {
				metrics.ResourceNumRegionCount.With(prometheus.Labels{
					common.LABEL_RESOURCE_TYPE: resourceType,
					common.LABEL_REGION:        i.Name,
				}).Set(float64(i.Value))
			}
			// 按 cloud provider 的分布
			clouds := ir.GetIndexReader().GetGroupByLabel(common.LABEL_CLOUD_PROVIDER)

			for _, i := range clouds.Group {
				metrics.ResourceNumCloudProviderCount.With(prometheus.Labels{
					common.LABEL_RESOURCE_TYPE:  resourceType,
					common.LABEL_CLOUD_PROVIDER: i.Name,
				}).Set(float64(i.Value))
			}

			// 按 cluster 的分布
			clusters := ir.GetIndexReader().GetGroupByLabel(common.LABEL_CLUSTER)
			for _, i := range clusters.Group {
				metrics.ResourceNumClusterCount.With(prometheus.Labels{
					common.LABEL_RESOURCE_TYPE: resourceType,
					common.LABEL_CLUSTER:       i.Name,
				}).Set(float64(i.Value))
			}

			// 单个g.p.a
			for _, gpa := range allGPAS {

				ss := strings.Split(gpa, ".")

				if len(ss) != 3 {
					continue
				}
				g := ss[0]
				p := ss[1]
				a := ss[2]

				csG := &common.SingleTagRequest{Key: common.LABEL_STREE_G, Value: g, Type: 1}
				csP := &common.SingleTagRequest{Key: common.LABEL_STREE_P, Value: p, Type: 1}
				csA := &common.SingleTagRequest{Key: common.LABEL_STREE_A, Value: a, Type: 1}

				matcherG := []*common.SingleTagRequest{csG}
				matcherGP := []*common.SingleTagRequest{csG, csP}
				matcherGPA := []*common.SingleTagRequest{csG, csP, csA}

				gpaNumWorker(resourceType, g, matcherG, metrics.GPAAllNumCount)
				gpaNumWorker(resourceType, g+"."+p, matcherGP, metrics.GPAAllNumCount)
				gpaNumWorker(resourceType, g+"."+p+"."+a, matcherGPA, metrics.GPAAllNumCount)

				// 这是 g 的按不同标签的分布
				gpaLabelNumWorker(resourceType, common.LABEL_REGION, g, matcherG, ir, metrics.GPAAllNumRegionCount)
				gpaLabelNumWorker(resourceType, common.LABEL_CLOUD_PROVIDER, g, matcherG, ir, metrics.GPAAllNumCloudProviderCount)
				gpaLabelNumWorker(resourceType, common.LABEL_CLUSTER, g, matcherG, ir, metrics.GPAAllNumClusterCount)
				gpaLabelNumWorker(resourceType, common.LABEL_INSTANCE_TYPE, g, matcherG, ir, metrics.GPAAllNumInstanceTypeCount)
				// 这是 g.p 的按不同标签的分布
				gpaLabelNumWorker(resourceType, common.LABEL_REGION, g+"."+p, matcherGP, ir, metrics.GPAAllNumRegionCount)
				gpaLabelNumWorker(resourceType, common.LABEL_CLOUD_PROVIDER, g+"."+p, matcherGP, ir, metrics.GPAAllNumCloudProviderCount)
				gpaLabelNumWorker(resourceType, common.LABEL_CLUSTER, g+"."+p, matcherGP, ir, metrics.GPAAllNumClusterCount)
				gpaLabelNumWorker(resourceType, common.LABEL_INSTANCE_TYPE, g+"."+p, matcherGP, ir, metrics.GPAAllNumInstanceTypeCount)
				// 这是 g.p.a 的按不同标签的分布
				gpaLabelNumWorker(resourceType, common.LABEL_REGION, g+"."+p+"."+a, matcherGPA, ir, metrics.GPAAllNumRegionCount)
				gpaLabelNumWorker(resourceType, common.LABEL_CLOUD_PROVIDER, g+"."+p+"."+a, matcherGPA, ir, metrics.GPAAllNumCloudProviderCount)
				gpaLabelNumWorker(resourceType, common.LABEL_CLUSTER, g+"."+p+"."+a, matcherGPA, ir, metrics.GPAAllNumClusterCount)
				gpaLabelNumWorker(resourceType, common.LABEL_INSTANCE_TYPE, g+"."+p+"."+a, matcherGPA, ir, metrics.GPAAllNumInstanceTypeCount)

				if resourceType == common.RESOURCE_HOST {
					// g
					hostSpecial(resourceType, common.LABEL_CPU, g, matcherG, ir, metrics.GPAHostCpuCores)
					hostSpecial(resourceType, common.LABEL_MEM, g, matcherG, ir, metrics.GPAHostMemGbs)
					hostSpecial(resourceType, common.LABEL_DISK, g, matcherG, ir, metrics.GPAHostDiskGbs)
					// g.p
					hostSpecial(resourceType, common.LABEL_CPU, g+"."+p, matcherGP, ir, metrics.GPAHostCpuCores)
					hostSpecial(resourceType, common.LABEL_MEM, g+"."+p, matcherGP, ir, metrics.GPAHostMemGbs)
					hostSpecial(resourceType, common.LABEL_DISK, g+"."+p, matcherGP, ir, metrics.GPAHostDiskGbs)
					// g.p.a
					hostSpecial(resourceType, common.LABEL_CPU, g+"."+p+"."+a, matcherGPA, ir, metrics.GPAHostCpuCores)
					hostSpecial(resourceType, common.LABEL_MEM, g+"."+p+"."+a, matcherGPA, ir, metrics.GPAHostMemGbs)
					hostSpecial(resourceType, common.LABEL_DISK, g+"."+p+"."+a, matcherGPA, ir, metrics.GPAHostDiskGbs)
				}
			}
		}()
	}
}

// 通过索引的 GetMatchIdsByIndex 接口获取个数分布
// 每个 g.p.a 在每种资源上的计数统计
func gpaNumWorker(resourceType string, gpaName string, matcher []*common.SingleTagRequest, ms *prometheus.GaugeVec) {
	req := common.ResourceQueryRequest{
		ResourceType: resourceType,
		Labels:       matcher,
	}
	matchIds := indexer.GetMatchIdsByIndex(req)
	if len(matchIds) > 0 {
		ms.With(prometheus.Labels{
			common.LABEL_GPA_NAME:      gpaName,
			common.LABEL_RESOURCE_TYPE: resourceType,
		}).Set(float64(len(matchIds)))
	}
}

func gpaLabelNumWorker(resourceType string, targetLabel string, gpaName string, matcher []*common.SingleTagRequest, ir indexer.ResourceIndexer, ms *prometheus.GaugeVec) {
	req := common.ResourceQueryRequest{
		ResourceType: resourceType,
		Labels:       matcher,
		TargetLabel:  targetLabel,
	}
	matchIds := indexer.GetMatchIdsByIndex(req)
	statsRs := ir.GetIndexReader().GetGroupDistributionByLabel(req.TargetLabel, matchIds)
	for _, x := range statsRs.Group {
		ms.With(prometheus.Labels{
			common.LABEL_GPA_NAME:      gpaName,
			common.LABEL_RESOURCE_TYPE: resourceType,
			targetLabel:                x.Name,
		}).Set(float64(x.Value))
	}
}

func hostSpecial(resourceType string, targetLabel string, gpaName string, matcher []*common.SingleTagRequest,
	ir indexer.ResourceIndexer, ms *prometheus.GaugeVec) {

	req := common.ResourceQueryRequest{
		ResourceType: resourceType,
		Labels:       matcher,
		TargetLabel:  targetLabel,
	}

	matchIds := indexer.GetMatchIdsByIndex(req)
	statsRe := ir.GetIndexReader().GetGroupDistributionByLabel(targetLabel, matchIds)
	var all uint64
	for _, x := range statsRe.Group {
		num, _ := strconv.Atoi(x.Name)
		all += uint64(num) * x.Value
	}
	if all > 0 {
		ms.With(prometheus.Labels{common.LABEL_GPA_NAME: gpaName}).Set(float64(all))
	}
}
