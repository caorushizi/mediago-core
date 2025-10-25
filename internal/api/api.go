package api

import (
	"caorushizi.cn/mediago/internal/api/server"
	"caorushizi.cn/mediago/internal/core"
)

// NewServer 创建 HTTP 服务器实例（向下兼容的入口）。
func NewServer(queue *core.TaskQueue) *server.Server {
	return server.New(queue)
}
