package server

import (
	"caorushizi.cn/mediago/internal/core"
	"caorushizi.cn/mediago/internal/logger"
)

func (s *Server) setupQueueCallbacks() {
	s.queue.OnStart(func(id core.TaskID) {
		s.hub.Broadcast("download-start", map[string]interface{}{"id": id})
	})

	s.queue.OnSuccess(func(id core.TaskID) {
		s.hub.Broadcast("download-success", map[string]interface{}{"id": id})
	})

	s.queue.OnFailed(func(id core.TaskID, err error) {
		s.hub.Broadcast("download-failed", map[string]interface{}{"id": id, "error": err.Error()})
	})

	s.queue.OnMessage(func(m core.MessageEvent) {
		logger.Infof("[task %s] %s", m.ID, m.Message)
	})

	s.queue.OnStopped(func(id core.TaskID) {
		s.hub.Broadcast("download-stop", map[string]interface{}{"id": id})
	})
}
