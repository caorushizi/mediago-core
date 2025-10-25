package server

import (
	"caorushizi.cn/mediago/internal/api/handler"
	"caorushizi.cn/mediago/internal/api/sse"
	"caorushizi.cn/mediago/internal/core"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server 包装 Gin Engine 与业务依赖。
type Server struct {
	queue  *core.TaskQueue
	hub    *sse.Hub
	engine *gin.Engine

	taskHandler   *handler.TaskHandler
	configHandler *handler.ConfigHandler
	eventHandler  *handler.EventHandler
	healthHandler *handler.HealthHandler
}

// New 创建 HTTP 服务器实例。
func New(queue *core.TaskQueue) *Server {
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	hub := sse.New()

	srv := &Server{
		queue:         queue,
		hub:           hub,
		engine:        engine,
		taskHandler:   handler.NewTaskHandler(queue),
		configHandler: handler.NewConfigHandler(queue),
		eventHandler:  handler.NewEventHandler(hub),
		healthHandler: handler.NewHealthHandler(),
	}

	srv.registerRoutes()
	srv.setupQueueCallbacks()

	return srv
}

// Run 启动 HTTP 服务。
func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

// Engine 返回底层 Gin Engine（主要用于测试）。
func (s *Server) Engine() *gin.Engine {
	return s.engine
}
