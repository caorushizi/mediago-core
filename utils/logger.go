package utils

import "go.uber.org/zap"

var Logger *zap.SugaredLogger

func init() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync() // flushes buffer, if any
	Logger = logger.Sugar()
}
