package model

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/db"
	"sort"
	"strings"
)

// Stree = infraops.stree
// ServiceTree = infraops.service_tree
type Stree struct {
	Id       int64  `json:"id"`
	Level    int64  `json:"level"`
	Path     string `json:"path"`
	NodeName string `json:"node_name"`
}

func (obj *Stree) Get() (*Stree, error) {
	ok, err := db.Database["stree"].Get(obj)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	return obj, nil
}

func (obj *Stree) Add() (int64, error) {
	ins, err := db.Database["stree"].InsertOne(obj)
	return ins, err
}

func (obj *Stree) GetOrAdd() (*Stree, error) {
	exist, err := obj.Get()
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return exist, nil
	}
	_, err = db.Database["stree"].Insert(obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (obj *Stree) Delete() (int64, error) {
	del, err := db.Database["stree"].Delete(obj)
	return del, err
}

func Get(where string, args ...interface{}) (*Stree, error) {
	var obj Stree
	ok, err := db.Database["stree"].Where(where, args...).Get(&obj)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &obj, nil
}

func GetMultiple(where string, args ...interface{}) ([]Stree, error) {
	var obj []Stree
	err := db.Database["stree"].Where(where, args...).Find(&obj)
	if err != nil {
		return obj, err
	}
	return obj, nil
}

func DeleteMultiple(where string) (int64, error) {
	raw := fmt.Sprintf(`delete from stree where %s`, where)
	result, err := db.Database["stree"].Exec(raw)
	if err != nil {
		return 0, err
	}
	row, err := result.RowsAffected()
	return row, err
}

func StreeAdd(req *common.NodeRequest, logger log.Logger) error {
	// g.p.a 三段式
	result := strings.Split(req.Node, ".")
	if len(result) != 3 {
		level.Info(logger).Log("msg", "add path invalidate", "path", req.Node)
		return nil
	}
	g, p, a := result[0], result[1], result[2]
	// 先查询 g
	nodeG := &Stree{
		Level:    1,
		Path:     "0",
		NodeName: g,
	}
	dbG, err := nodeG.Get()
	if err != nil {
		level.Error(logger).Log("msg", "add g failed", "path", req.Node, "err", err)
		return err
	}
	level.Info(logger).Log("msg", "add g success", "path", req.Node)

	// 根据 g 查询结果再判断
	switch dbG {
	case nil:
		// g 不存在，依次插入 g.p.a
		// 插入 g
		_, err := nodeG.Add()
		if err != nil {
			level.Error(logger).Log("msg", "add g failed; g not exist", "path", req.Node, "err", err)
			return err
		}
		level.Info(logger).Log("msg", "add g success", "path", req.Node)

		// 插入 p
		pathP := fmt.Sprintf("/%d", nodeG.Id)
		nodeP := &Stree{
			Level:    2,
			Path:     pathP,
			NodeName: p,
		}
		_, err = nodeP.Add()
		if err != nil {
			level.Error(logger).Log("msg", "add g failed; g not exist", "path", req.Node, "err", err)
			return err
		}
		level.Info(logger).Log("msg", "add g success", "path", req.Node)

		// 插入 a
		pathA := fmt.Sprintf("%s/%d", pathP, nodeP.Id)
		nodeA := &Stree{
			Level:    3,
			Path:     pathA,
			NodeName: a,
		}
		_, err = nodeA.Add()
		if err != nil {
			level.Error(logger).Log("msg", "add g failed; g not exist", "path", req.Node, "err", err)
			return err
		}
		level.Info(logger).Log("msg", "add g success", "path", req.Node)

	default:
		// 说明 g 存在，再查询 p
		pathP := fmt.Sprintf("/%d", dbG.Id)
		nodeP := &Stree{
			Level:    2,
			Path:     pathP,
			NodeName: p,
		}
		dbP, err := nodeP.Get()
		if err != nil {
			level.Error(logger).Log("msg", "add p failed; p not exist", "path", req.Node, "err", err)
			return err
		}
		if dbP != nil {
			// 说明 p 存在，继续查询 a
			PathA := fmt.Sprintf("%s/%d", pathP, dbP.Id)
			nodeA := &Stree{
				Level:    3,
				Path:     PathA,
				NodeName: a,
			}
			dbA, err := nodeA.Get()
			if err != nil {
				level.Error(logger).Log("msg", "add a failed; g_p exist", "path", req.Node, "err", err)
				return err
			}
			if dbA == nil {
				_, err := nodeA.Add()
				if err != nil {
					level.Error(logger).Log("msg", "add a failed; g_p exist", "path", req.Node, "err", err)
					return err
				}
				level.Info(logger).Log("msg", "g_p_a", "path", req.Node)
				return nil
			}
			_, err = nodeP.Add()
			if err != nil {
				err = fmt.Errorf("add p failed; g exist %v", req.Node)
				return err
			}
			level.Info(logger).Log("msg", "add p success", "path", req.Node)
		}

		// 说明 p 不存在，插入 p 和 a
		_, err = nodeP.Add()
		if err != nil {
			level.Error(logger).Log("msg", "add p failed; g exist", "path", req.Node, "err", err)
			return err
		}
		level.Info(logger).Log("msg", "add p success", "path", req.Node)

		// 插入 a
		PathA := fmt.Sprintf("%s/%d", pathP, nodeP.Id)
		nodeA := &Stree{
			Level:    3,
			Path:     PathA,
			NodeName: a,
		}
		_, err = nodeA.Add()
		if err != nil {
			level.Error(logger).Log("msg", "add p failed; g exist", "path", req.Node, "err", err)
			return err
		}
		level.Info(logger).Log("msg", "add p success", "path", req.Node)
	}
	return nil
}

func StreeQuery(req *common.NodeRequest, logger log.Logger) (result []string) {
	switch req.QueryType {
	case 1:
		// 根据 g 查询，所有 p 的列表 node=g query_type=1
		nodeG := &Stree{
			Level:    1,
			Path:     "0",
			NodeName: req.Node,
		}
		dbG, err := nodeG.Get()
		if err != nil {
			level.Error(logger).Log("msg", "query g failed", "path", req.Node, "err", err)
			return
		}
		// g 查询不存在
		if dbG == nil {
			return
		}
		pathP := fmt.Sprintf("/%d", dbG.Id)
		whereStr := "level=? and path=?"
		// 获取多个 p
		ps, err := GetMultiple(whereStr, 2, pathP)
		if err != nil {
			level.Error(logger).Log("msg", "query a failed", "path", req.Node)
			return
		}
		for _, i := range ps {
			result = append(result, i.NodeName)
		}
		sort.Strings(result)
		return
	case 2:
		// query_type=2 的查询，根据 g 查询所有g.p.a的列表
		// 先查g，再查p，最后查a，中间有一步没有返回空

		// 根据 g 查询，所有 p 的列表 node=g query_type=1
		nodeG := &Stree{
			Level:    1,
			Path:     "0",
			NodeName: req.Node,
		}
		dbG, err := nodeG.Get()
		if err != nil {
			level.Error(logger).Log("msg", "query g failed", "path", req.Node, "err", err)
			return
		}
		if dbG == nil {
			// 查询 g 不存在
			return
		}
		pathP := fmt.Sprintf("/%d", dbG.Id)
		whereStr := "level=? and path=?"
		// 获取多个 p
		ps, err := GetMultiple(whereStr, 2, pathP)
		if err != nil {
			level.Info(logger).Log("msg", "query ps failed", "path", req.Node, "err", err)
			return
		}
		if len(ps) == 0 {
			// g 下面没有 p
			return
		}
		for _, p := range ps {
			pathA := fmt.Sprintf("%s/%d", p.Path, p.Id)
			// 获取多个 a
			as, err := GetMultiple(whereStr, 3, pathA)
			if err != nil {
				level.Error(logger).Log("msg", "query as failed", "path", req.Node, "err", err)
				continue
			}
			if len(as) == 0 {
				// p 下面没有 a
				continue
			}
			for _, a := range as {
				fullPath := fmt.Sprintf("%s.%s.%s", dbG.NodeName, p.NodeName, a.NodeName)
				result = append(result, fullPath)
			}
		}
		sort.Strings(result)
		return
	case 3:
		// query_type=3 的查询，根据 g.p 查询，所有 g.p.a 的列表 node=g.p query_type=3
		// 查询 g 和 p 不存在直接返回空
		// 查 p 时需要带上 p.name 查询
		gps := strings.Split(req.Node, ".")
		if len(gps) != 2 {
			return
		}
		g, p := gps[0], gps[1]
		nodeG := &Stree{
			Level:    1,
			Path:     "0",
			NodeName: g,
		}
		dbG, err := nodeG.Get()
		if err != nil {
			level.Error(logger).Log("msg", "query g failed", "path", req.Node, "err", err)
			return
		}
		if dbG == nil {
			// 查询 g 不存在
			return
		}
		// p 存在，这里不需要查全量 p，只查询匹配 node_name 的 p
		pathP := fmt.Sprintf("/%d", dbG.Id)
		whereStr := "level=? and path=? and node_name=?"
		dbP, err := Get(whereStr, 2, pathP, p)
		if err != nil {
			level.Error(logger).Log("msg", "query p failed", "path", req.Node, "err", err)
			return
		}
		if dbP == nil {
			// p 不存在
			return
		}
		pathA := fmt.Sprintf("%s/%d", pathP, dbP.Id)
		whereStr = "level=? and path=?"
		as, err := GetMultiple(whereStr, 3, pathA)
		if err != nil {
			level.Error(logger).Log("msg", "query as failed", "path", req.Node, "err", err)
			return
		}
		for _, a := range as {
			fullPath := fmt.Sprintf("%s.%s.%s", dbG.NodeName, dbP.NodeName, a.NodeName)
			result = append(result, fullPath)
		}
		sort.Strings(result)
		return
	case 4:
		// 直接查询g.p.a是否存在
		gpas := strings.Split(req.Node, ".")
		g, p, a := gpas[0], gpas[1], gpas[2]
		nodeG := &Stree{
			Level:    1,
			Path:     "0",
			NodeName: g,
		}
		dbG, err := nodeG.Get()
		if err != nil {
			level.Error(logger).Log("msg", "query g failed", "path", req.Node, "err", err)
			return
		}
		if dbG == nil {
			return
		}
		pathP := fmt.Sprintf("/%d", dbG.Id)
		nodeP := &Stree{
			Level:    2,
			Path:     pathP,
			NodeName: p,
		}
		dbP, err := nodeP.Get()
		if err != nil {
			level.Error(logger).Log("msg", "query g failed", "path", req.Node, "err", err)
			return
		}
		if dbP == nil {
			return
		}
		pathA := fmt.Sprintf("%s/%d", dbP.Path, dbP.Id)
		nodeA := &Stree{
			Level:    3,
			Path:     pathA,
			NodeName: a,
		}
		dbA, err := nodeA.Get()
		if err != nil {
			level.Error(logger).Log("msg", "query a failed", "path", req.Node, "err", err)
			return
		}
		if dbA == nil {
			return
		}
		result = append(result, req.Node)
		return
	case 5:
		// 获得全量g.p.a 给统计用的
		whereStr := "id>0"
		ps, err := GetMultiple(whereStr)
		if err != nil {
			return
		}

		existMapGs := make(map[int64]Stree)
		existMapPs := make(map[int64]Stree)
		existMapAs := make(map[int64]Stree)

		for _, p := range ps {
			switch p.Level {
			case 1:
				existMapGs[p.Id] = p
			case 2:
				existMapPs[p.Id] = p
			case 3:
				existMapAs[p.Id] = p
			}
		}

		for gid, g := range existMapGs {
			for pid, p := range existMapPs {
				pathP := fmt.Sprintf("/%d", gid)
				if pathP == p.Path {
					for _, a := range existMapAs {
						pathA := fmt.Sprintf("%s/%d", p.Path, pid)
						if pathA == a.Path {
							result = append(result, fmt.Sprintf("%s.%s.%s", g.NodeName, p.NodeName, a.NodeName))
						}
					}
				}
			}
		}
	}
	return
}

func StreeDelete(req *common.NodeRequest, logger log.Logger) (del int64) {
	path := strings.Split(req.Node, ".")
	levelP := len(path)
	nodeG := &Stree{
		Level:    1,
		Path:     "0",
		NodeName: path[0],
	}
	dbG, err := nodeG.Get()
	if err != nil {
		level.Error(logger).Log("msg", "query g failed", "path", req.Node, "err", err)
		return
	}
	if dbG == nil {
		return
	}
	pathP := fmt.Sprintf("/%d", dbG.Id)
	// 传入的参数为服务标识，如果下一级子节点还有数据不让删
	// g 传入 g，如果 g 下有 p 就不让删 g
	// g.p 传入 g.p，如果 p 下有 a 就不让删 p
	// g.p.a 传入 g.p.a，直接删
	switch levelP {
	// 传入 g，如果 g 下有 p 就不让删 g
	case 1:
		// ForceDelete 暴力强制删除
		// 强制删除 g 的时候，分两步
		// - 第一步 删除 path 前缀的 p 和 a del where="path like '/1/%' and level in(2,3)'
		// - 第二步 删除这个 g
		// if req.ForceDelete {
		// 	whereStrDelA := fmt.Sprintf(`path like '/%d/%%' and level=3`, dbG.Id)
		// 	delA, err := DeleteMultiple(whereStrDelA)
		// 	if err != nil {
		// 		level.Error(logger).Log("del pa failed", "path", req.Node, "err", err)
		// 		return
		// 	}
		// 	level.Info(logger).Log("msg", "del as success", "path", req.Node, "num", delA, "del_where", whereStrDelA)
		// 	del += delA
		// 	whereStrDelP := fmt.Sprintf(`path='%d' and level=2 `, dbG.Id)
		// 	delP, err := DeleteMultiple(whereStrDelP)
		// 	if err != nil {
		// 		level.Error(logger).Log("del pa failed", "path", req.Node, "err", err)
		// 		return
		// 	}
		// 	level.Info(logger).Log("msg", "del ps success", "path", req.Node, "num", delP, "del_where", whereStrDelP)
		// 	del += delP
		//
		// 	_, err = dbG.StreeDelete()
		// 	if err != nil {
		// 		level.Error(logger).Log("msg", "del g failed", "path", req.Node, "err", err)
		// 		return
		// 	}
		// 	level.Info(logger).Log("msg", "del g success", "path", req.Node)
		// 	del += 1
		// 	return
		// }

		whereStr := "level=? and path=?"
		ps, err := GetMultiple(whereStr, 2, pathP)
		if err != nil {
			level.Error(logger).Log("query ps failed", "path", req.Node, "err", err)
			return
		}
		if len(ps) > 0 {
			level.Warn(logger).Log("msg", "del g reject", "path", req.Node, "reason", "g has ps", "ps_num", len(ps))
			return
		}
		del, err = dbG.Delete()
		if err != nil {
			level.Error(logger).Log("msg", "del g failed", "path", req.Node, "err", err)
			return
		}
		level.Info(logger).Log("msg", "del g success", "path", req.Node)
	// 传入 g.p，如果 p 下有 a 就不让删 p
	case 2:
		nodeP := &Stree{
			Level:    2,
			Path:     pathP,
			NodeName: path[1],
		}
		dbP, err := nodeP.Get()
		if err != nil {
			level.Error(logger).Log("msg", "query p failed", "path", req.Node, "err", err)
			return
		}
		if dbP == nil {
			// p 不存在
			return
		}
		pathA := fmt.Sprintf("%s/%d", dbP.Path, dbP.Id)
		whereStr := "level=? and path=?"
		as, err := GetMultiple(whereStr, 3, pathA)
		if err != nil {
			level.Error(logger).Log("msg", "query as failed", "path", req.Node, "err", err)
			return
		}
		if len(as) > 0 {
			level.Warn(logger).Log("msg", "del g p reject", "path", req.Node, "reason", "p has as", "as_num", len(as))
			return
		}
		del, err = dbP.Delete()
		if err != nil {
			level.Error(logger).Log("msg", "del p failed", "path", req.Node, "err", err)
			return
		}
		level.Info(logger).Log("msg", "del p success", "path", req.Node)
	case 3:
		nodeP := &Stree{
			Level:    2,
			Path:     pathP,
			NodeName: path[1],
		}
		dbP, err := nodeP.Get()
		if err != nil {
			level.Error(logger).Log("msg", "query p failed", "path", req.Node, "err", err)
			return
		}
		if dbP == nil {
			return
		}
		pathA := fmt.Sprintf("%s/%d", dbP.Path, dbP.Id)
		whereStr := "level=? and path=? and node_name=?"
		dbA, err := Get(whereStr, 3, pathA, path[2])
		if err != nil {
			level.Error(logger).Log("msg", "query a failed", "path", req.Node, "err", err)
			return
		}
		if dbA == nil {
			return
		}
		del, err = dbA.Delete()
		if err != nil {
			level.Error(logger).Log("msg", "del a failed", "path", req.Node, "err", err)
			return
		}
		level.Info(logger).Log("msg", "del a success", "path", req.Node)
	}
	return
}

func CheckPathExists(targetPath string, logger log.Logger) error {
	gpa := strings.Split(targetPath, ".")
	if len(gpa) < 3 {
		return fmt.Errorf("invalid target path: %s", targetPath)
	}
	g, p, a := gpa[0], gpa[1], gpa[2]

	nodeG := &Stree{
		Level:    1,
		Path:     "0",
		NodeName: g,
	}
	dbG, err := nodeG.GetOrInsert()
	if err != nil {
		level.Error(logger).Log("msg", "create g failed", "path", targetPath, "err", err)
		return err
	}

	pathP := fmt.Sprintf("/%d", dbG.Id)
	nodeP := &Stree{
		Level:    2,
		Path:     pathP,
		NodeName: p,
	}
	dbP, err := nodeP.GetOrInsert()
	if err != nil {
		level.Error(logger).Log("msg", "create p failed", "path", targetPath, "err", err)
		return err
	}

	pathA := fmt.Sprintf("%s/%d", dbP.Path, dbP.Id)
	nodeA := &Stree{
		Level:    3,
		Path:     pathA,
		NodeName: a,
	}
	_, err = nodeA.GetOrInsert()
	if err != nil {
		level.Error(logger).Log("msg", "create a failed", "path", targetPath, "err", err)
		return err
	}
	return nil
}

func (obj *Stree) GetOrInsert() (*Stree, error) {
	exist, err := obj.Get()
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return exist, nil
	}
	_, err = db.Database["stree"].Insert(obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func AddTest(logger log.Logger) {
	ns := []string{
		"infra.monitor.thanos",
		"infra.monitor.kafka",
		"infra.monitor.prometheus",
		"infra.monitor.m3db",
		"infra.cicd.tekton",
		"infra.cicd.argocd",
		"infra.cicd.jenkins",
		"waimai.qiangdan.queue",
		"waimai.qiangdan.worker",
		"waimai.qiangdan.es",
		"waimai.ditu.kafka",
		"waimai.ditu.es",
	}
	for _, n := range ns {
		req := &common.NodeRequest{
			Node: n,
		}
		StreeAdd(req, logger)
	}
}

func QueryCase1Test(logger log.Logger) {
	ns := []string{
		"a",
		"b",
		"c",
		"infra",
		"waimai",
	}
	for _, n := range ns {
		req := &common.NodeRequest{
			Node:      n,
			QueryType: 1,
		}
		res := StreeQuery(req, logger)
		level.Info(logger).Log("msg", "query result", "req.node", n, "num", len(res),
			"details", strings.Join(res, ","),
		)
	}
}

func QueryCase2Test(logger log.Logger) {
	ns := []string{
		"a",
		"b",
		"c",
		"infra",
		"waimai",
	}
	for _, n := range ns {
		req := &common.NodeRequest{
			Node:      n,
			QueryType: 2,
		}
		res := StreeQuery(req, logger)
		level.Info(logger).Log("msg", "query  result", "req.node", n, "num", len(res),
			"details", strings.Join(res, ","),
		)
	}
}

func QueryCase3Test(logger log.Logger) {
	ns := []string{
		"a.b",
		"b.a",
		"c.d",
		"infra.cicd",
		"infra.monitor",
		"waimai.ditu",
		"waimai.qiangdan",
	}
	for _, n := range ns {
		req := &common.NodeRequest{
			Node:      n,
			QueryType: 3,
		}
		res := StreeQuery(req, logger)
		level.Info(logger).Log("msg", "query result", "req.node", n, "num", len(res),
			"details", strings.Join(res, ","),
		)
	}
}

func DeleteTest(logger log.Logger) {
	ns := []string{
		"infra.cicd.jenkins",
		"infra.cicd",
		"infra",
	}
	for _, n := range ns {
		req := &common.NodeRequest{
			Node:        n,
			ForceDelete: true,
		}
		res := StreeDelete(req, logger)
		level.Info(logger).Log("msg", "delete result", "req.node", n, "del_num", res)
	}
}

// TODO 修改可以先不支持
// TODO 01 可以通过先删除后增加变向支持
// TODO 02 修改的时候如果使用 nodename 拼接的 path 那么涉及到 nodename 不能修改的问题
