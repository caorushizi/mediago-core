package server

import (
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func (s *Server) registerRoutes() {
	s.engine.GET("/healthy", s.healthHandler.Check)
	s.engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := s.engine.Group("/api")
	{
		api.POST("/tasks", s.taskHandler.Create)
		api.GET("/tasks/:id", s.taskHandler.Get)
		api.GET("/tasks", s.taskHandler.List)
		api.POST("/tasks/:id/stop", s.taskHandler.Stop)

		api.POST("/config", s.configHandler.Update)

		api.GET("/events", s.eventHandler.Stream)
	}
}
