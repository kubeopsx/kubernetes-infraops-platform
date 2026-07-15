package action

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"os"
	"strings"
)

func ResourceMount(c *gin.Context) {
	var input common.ResourceMountRequest
	if err := c.BindJSON(&input); err != nil {
		common.JsonResp(c, 400, err)
		return
	}
	v, exist := c.Get("logger")
	if !exist {
		v = log.NewLogfmtLogger(os.Stdout)
	}
	logger := v.(log.Logger)
	ok := model.CheckResources(input.ResourceType)
	if !ok {
		common.JsonResp(c, 400, fmt.Errorf("resource node exist:%v", input.ResourceType))
		return
	}
	if err := model.CheckPathExists(input.TargetPath, logger); err != nil {
		common.JsonResp(c, 400, fmt.Errorf("target path not exist:%v", input.TargetPath))
		return
	}
	row, err := model.ResourceMount(&input, logger)
	if err != nil {
		common.JsonResp(c, 500, err)
		return
	}
	common.JsonResp(c, 200, fmt.Sprintf("row:%d", row))
	return
}

func ResourceUnMount(c *gin.Context) {
	var inputs common.ResourceMountRequest
	if err := c.BindJSON(&inputs); err != nil {
		common.JsonResp(c, 400, err)
		return
	}
	v, exists := c.Get("logger")
	if !exists {
		v = log.NewLogfmtLogger(os.Stdout)
	}
	logger := v.(log.Logger)

	ok := model.CheckResources(inputs.ResourceType)
	if !ok {
		common.JsonResp(c, 400, fmt.Errorf("resource node exist:%v", inputs.ResourceType))
		return
	}

	// 验证并确保路径存在
	gpa := strings.Split(inputs.TargetPath, ".")
	if len(gpa) < 3 {
		common.JsonResp(c, 400, fmt.Errorf("invalid target path: %s", inputs.TargetPath))
		return
	}
	g, p, a := gpa[0], gpa[1], gpa[2]

	// 查询第一级节点
	nodeG := &model.Stree{
		Level:    1,
		Path:     "0",
		NodeName: g,
	}
	dbG, err := nodeG.Get()
	if err != nil || dbG == nil {
		common.JsonResp(c, 400, fmt.Errorf("group not exist: %s", g))
		return
	}

	// 查询第二级节点
	pathP := fmt.Sprintf("/%d", dbG.Id)
	nodeP := &model.Stree{
		Level:    2,
		Path:     pathP,
		NodeName: p,
	}
	dbP, err := nodeP.Get()
	if err != nil || dbP == nil {
		common.JsonResp(c, 400, fmt.Errorf("product not exist: %s", p))
		return
	}

	// 查询第三级节点
	pathA := fmt.Sprintf("%s/%d", dbP.Path, dbP.Id)
	nodeA := &model.Stree{
		Level:    3,
		Path:     pathA,
		NodeName: a,
	}
	dbA, err := nodeA.Get()
	if err != nil || dbA == nil {
		common.JsonResp(c, 400, fmt.Errorf("app not exist: %s", a))
		return
	}

	// 资源卸载操作
	row, err := model.ResourceUnMount(&inputs, logger)
	if err != nil {
		common.JsonResp(c, 500, err)
		return
	}
	common.JsonResp(c, 200, fmt.Sprintf("row:%d", row))
	return
}
