package action

import (
	"github.com/gin-gonic/gin"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
)

func LogAdd(c *gin.Context) {
	var input model.LogStrategy
	if err := c.BindJSON(&input); err != nil {
		common.JsonResp(c, 400, err)
		return
	}
	id, err := input.Add()
	if err != nil {
		common.JsonResp(c, 500, err)
		return
	}
	common.JsonResp(c, 200, id)
}

func LogGet(c *gin.Context) {
	l, err := model.Gets("id>0")
	if err != nil {
		common.JsonResp(c, 500, err)
		return
	}
	common.JsonResp(c, 200, l)
}
