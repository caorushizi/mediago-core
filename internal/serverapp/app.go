package serverapp

import (
	"fmt"
	"net"
	"os"
	"strings"

	"caorushizi.cn/mediago/internal/api"
	"caorushizi.cn/mediago/internal/core"
	"caorushizi.cn/mediago/internal/core/runner"
	"caorushizi.cn/mediago/internal/core/schema"
	"caorushizi.cn/mediago/internal/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	defaultMode = "release"
	defaultHost = "0.0.0.0"
	defaultPort = "8080"
)

func Run(rawConfig string) error {
	cfg, err := loadAppConfig(rawConfig)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if err := initLogger(cfg.Log); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer logger.Sync()

	if err := writeDefaultConfigTemplate(); err != nil {
		logger.Warnf("failed to write default config template: %v", err)
	}

	logger.Info("MediaGo Downloader Service Starting...")

	schemas, err := loadSchemas(cfg.SchemaPath)
	if err != nil {
		return err
	}

	queue := buildTaskQueue(cfg, schemas)

	if err := startHTTPServer(queue); err != nil {
		return fmt.Errorf("start http server: %w", err)
	}

	return nil
}

func initLogger(cfg logConfig) error {
	logCfg := logger.DefaultConfig()
	logCfg.Level = cfg.Level
	logCfg.LogDir = cfg.Dir
	if strings.TrimSpace(cfg.File) != "" {
		logCfg.LogFileName = cfg.File
	}
	if strings.TrimSpace(cfg.DownloaderFile) != "" {
		logCfg.DownloaderLogFileName = cfg.DownloaderFile
	}
	return logger.Init(logCfg)
}

func loadSchemas(path string) (schema.SchemaList, error) {
	schemaPath := resolveSchemaPath(path)
	logger.Infof("Loading schemas from: %s", schemaPath)

	schemas, err := schema.LoadSchemasFromJSON(schemaPath)
	if err != nil {
		return schema.SchemaList{}, fmt.Errorf("load schemas: %w", err)
	}

	logger.Infof("Loaded %d download schemas", len(schemas.Schemas))
	return schemas, nil
}

func buildTaskQueue(cfg appConfig, schemas schema.SchemaList) *core.TaskQueue {
	binMap := cfg.Binaries.toMap()
	for dt, path := range binMap {
		logger.Infof("%s downloader: %s", dt, path)
	}

	runner := runner.NewPTYRunner()
	downloader := core.NewDownloader(binMap, runner, schemas)
	queueCfg := cfg.Queue.toCoreConfig()
	queue := core.NewTaskQueue(downloader, queueCfg)

	logger.Info("Task queue initialized with defaults",
		zap.Int("maxRunner", queueCfg.MaxRunner),
		zap.String("localDir", queueCfg.LocalDir),
		zap.Bool("deleteSegments", queueCfg.DeleteSegments),
		zap.String("proxy", queueCfg.Proxy))

	return queue
}

func startHTTPServer(queue *core.TaskQueue) error {
	server := api.NewServer(queue)
	addr := buildListenAddr()

	ginMode := getEnv("GIN_MODE", defaultMode)
	if strings.TrimSpace(ginMode) == "" {
		ginMode = defaultMode
	}
	gin.SetMode(ginMode)

	logger.Infof("Starting HTTP server on %s", addr)
	logger.Info("API Endpoints:")
	logger.Info("  GET  /healthy            - Health check")
	logger.Info("  POST /api/tasks          - Create download task")
	logger.Info("  GET  /api/tasks          - Get all tasks status")
	logger.Info("  GET  /api/tasks/:id      - Get task status")
	logger.Info("  POST /api/tasks/:id/stop - Stop task")
	logger.Info("  POST /api/config         - Update config")
	logger.Info("  GET  /api/events         - SSE event stream (status changes only)")
	logger.Info("Swagger Documentation:")
	logger.Infof("  http://%s/swagger/index.html", addr)

	return server.Run(addr)
}

func buildListenAddr() string {
	host := getEnv("HOST", defaultHost)
	port := getEnv("PORT", defaultPort)
	return net.JoinHostPort(host, port)
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
