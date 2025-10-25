package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler 提供健康检查接口。
type HealthHandler struct{}

// NewHealthHandler 创建 HealthHandler。
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Check 健康检查
// @Summary 健康检查
// @Description 服务健康检查接口，用于监控服务是否正常运行
// @Tags Health
// @Produce plain
// @Success 200 {string} string "ok"
// @Router /healthy [get]
func (h *HealthHandler) Check(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}
