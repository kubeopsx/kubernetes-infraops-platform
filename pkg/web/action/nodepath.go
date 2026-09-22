package action

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"strings"
)

func NodePathAdd(c *gin.Context) {
	var input common.NodeRequest
	if err := c.Bind(&input); err != nil {
		common.JsonResp(c, 400, err)
		return
	}
	logger := c.MustGet("logger").(log.Logger)
	result := strings.Split(input.Node, ".")
	if len(result) != 3 {
		common.JsonResp(c, 400, fmt.Errorf("path invalidate:%v", input.Node))
		return
	}
	err := model.StreeAdd(&input, logger)
	if err != nil {
		common.JsonResp(c, 500, err)
		return
	}
	common.JsonResp(c, 200, "path add success")
}

func NodePathQuery(c *gin.Context) {
	var input common.NodeRequest
	if err := c.Bind(&input); err != nil {
		common.JsonResp(c, 400, err)
		return
	}
	logger := c.MustGet("logger").(log.Logger)
	if input.QueryType < 1 || input.QueryType > 5 {
		common.JsonResp(c, 400, fmt.Errorf("query_type must be between 1 and 5"))
		return
	}
	parts := strings.Split(input.Node, ".")
	if input.QueryType == 3 && len(parts) != 2 {
		common.JsonResp(c, 400, fmt.Errorf("path should be a.b:%v", input.Node))
		return
	}
	if input.QueryType == 4 && len(parts) != 3 {
		common.JsonResp(c, 400, fmt.Errorf("path should be a.b.c:%v", input.Node))
		return
	}
	common.JsonResp(c, model.StreeQuery(&input, logger))
}
