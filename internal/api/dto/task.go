package dto

import "caorushizi.cn/mediago/internal/core"

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	ID             string   `json:"id" example:"task-1"`                                  // 任务ID（可选，不提供时自动生成）
	Type           string   `json:"type" binding:"required,oneof=m3u8 bilibili direct"`   // 下载类型：m3u8/bilibili/direct
	URL            string   `json:"url" binding:"required" example:"https://example.com"` // 下载URL
	LocalDir       string   `json:"localDir" binding:"required" example:"/downloads"`     // 本地保存目录
	Name           string   `json:"name" binding:"required" example:"video.mp4"`          // 文件名
	DeleteSegments bool     `json:"deleteSegments" example:"true"`                        // 是否删除分段文件
	Headers        []string `json:"headers" example:"User-Agent: Mozilla/5.0"`            // 自定义HTTP头
	Proxy          string   `json:"proxy" example:"http://proxy.com:8080"`                // 代理地址
	Folder         string   `json:"folder" example:"movies"`                              // 子文件夹
}

// ToDownloadParams converts request payload to core download params.
func (r CreateTaskRequest) ToDownloadParams() core.DownloadParams {
	return core.DownloadParams{
		ID:             core.TaskID(r.ID),
		Type:           core.DownloadType(r.Type),
		URL:            r.URL,
		LocalDir:       r.LocalDir,
		Name:           r.Name,
		DeleteSegments: r.DeleteSegments,
		Headers:        append([]string(nil), r.Headers...),
		Proxy:          r.Proxy,
		Folder:         r.Folder,
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
