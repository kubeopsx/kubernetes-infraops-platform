package model

import (
	"fmt"
	"strings"

	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/db"
)

type PrometheusHostFilter struct {
	Group   string
	Product string
	App     string
	Region  string
	Status  string
}

func GetPrometheusHosts(filter PrometheusHostFilter) ([]ResourceHost, error) {
	engine, ok := db.Database["stree"]
	if !ok || engine == nil {
		return nil, fmt.Errorf("database stree is not initialized")
	}

	where := []string{"stree_group <> ''", "stree_product <> ''", "stree_app <> ''"}
	var args []interface{}
	for _, item := range []struct {
		column string
		value  string
	}{
		{"stree_group", filter.Group},
		{"stree_product", filter.Product},
		{"stree_app", filter.App},
		{"region", filter.Region},
		{"status", filter.Status},
	} {
		if item.value == "" {
			continue
		}
		where = append(where, item.column+" = ?")
		args = append(args, item.value)
	}

	var hosts []ResourceHost
	err := engine.Where(strings.Join(where, " AND "), args...).
		Asc("stree_group", "stree_product", "stree_app", "name", "id").
		Find(&hosts)
	return hosts, err
}
