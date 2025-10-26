// MediaGo 下载服务主程序
package main

import (
	"encoding/json"
	"flag"
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

	_ "caorushizi.cn/mediago/docs" // Swagger 文档
)

// @title MediaGo Downloader API
// @version 1.0
// @description MediaGo 多任务下载系统 API 文档
// @description 支持 M3U8、Bilibili、Direct 三种下载类型
// @description 提供任务管理、配置更新、实时事件推送等功能
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url https://github.com/caorushizi/mediago-core
// @contact.email support@mediago.local

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api
// @schemes http https

// @tag.name Health
// @tag.description 健康检查相关接口
// @tag.name Tasks
// @tag.description 下载任务管理相关接口
// @tag.name Config
// @tag.description 系统配置相关接口
// @tag.name Events
// @tag.description 实时事件推送相关接口

func main() {
	configJSON := flag.String("config", "", "JSON string with server configuration")
	flag.Parse()

	appCfg, err := loadAppConfig(*configJSON)
	if err != nil {
		panic("Failed to parse config: " + err.Error())
	}

	// 1. 初始化日志系统
	logCfg := logger.DefaultConfig()
	logCfg.Level = appCfg.Log.Level
	logCfg.LogDir = appCfg.Log.Dir

	if err := logger.Init(logCfg); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("MediaGo Downloader Service Starting...")

	// 2. 加载 JSON Schema 配置
	schemaPath := resolveSchemaPath(appCfg.SchemaPath)
	logger.Infof("Loading schemas from: %s", schemaPath)

	schemas, err := schema.LoadSchemasFromJSON(schemaPath)
	if err != nil {
		logger.Fatalf("Failed to load schemas: %v", err)
	}
	logger.Infof("Loaded %d download schemas", len(schemas.Schemas))

	// 3. 配置下载器二进制路径
	binMap := appCfg.Binaries.toMap()
	for dt, path := range binMap {
		logger.Infof("%s downloader: %s", dt, path)
	}

	// 4. 创建核心组件
	r := runner.NewPTYRunner()
	downloader := core.NewDownloader(binMap, r, schemas)
	queueCfg := appCfg.Queue.toCoreConfig()

	queue := core.NewTaskQueue(downloader, queueCfg)

	logger.Info("Task queue initialized with defaults",
		zap.Int("maxRunner", queueCfg.MaxRunner),
		zap.String("localDir", queueCfg.LocalDir),
		zap.Bool("deleteSegments", queueCfg.DeleteSegments),
		zap.String("proxy", queueCfg.Proxy))

	// 5. 启动 HTTP 服务器
	server := api.NewServer(queue)
	host := getEnv("HOST", appCfg.Host)
	port := getEnv("PORT", appCfg.Port)
	addr := net.JoinHostPort(host, port)
	gin.SetMode(appCfg.Mode)
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

	if err := server.Run(addr); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}

// resolveSchemaPath 获取配置文件路径
func resolveSchemaPath(override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	// 默认路径：优先使用可执行文件所在目录下的 config.json
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	localConfig := filepath.Join(execDir, "config.json")
	if _, err := os.Stat(localConfig); err == nil {
		return localConfig
	}
	// 回退到仓库内的配置文件路径
	return filepath.Join(execDir, "..", "..", "configs", "config.json")
}

// loadAppConfig loads application configuration from a JSON string and applies defaults.
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
		Mode: "release",
		Host: "0.0.0.0",
		Port: "8080",
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
		c.Mode = "release"
	}
	if strings.TrimSpace(c.Host) == "" {
		c.Host = "0.0.0.0"
	}
	if strings.TrimSpace(c.Port) == "" {
		c.Port = "8080"
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
