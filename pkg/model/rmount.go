package model

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/db"
	"strings"
)

var availbleResources = map[string]struct{}{
	"resource_host": {},
}

func CheckResources(resource string) bool {
	_, ok := availbleResources[resource]
	return ok
}

func ResourceMount(req *common.ResourceMountRequest, logger log.Logger) (int64, error) {
	gpa := strings.Split(req.TargetPath, ".")
	if len(gpa) != 3 || gpa[0] == "" || gpa[1] == "" || gpa[2] == "" {
		return 0, fmt.Errorf("invalid target path %s", req.TargetPath)
	}
	if len(req.ResourceIds) == 0 {
		return 0, fmt.Errorf("no resource IDs provided")
	}
	g, p, a := gpa[0], gpa[1], gpa[2]
	placeholders := make([]string, 0, len(req.ResourceIds))
	args := []interface{}{g, p, a}
	for _, id := range req.ResourceIds {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	raw := fmt.Sprintf(`UPDATE %s SET stree_group=?, stree_product=?, stree_app=? WHERE id IN (%s)`,
		req.ResourceType, strings.Join(placeholders, ","))
	level.Info(logger).Log("msg", "resource mount", "resource_type", req.ResourceType, "resource_count", len(req.ResourceIds), "g.p.a", req.TargetPath)
	execArgs := append([]interface{}{raw}, args...)
	res, err := db.Database["stree"].Exec(execArgs...)
	if err != nil {
		return 0, err
	}
	row, err := res.RowsAffected()
	return row, err
}

func ResourceUnMount(req *common.ResourceMountRequest, logger log.Logger) (int64, error) {
	ids := ""
	for _, id := range req.ResourceIds {
		ids += fmt.Sprintf("%d,", id)
	}
	ids = strings.TrimRight(ids, ",")
	if ids == "" {
		return 0, fmt.Errorf("no resource IDs provided")
	}

	// 构建 SQL 语句，清空 stree_group、stree_product 和 stree_app 字段
	raw := fmt.Sprintf(`UPDATE %s SET stree_group='', stree_product='', stree_app='' WHERE id IN (%s)`,
		req.ResourceType,
		ids,
	)
	level.Info(logger).Log("msg", "resource unmount sql", "raw", raw, "g.p.a", req.TargetPath)

	// 执行 SQL 更新
	res, err := db.Database["stree"].Exec(raw)
	if err != nil {
		return 0, err
	}
	row, err := res.RowsAffected()
	return row, err
}
