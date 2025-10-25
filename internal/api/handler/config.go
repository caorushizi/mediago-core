package handler

import (
	"net/http"

	"caorushizi.cn/mediago/internal/api/dto"
	"caorushizi.cn/mediago/internal/core"
	"caorushizi.cn/mediago/internal/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ConfigHandler 处理配置相关接口。
type ConfigHandler struct {
	queue *core.TaskQueue
}

// NewConfigHandler 创建 ConfigHandler。
func NewConfigHandler(queue *core.TaskQueue) *ConfigHandler {
	return &ConfigHandler{queue: queue}
}

// Update 更新系统配置
// @Summary 更新系统配置
// @Description 更新系统配置，包括最大并发下载数和代理设置
// @Tags Config
// @Accept json
// @Produce json
// @Param config body dto.UpdateConfigRequest true "配置参数"
// @Success 200 {object} dto.UpdateConfigResponse "配置更新成功"
// @Failure 400 {object} dto.ErrorResponse "请求参数错误"
// @Router /config [post]
func (h *ConfigHandler) Update(c *gin.Context) {
	var req dto.UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("Invalid config update request",
			zap.String("clientIP", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	logger.Info("Config update request received",
		zap.Int("maxRunner", req.MaxRunner),
		zap.String("proxy", req.Proxy),
		zap.String("clientIP", c.ClientIP()))

	if req.MaxRunner > 0 {
		h.queue.SetMaxRunner(req.MaxRunner)
		logger.Info("Max runner updated", zap.Int("maxRunner", req.MaxRunner))
	}

	if req.Proxy != "" {
		h.queue.SetProxy(req.Proxy)
		logger.Info("Proxy updated", zap.String("proxy", req.Proxy))
	}

	c.JSON(http.StatusOK, dto.UpdateConfigResponse{Message: "Config updated"})
}
