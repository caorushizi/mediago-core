package dto

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	MaxRunner int    `json:"maxRunner" example:"3"`                 // 最大并发下载数
	Proxy     string `json:"proxy" example:"http://proxy.com:8080"` // 代理服务器地址
}

// UpdateConfigResponse 更新配置响应
type UpdateConfigResponse struct {
	Message string `json:"message" example:"Config updated"` // 响应消息
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error" example:"invalid request"` // 错误信息
}
