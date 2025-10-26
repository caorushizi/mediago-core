package dto

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	MaxRunner      *int    `json:"maxRunner,omitempty"`      // 最大并发下载数
	LocalDir       *string `json:"localDir,omitempty"`       // 本地保存目录
	DeleteSegments *bool   `json:"deleteSegments,omitempty"` // 是否删除分段
	Proxy          *string `json:"proxy,omitempty"`          // 代理地址
	UseProxy       *bool   `json:"useProxy,omitempty"`       // 是否使用代理
}

// UpdateConfigResponse 更新配置响应
type UpdateConfigResponse struct {
	Message string `json:"message" example:"Config updated"` // 响应消息
}
