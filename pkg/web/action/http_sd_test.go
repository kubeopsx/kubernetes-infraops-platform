package action

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
)

func TestPrometheusHTTPServiceDiscovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := prometheusHostLoader
	t.Cleanup(func() { prometheusHostLoader = original })

	var gotFilter model.PrometheusHostFilter
	prometheusHostLoader = func(filter model.PrometheusHostFilter) ([]model.ResourceHost, error) {
		gotFilter = filter
		return []model.ResourceHost{{
			Id:           1,
			Name:         "node-a",
			PrivateIps:   json.RawMessage(`["10.0.0.1"]`),
			Tags:         json.RawMessage(`{}`),
			StreeGroup:   "platform",
			StreeProduct: "monitoring",
			StreeApp:     "prometheus",
		}}, nil
	}

	router := gin.New()
	router.GET("/http-sd", PrometheusHTTPServiceDiscovery)
	request := httptest.NewRequest(http.MethodGet, "/http-sd?service_tree=platform.monitoring.prometheus&port=9200", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	wantFilter := model.PrometheusHostFilter{Group: "platform", Product: "monitoring", App: "prometheus"}
	if gotFilter != wantFilter {
		t.Fatalf("filter = %#v, want %#v", gotFilter, wantFilter)
	}
	var body []struct {
		Targets []string          `json:"targets"`
		Labels  map[string]string `json:"labels"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || len(body[0].Targets) != 1 || body[0].Targets[0] != "10.0.0.1:9200" {
		t.Fatalf("body = %#v", body)
	}
	if got := body[0].Labels["service_tree"]; got != "platform.monitoring.prometheus" {
		t.Fatalf("service_tree = %q", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestPrometheusHTTPServiceDiscoveryRejectsInvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/http-sd", PrometheusHTTPServiceDiscovery)
	for _, target := range []string{
		"/http-sd?service_tree=missing.parts",
		"/http-sd?service_tree=a.b.c&group=a",
		"/http-sd?network=unknown",
		"/http-sd?port=70000",
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, body = %s", target, response.Code, response.Body.String())
		}
	}
}
