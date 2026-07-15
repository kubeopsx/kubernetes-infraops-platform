package action

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/kubeopsx/kubernetes-infraops-platform/cmd/server/indexer"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/common"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/model"
	"math"
	"strconv"
)

func ResourceQuery(context *gin.Context) {
	var inputs common.ResourceQueryRequest
	if err := context.BindJSON(&inputs); err != nil {
		common.JsonResp(context, 400, err)
		return
	}
	ok := indexer.ResourceIndexExists(inputs.ResourceType)
	if !ok {
		common.JsonResp(context, 400, fmt.Sprintf("Resourse_not_exists:%v", inputs.ResourceType))
		return
	}
	pageSize, err := strconv.Atoi(context.DefaultQuery("page_size", "100"))
	if err != nil {
		common.JsonResp(context, 400, fmt.Sprintf("invalid_page_size"))
		return
	}
	currentPage, err := strconv.Atoi(context.DefaultQuery("current_page", "1"))
	if err != nil {
		common.JsonResp(context, 400, fmt.Errorf(" current_page"))
		return
	}
	offset := 0
	limit := 0
	limit = pageSize
	if currentPage > 1 {
		offset = (currentPage - 1) * limit
	}

	matchIds := indexer.GetMatchIdsByIndex(inputs)
	// TODO 可以先在查询的 view 中写死 match matchIds := []uint64{1, 2, 3} 1，2，3 是 id
	totalCount := len(matchIds)
	logger := context.MustGet("logger").(log.Logger)

	pageCount := int(math.Ceil(float64(totalCount) / float64(limit)))
	resp := common.QueryResponse{
		Code:        200,
		CurrentPage: currentPage,
		PageSize:    pageSize,
		PageCount:   pageCount,
		TotalCount:  totalCount,
	}
	res, err := model.ResourceQuery(inputs.ResourceType, matchIds, logger, limit, offset)
	if err != nil {
		resp.Code = 500
		resp.Result = err
	}
	resp.Result = res
	common.JsonResp(context, resp)
}

func ResourceGroup(context *gin.Context) {
	resourceType := context.DefaultQuery("resource_type", common.RESOURCE_HOST)
	label := context.DefaultQuery("label", "region")
	ok := indexer.ResourceIndexExists(resourceType)
	if !ok {
		common.JsonResp(context, 400, fmt.Errorf("ResourceType not exists:%v", resourceType))
		return
	}
	_, ri := indexer.GetResourceIndexReader(resourceType)
	res := ri.GetIndexReader().GetGroupByLabel(label)
	common.JsonResp(context, res)
}

func ResourceDistribution(context *gin.Context) {
	var inputs common.ResourceQueryRequest
	if err := context.BindJSON(&inputs); err != nil {
		common.JsonResp(context, 400, err)
		return
	}
	ok, ri := indexer.GetResourceIndexReader(inputs.ResourceType)
	if !ok {
		common.JsonResp(context, 400, fmt.Errorf("ResourceType not exist:%v", inputs.ResourceType))
		return
	}
	matchIds := indexer.GetMatchIdsByIndex(inputs)
	res := ri.GetIndexReader().GetGroupDistributionByLabel(inputs.TargetLabel, matchIds)
	common.JsonResp(context, res)
}
