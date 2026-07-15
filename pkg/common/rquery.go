package common

import "github.com/kubeopsx/kubernetes-infraops-platform/pkg/inverted/labels"

type ResourceQueryRequest struct {
	ResourceType string              `json:"resource_type" binding:"required"`
	Labels       []*SingleTagRequest `json:"labels" binding:"required"`
	TargetLabel  string              `json:"target_label"`
}

type SingleTagRequest struct {
	Key   string `json:"key" binding:"required"`   // 标签名
	Value string `json:"value" binding:"required"` // 标签值
	Type  int    `json:"type" binding:"required"`  // 类型 1-4 = != ~= ~!
}

type QueryResponse struct {
	Code        int         `json:"code"`
	CurrentPage int         `json:"current_page"`
	PageSize    int         `json:"page_size"`
	PageCount   int         `json:"page_count"`
	TotalCount  int         `json:"total_count"`
	Result      interface{} `json:"result"`
}

func FormatLabelMatcher(ls []*SingleTagRequest) []*labels.Matcher {
	matcher := make([]*labels.Matcher, 0)
	for _, i := range ls {
		mType, ok := labels.MatchMap[i.Type]
		if !ok {
			continue
		}
		matcher = append(matcher, labels.MustNewMatcher(mType, i.Key, i.Value))
	}
	return matcher
}
