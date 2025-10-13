// Package api 包含 HTTP API 实现
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"caorushizi.cn/mediago/internal/core"
	"caorushizi.cn/mediago/internal/logger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// Server HTTP 服务器
type Server struct {
	queue   *core.TaskQueue
	engine  *gin.Engine
	sseHub  *SSEHub
	taskSeq int64
	mu      sync.Mutex
}

// NewServer 创建 HTTP 服务器实例
func NewServer(queue *core.TaskQueue) *Server {
	s := &Server{
		queue:  queue,
		engine: gin.Default(),
		sseHub: NewSSEHub(),
	}

	// 配置 CORS
	s.engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 注册路由
	s.setupRoutes()

	// 注册任务队列事件回调
	s.setupQueueCallbacks()

	return s
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// 健康检查接口
	s.engine.Any("/healthz", s.healthCheck)

	// Swagger 文档路由
	s.engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := s.engine.Group("/api")
	{
		api.POST("/tasks", s.createTask)
		api.GET("/tasks/:id", s.getTask)
		api.GET("/tasks", s.getAllTasks)
		api.POST("/tasks/:id/stop", s.stopTask)
		api.POST("/config", s.updateConfig)
		api.GET("/events", s.sseHandler)
	}
}

// setupQueueCallbacks 设置队列事件回调
func (s *Server) setupQueueCallbacks() {
	s.queue.OnStart(func(id core.TaskID) {
		s.sseHub.Broadcast(SSEEvent{
			Event: "download-start",
			Data:  map[string]interface{}{"id": id},
		})
	})

	s.queue.OnSuccess(func(id core.TaskID) {
		s.sseHub.Broadcast(SSEEvent{
			Event: "download-success",
			Data:  map[string]interface{}{"id": id},
		})
	})

	s.queue.OnFailed(func(id core.TaskID, err error) {
		s.sseHub.Broadcast(SSEEvent{
			Event: "download-failed",
			Data:  map[string]interface{}{"id": id, "error": err.Error()},
		})
	})

	s.queue.OnStopped(func(id core.TaskID) {
		s.sseHub.Broadcast(SSEEvent{
			Event: "download-stop",
			Data:  map[string]interface{}{"id": id},
		})
	})

	// 注意：进度更新和消息事件已移除
	// 客户端应通过轮询 GET /api/tasks/{id} 接口获取实时进度
}

// healthCheck 健康检查接口
// @Summary 健康检查
// @Description 服务健康检查接口，用于监控服务是否正常运行
// @Tags Health
// @Produce plain
// @Success 200 {string} string "ok"
// @Router /healthz [get]
// @Router /healthz [post]
// @Router /healthz [put]
// @Router /healthz [delete]
// @Router /healthz [patch]
// @Router /healthz [head]
// @Router /healthz [options]
func (s *Server) healthCheck(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	ID             int64  `json:"id" example:"1"`                                            // 任务ID（可选，不提供时自动生成）
	Type           string `json:"type" binding:"required" example:"m3u8"`                    // 下载类型：m3u8/bilibili/direct
	URL            string `json:"url" binding:"required" example:"https://example.com/a.m3u8"` // 下载URL
	LocalDir       string `json:"localDir" binding:"required" example:"/downloads"`          // 本地保存目录
	Name           string `json:"name" binding:"required" example:"video.mp4"`               // 文件名
	DeleteSegments bool   `json:"deleteSegments" example:"true"`                             // 是否删除分段文件
	Headers        []string `json:"headers" example:"User-Agent: Mozilla/5.0"`              // 自定义HTTP头
	Proxy          string `json:"proxy" example:"http://proxy.com:8080"`                     // 代理地址
	Folder         string `json:"folder" example:"movies"`                                   // 子文件夹
}

// CreateTaskResponse 创建任务响应
type CreateTaskResponse struct {
	ID      int64  `json:"id" example:"1"`                     // 任务ID
	Message string `json:"message" example:"Task enqueued successfully"` // 响应消息
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error" example:"invalid request"` // 错误信息
}

// createTask 创建下载任务
// @Summary 创建下载任务
// @Description 创建一个新的下载任务并加入队列
// @Description 支持 M3U8、Bilibili、Direct 三种下载类型
// @Tags Tasks
// @Accept json
// @Produce json
// @Param task body CreateTaskRequest true "下载任务参数"
// @Success 200 {object} CreateTaskResponse "任务创建成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Router /tasks [post]
func (s *Server) createTask(c *gin.Context) {
	var params core.DownloadParams
	if err := c.ShouldBindJSON(&params); err != nil {
		logger.Warn("Invalid task creation request",
			zap.String("clientIP", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果客户端未提供 ID，自动生成
	if params.ID == 0 {
		s.mu.Lock()
		s.taskSeq++
		params.ID = core.TaskID(s.taskSeq)
		s.mu.Unlock()
	}

	logger.Info("Task creation request received",
		zap.Int64("id", int64(params.ID)),
		zap.String("type", string(params.Type)),
		zap.String("url", params.URL),
		zap.String("clientIP", c.ClientIP()))

	// 添加到队列
	s.queue.Enqueue(params)

	c.JSON(http.StatusOK, gin.H{
		"id":      params.ID,
		"message": "Task enqueued successfully",
	})
}

// getTask 获取单个任务状态
// @Summary 获取任务状态
// @Description 获取指定ID的任务状态和进度信息
// @Tags Tasks
// @Accept json
// @Produce json
// @Param id path int true "任务ID" example(1)
// @Success 200 {object} core.TaskInfo "任务信息"
// @Failure 400 {object} ErrorResponse "无效的任务ID"
// @Failure 404 {object} ErrorResponse "任务不存在"
// @Router /tasks/{id} [get]
func (s *Server) getTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.Warn("Invalid task ID in get request",
			zap.String("id", idStr),
			zap.String("clientIP", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	task, ok := s.queue.GetTask(core.TaskID(id))
	if !ok {
		logger.Warn("Task not found",
			zap.Int64("id", id),
			zap.String("clientIP", c.ClientIP()))
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	logger.Debug("Task info retrieved",
		zap.Int64("id", id),
		zap.String("status", string(task.Status)),
		zap.String("clientIP", c.ClientIP()))

	c.JSON(http.StatusOK, task)
}

// getAllTasks 获取所有任务状态
// @Summary 获取所有任务状态
// @Description 获取所有任务的状态和进度信息列表
// @Tags Tasks
// @Accept json
// @Produce json
// @Success 200 {array} core.TaskInfo "任务列表"
// @Router /tasks [get]
func (s *Server) getAllTasks(c *gin.Context) {
	tasks := s.queue.GetAllTasks()

	logger.Debug("All tasks info retrieved",
		zap.Int("count", len(tasks)),
		zap.String("clientIP", c.ClientIP()))

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// StopTaskResponse 停止任务响应
type StopTaskResponse struct {
	Message string `json:"message" example:"Task stopped"` // 响应消息
}

// stopTask 停止下载任务
// @Summary 停止下载任务
// @Description 停止指定ID的下载任务
// @Tags Tasks
// @Accept json
// @Produce json
// @Param id path int true "任务ID" example(1)
// @Success 200 {object} StopTaskResponse "任务停止成功"
// @Failure 400 {object} ErrorResponse "无效的任务ID"
// @Failure 404 {object} ErrorResponse "任务不存在"
// @Router /tasks/{id}/stop [post]
func (s *Server) stopTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.Warn("Invalid task ID in stop request",
			zap.String("id", idStr),
			zap.String("clientIP", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	logger.Info("Stop task request received",
		zap.Int64("id", id),
		zap.String("clientIP", c.ClientIP()))

	if err := s.queue.Stop(core.TaskID(id)); err != nil {
		logger.Warn("Failed to stop task",
			zap.Int64("id", id),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task stopped"})
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	MaxRunner int    `json:"maxRunner" example:"3"` // 最大并发下载数
	Proxy     string `json:"proxy" example:"http://proxy.com:8080"` // 代理服务器地址
}

// UpdateConfigResponse 更新配置响应
type UpdateConfigResponse struct {
	Message string `json:"message" example:"Config updated"` // 响应消息
}

// updateConfig 更新配置（并发数、代理）
// @Summary 更新系统配置
// @Description 更新系统配置，包括最大并发下载数和代理设置
// @Tags Config
// @Accept json
// @Produce json
// @Param config body UpdateConfigRequest true "配置参数"
// @Success 200 {object} UpdateConfigResponse "配置更新成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Router /config [post]
func (s *Server) updateConfig(c *gin.Context) {
	var config struct {
		MaxRunner int    `json:"maxRunner"`
		Proxy     string `json:"proxy"`
	}

	if err := c.ShouldBindJSON(&config); err != nil {
		logger.Warn("Invalid config update request",
			zap.String("clientIP", c.ClientIP()),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Info("Config update request received",
		zap.Int("maxRunner", config.MaxRunner),
		zap.String("proxy", config.Proxy),
		zap.String("clientIP", c.ClientIP()))

	if config.MaxRunner > 0 {
		s.queue.SetMaxRunner(config.MaxRunner)
		logger.Info("Max runner updated", zap.Int("maxRunner", config.MaxRunner))
	}

	if config.Proxy != "" {
		s.queue.SetProxy(config.Proxy)
		logger.Info("Proxy updated", zap.String("proxy", config.Proxy))
	}

	c.JSON(http.StatusOK, gin.H{"message": "Config updated"})
}

// sseHandler SSE 事件流处理器
// @Summary SSE 事件流
// @Description 订阅服务器推送事件（SSE），实时接收下载任务的状态变更通知
// @Description 事件类型包括：download-start（任务开始）, download-success（任务成功）, download-failed（任务失败）, download-stop（任务停止）
// @Description 注意：不包含进度更新事件，如需获取下载进度，请通过 GET /api/tasks/{id} 接口轮询
// @Tags Events
// @Produce text/event-stream
// @Success 200 {string} string "SSE 事件流"
// @Router /events [get]
func (s *Server) sseHandler(c *gin.Context) {
	logger.Info("SSE client connected", zap.String("clientIP", c.ClientIP()))

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 创建客户端通道
	client := make(chan SSEEvent, 10)
	s.sseHub.Register(client)
	defer func() {
		s.sseHub.Unregister(client)
		logger.Info("SSE client disconnected", zap.String("clientIP", c.ClientIP()))
	}()

	// 获取响应写入器
	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Error("SSE streaming not supported")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// 监听客户端断开
	notify := c.Request.Context().Done()

	// 发送事件流
	for {
		select {
		case <-notify:
			return
		case evt := <-client:
			fmt.Fprintf(w, "event: %s\n", evt.Event)
			fmt.Fprintf(w, "data: %s\n\n", evt.JSON())
			flusher.Flush()
		}
	}
}

// Run 启动 HTTP 服务器
func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

// SSEEvent SSE 事件
type SSEEvent struct {
	Event string
	Data  interface{}
}

// JSON 将事件数据序列化为 JSON
func (e SSEEvent) JSON() string {
	jsonBytes, _ := json.Marshal(e.Data)
	return string(jsonBytes)
}

// SSEHub SSE 事件广播中心
type SSEHub struct {
	clients map[chan SSEEvent]bool
	mu      sync.RWMutex
}

// NewSSEHub 创建 SSE Hub
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[chan SSEEvent]bool),
	}
}

// Register 注册客户端
func (h *SSEHub) Register(client chan SSEEvent) {
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()
}

// Unregister 注销客户端
func (h *SSEHub) Unregister(client chan SSEEvent) {
	h.mu.Lock()
	delete(h.clients, client)
	close(client)
	h.mu.Unlock()
}

// Broadcast 广播事件到所有客户端
func (h *SSEHub) Broadcast(evt SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client <- evt:
		default:
			// 客户端通道满，跳过
		}
	}
}

// Helper function to prevent unused import error
func init() {
	_ = io.EOF
}
