package dto

import "caorushizi.cn/mediago/internal/core"

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	ID      string   `json:"id" example:"task-1"`                                  // 任务ID（可选，不提供时自动生成）
	Type    string   `json:"type" binding:"required,oneof=m3u8 bilibili direct"`   // 下载类型：m3u8/bilibili/direct
	URL     string   `json:"url" binding:"required" example:"https://example.com"` // 下载URL
	Name    string   `json:"name" binding:"required" example:"video.mp4"`          // 文件名
	Headers []string `json:"headers" example:"User-Agent: Mozilla/5.0"`            // 自定义HTTP头
	Folder  string   `json:"folder" example:"movies"`                              // 子文件夹
}

// ToDownloadParams converts request payload to core download params.
func (r CreateTaskRequest) ToDownloadParams() core.DownloadParams {
	return core.DownloadParams{
		ID:      core.TaskID(r.ID),
		Type:    core.DownloadType(r.Type),
		URL:     r.URL,
		Name:    r.Name,
		Headers: append([]string(nil), r.Headers...),
		Folder:  r.Folder,
	}
}

// CreateTaskResponse 创建任务响应
type CreateTaskResponse struct {
	ID      string `json:"id" example:"task-1"`                          // 任务ID
	Message string `json:"message" example:"Task enqueued successfully"` // 响应消息
}

// TaskListResponse 任务列表响应
type TaskListResponse struct {
	Tasks []core.TaskInfo `json:"tasks"` // 任务列表
	Total int             `json:"total"` // 总数量
}

// StopTaskResponse 停止任务响应
type StopTaskResponse struct {
	Message string `json:"message" example:"Task stopped"` // 响应消息
}
