package common

type ResourceMountRequest struct {
	ResourceType string  `json:"resource_type" binding:"required"` // 资源类型，必须提供
	ResourceIds  []int64 `json:"resource_ids" binding:"required"`  // 资源 ID 列表，必须提供
	TargetPath   string  `json:"target_path" binding:"required"`   // 目标路径，必须提供
}
