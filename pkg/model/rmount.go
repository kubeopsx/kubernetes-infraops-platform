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
	if len(gpa) < 3 {
		return 0, fmt.Errorf("invalid target path %s", req.TargetPath)
	}
	g, p, a := gpa[0], gpa[1], gpa[2]
	ids := ""
	for _, id := range req.ResourceIds {
		ids += fmt.Sprintf("%d", id)
	}
	raw := fmt.Sprintf(`update %s set stree_group='%s', stree_product='%s', stree_app='%s' where id in (%s)`,
		req.ResourceType,
		g,
		p,
		a,
		ids)
	level.Info(logger).Log("msg", "resource mount sql", "raw", raw)
	res, err := db.Database["stree"].Exec(raw)
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
