package dto

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	MaxRunner      *int    `json:"maxRunner,omitempty" example:"3"`                 // 最大并发下载数
	Proxy          *string `json:"proxy,omitempty" example:"http://proxy.com:8080"` // 代理服务器地址
	LocalDir       *string `json:"localDir,omitempty" example:"/downloads"`         // 全局本地保存目录
	DeleteSegments *bool   `json:"deleteSegments,omitempty" example:"true"`         // 是否删除分段文件
}

// UpdateConfigResponse 更新配置响应
type UpdateConfigResponse struct {
	Message string `json:"message" example:"Config updated"` // 响应消息
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error" example:"invalid request"` // 错误信息
}
