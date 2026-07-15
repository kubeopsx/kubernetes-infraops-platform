package rpc

import (
	"testing"
	"time"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
)

func TestPushResultsAcceptsEveryProtocol(t *testing.T) {
	protocols := []string{"icmp", "http", "tcp", "dns", "tls"}
	results := make([]*common.Result, 0, len(protocols))
	for _, protocol := range protocols {
		results = append(results, &common.Result{
			WorkerName: "agent", MetricsName: protocol + "_test_value", TargetAddress: "target",
			SourceRegion: "source", TargetRegion: "target", Type: protocol, TimeStamp: time.Now().Unix(), Value: 1,
		})
	}
	var response common.ResultPushResponse
	if err := new(Server).PushResults(common.ResultPushRequest{Results: results}, &response); err != nil {
		t.Fatal(err)
	}
	if got := int(response.SuccessNum); got != len(protocols) {
		t.Fatalf("accepted results = %d, want %d", got, len(protocols))
	}
}
