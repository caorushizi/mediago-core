package serverapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
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

	if err := startHTTPServer(cfg, queue); err != nil {
		return fmt.Errorf("start http server: %w", err)
	}

	return nil
}

func initLogger(cfg logConfig) error {
	logCfg := logger.DefaultConfig()
	logCfg.Level = cfg.Level
	logCfg.LogDir = cfg.Dir
	return logger.Init(logCfg)
}

func loadSchemas(path string) (schema.Repository, error) {
	schemaPath := resolveSchemaPath(path)
	logger.Infof("Loading schemas from: %s", schemaPath)

	schemas, err := schema.LoadSchemasFromJSON(schemaPath)
	if err != nil {
		return schema.Repository{}, fmt.Errorf("load schemas: %w", err)
	}

	logger.Infof("Loaded %d download schemas", len(schemas.Schemas))
	return schemas, nil
}

func buildTaskQueue(cfg appConfig, schemas schema.Repository) *core.TaskQueue {
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

func startHTTPServer(cfg appConfig, queue *core.TaskQueue) error {
	server := api.NewServer(queue)
	addr := buildListenAddr(cfg)

	ginMode := getEnv("GIN_MODE", cfg.Mode)
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

func buildListenAddr(cfg appConfig) string {
	host := getEnv("HOST", cfg.Host)
	port := getEnv("PORT", cfg.Port)
	return net.JoinHostPort(host, port)
}

func resolveSchemaPath(override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}

	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	localConfig := filepath.Join(execDir, "config.json")
	if _, err := os.Stat(localConfig); err == nil {
		return localConfig
	}

	return filepath.Join(execDir, "..", "..", "configs", "config.json")
}

func loadAppConfig(raw string) (appConfig, error) {
	cfg := defaultAppConfig()
	if strings.TrimSpace(raw) == "" {
		return cfg, nil
	}

	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return appConfig{}, err
	}

	cfg.applyDefaults()
	return cfg, nil
}

type appConfig struct {
	Mode       string           `json:"mode"`
	Host       string           `json:"host"`
	Port       string           `json:"port"`
	Log        logConfig        `json:"log"`
	SchemaPath string           `json:"schemaPath"`
	Queue      queueConfigInput `json:"queue"`
	Binaries   binaryConfig     `json:"binaries"`
}

type logConfig struct {
	Level string `json:"level"`
	Dir   string `json:"dir"`
}

type queueConfigInput struct {
	MaxRunner      int    `json:"maxRunner"`
	LocalDir       string `json:"localDir"`
	DeleteSegments bool   `json:"deleteSegments"`
	Proxy          string `json:"proxy"`
}

func (q queueConfigInput) toCoreConfig() core.QueueConfig {
	return core.QueueConfig{
		MaxRunner:      q.MaxRunner,
		LocalDir:       q.LocalDir,
		DeleteSegments: q.DeleteSegments,
		Proxy:          q.Proxy,
	}
}

type binaryConfig struct {
	M3U8     string `json:"m3u8"`
	Bilibili string `json:"bilibili"`
	Direct   string `json:"direct"`
}

func (b binaryConfig) toMap() map[core.DownloadType]string {
	return map[core.DownloadType]string{
		core.TypeM3U8:     b.M3U8,
		core.TypeBilibili: b.Bilibili,
		core.TypeDirect:   b.Direct,
	}
}

func defaultAppConfig() appConfig {
	return appConfig{
		Mode: defaultMode,
		Host: defaultHost,
		Port: defaultPort,
		Log: logConfig{
			Level: "info",
			Dir:   "./logs",
		},
		Queue: queueConfigInput{
			MaxRunner:      2,
			LocalDir:       "./downloads",
			DeleteSegments: false,
			Proxy:          "",
		},
		Binaries: binaryConfig{
			M3U8:     "/usr/local/bin/N_m3u8DL-RE",
			Bilibili: "/usr/local/bin/BBDown",
			Direct:   "/usr/local/bin/aria2c",
		},
	}
}

func (c *appConfig) applyDefaults() {
	if strings.TrimSpace(c.Mode) == "" {
		c.Mode = defaultMode
	}
	if strings.TrimSpace(c.Host) == "" {
		c.Host = defaultHost
	}
	if strings.TrimSpace(c.Port) == "" {
		c.Port = defaultPort
	}
	if strings.TrimSpace(c.Log.Level) == "" {
		c.Log.Level = "info"
	}
	if strings.TrimSpace(c.Log.Dir) == "" {
		c.Log.Dir = "./logs"
	}
	if c.Queue.MaxRunner <= 0 {
		c.Queue.MaxRunner = 2
	}
	if strings.TrimSpace(c.Queue.LocalDir) == "" {
		c.Queue.LocalDir = "./downloads"
	}
	if strings.TrimSpace(c.Binaries.M3U8) == "" {
		c.Binaries.M3U8 = "/usr/local/bin/N_m3u8DL-RE"
	}
	if strings.TrimSpace(c.Binaries.Bilibili) == "" {
		c.Binaries.Bilibili = "/usr/local/bin/BBDown"
	}
	if strings.TrimSpace(c.Binaries.Direct) == "" {
		c.Binaries.Direct = "/usr/local/bin/aria2c"
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func writeDefaultConfigTemplate() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	execDir := filepath.Dir(execPath)
	configPath := filepath.Join(execDir, "config.default.json")

	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check default config file: %w", err)
	}

	cfg := defaultAppConfig()
	cfg.applyDefaults()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}

	logger.Infof("Default config template created at %s", configPath)
	return nil
}
