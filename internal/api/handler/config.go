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

// reference types to help swagger parsing
var (
    _ dto.UpdateConfigRequest
    _ dto.UpdateConfigResponse
)

// NewConfigHandler 创建 ConfigHandler。
func NewConfigHandler(queue *core.TaskQueue) *ConfigHandler {
	return &ConfigHandler{queue: queue}
}

type AppConfig interface {
	SetLocalDir(string)
	SetDeleteSegments(bool)
	SetProxy(string)
	SetUseProxy(bool)
}

// Update 更新系统配置
// @Summary 更新系统配置
// @Description 更新系统配置，包括最大并发下载数、下载目录、代理、代理开关等、下载目录、代理等
// @Tags Config
// @Accept json
// @Produce json
// @Param config body dto.UpdateConfigRequest true "配置参数"
// @Success 200 {object} dto.SuccessResponse{data=dto.UpdateConfigResponse} "配置更新成功"
// @Failure 400 {object} dto.ErrorResponse "请求参数错误"
// @Router /config [post]
func (h *ConfigHandler) Update(c *gin.Context) {
	var req dto.UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("Invalid config update request",
			zap.String("clientIP", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Success: false, Code: http.StatusBadRequest, Message: err.Error()})
		return
	}

	logger.Info("Config update request received", zap.Any("req", req), zap.String("clientIP", c.ClientIP()))

	if req.MaxRunner != nil {
		h.queue.SetMaxRunner(*req.MaxRunner)
		logger.Info("Max runner updated", zap.Int("maxRunner", *req.MaxRunner))
	}

	appConfig := h.queue.Downloader().Config().(AppConfig)

	if req.LocalDir != nil {
		appConfig.SetLocalDir(*req.LocalDir)
		logger.Info("Local dir updated", zap.String("localDir", *req.LocalDir))
	}

	if req.DeleteSegments != nil {
		appConfig.SetDeleteSegments(*req.DeleteSegments)
		logger.Info("Delete segments updated", zap.Bool("deleteSegments", *req.DeleteSegments))
	}

	if req.Proxy != nil {
		appConfig.SetProxy(*req.Proxy)
		logger.Info("Proxy updated", zap.String("proxy", *req.Proxy))
	}

	if req.UseProxy != nil {
		appConfig.SetUseProxy(*req.UseProxy)
		logger.Info("Use proxy updated", zap.Bool("useProxy", *req.UseProxy))
	}

	c.JSON(http.StatusOK, dto.SuccessResponse{
		Success: true,
		Code:    http.StatusOK,
		Message: "Config updated",
		Data:    dto.UpdateConfigResponse{Message: "Config updated"},
	})
}
