package action

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/promsd"
)

var prometheusHostLoader = model.GetPrometheusHosts

func PrometheusHTTPServiceDiscovery(c *gin.Context) {
	filter, options, err := parseHTTPServiceDiscoveryQuery(c)
	if err != nil {
		common.JsonResp(c, http.StatusBadRequest, err)
		return
	}
	hosts, err := prometheusHostLoader(filter)
	if err != nil {
		common.JsonResp(c, http.StatusInternalServerError, err)
		return
	}

	groups, warnings := promsd.Build(hosts, options)
	if logger, ok := c.Get("logger"); ok {
		for _, warning := range warnings {
			level.Warn(logger.(log.Logger)).Log("msg", "prometheus http-sd target skipped", "err", warning)
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, groups)
}

func parseHTTPServiceDiscoveryQuery(c *gin.Context) (model.PrometheusHostFilter, promsd.Options, error) {
	filter := model.PrometheusHostFilter{
		Group:   strings.TrimSpace(c.Query("group")),
		Product: strings.TrimSpace(c.Query("product")),
		App:     strings.TrimSpace(c.Query("app")),
		Region:  strings.TrimSpace(c.Query("region")),
		Status:  strings.TrimSpace(c.Query("status")),
	}
	if path := strings.TrimSpace(c.Query("service_tree")); path != "" {
		parts := strings.Split(path, ".")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return filter, promsd.Options{}, fmt.Errorf("service_tree must use group.product.app")
		}
		if filter.Group != "" || filter.Product != "" || filter.App != "" {
			return filter, promsd.Options{}, fmt.Errorf("service_tree cannot be combined with group, product, or app")
		}
		filter.Group, filter.Product, filter.App = parts[0], parts[1], parts[2]
	}

	options := promsd.Options{
		Job:     strings.TrimSpace(c.DefaultQuery("job", promsd.DefaultJob)),
		Network: strings.TrimSpace(c.DefaultQuery("network", promsd.DefaultNetwork)),
		Port:    promsd.DefaultPort,
	}
	if options.Network != "private" && options.Network != "public" && options.Network != "all" {
		return filter, options, fmt.Errorf("network must be private, public, or all")
	}
	if value := strings.TrimSpace(c.Query("port")); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return filter, options, fmt.Errorf("port must be an integer between 1 and 65535")
		}
		options.Port = port
	}
	return filter, options, nil
}
