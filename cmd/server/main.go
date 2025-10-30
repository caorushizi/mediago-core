// MediaGo 下载服务主程序
package main

import (
	"flag"
	"os"
	"path/filepath"

	"caorushizi.cn/mediago/internal/api"
	"caorushizi.cn/mediago/internal/core"
	"caorushizi.cn/mediago/internal/core/runner"
	"caorushizi.cn/mediago/internal/core/schema"
	"caorushizi.cn/mediago/internal/logger"
	"caorushizi.cn/mediago/internal/tasklog"
	"github.com/gin-gonic/gin"

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

// AppConfig 存储所有配置项
type AppConfig struct {
	GinMode        string `json:"gin_mode"`
	Host           string `json:"host"`
	Port           string `json:"port"`
	LogLevel       string `json:"log_level"`
	LogDir         string `json:"log_dir"`
	SchemaPath     string `json:"schema_path"`
	M3U8Bin        string `json:"m3u8_bin"`
	BilibiliBin    string `json:"bilibili_bin"`
	DirectBin      string `json:"direct_bin"`
	MaxRunner      int    `json:"max_runner"`
	LocalDir       string `json:"local_dir"`
	DeleteSegments bool   `json:"delete_segments"`
	Proxy          string `json:"proxy"`
	UseProxy       bool   `json:"use_proxy"`
}

func (c *AppConfig) GetLocalDir() string {
	return c.LocalDir
}

func (c *AppConfig) GetDeleteSegments() bool {
	return c.DeleteSegments
}

func (c *AppConfig) GetProxy() string {
	return c.Proxy
}

func (c *AppConfig) GetUseProxy() bool {
	return c.UseProxy
}

func (c *AppConfig) SetLocalDir(dir string) {
	c.LocalDir = dir
}

func (c *AppConfig) SetDeleteSegments(del bool) {
	c.DeleteSegments = del
}

func (c *AppConfig) SetProxy(proxy string) {
	c.Proxy = proxy
}

func (c *AppConfig) SetUseProxy(useProxy bool) {
	c.UseProxy = useProxy
}

func main() {
	// 1. 先用默认配置初始化日志系统，以便在配置解析过程中使用
	if err := logger.Init(logger.DefaultConfig()); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	// 2. 初始化和解析配置
	cfg := initConfig()

	// 3. 根据配置重新初始化日志系统
	logCfg := logger.DefaultConfig()
	logCfg.Level = cfg.LogLevel
	logCfg.LogDir = cfg.LogDir

	if err := logger.Init(logCfg); err != nil {
		logger.Fatalf("Failed to reinitialize logger with config: %v", err)
	}

	logger.Info("MediaGo Downloader Service Starting...")
	logger.Infof("Final Config: %+v", cfg)

	// 4. 加载 JSON Schema 配置
	logger.Infof("Loading schemas from: %s", cfg.SchemaPath)
	schemas, err := schema.LoadSchemasFromJSON(cfg.SchemaPath)
	if err != nil {
		logger.Fatalf("Failed to load schemas: %v", err)
	}
	logger.Infof("Loaded %d download schemas", len(schemas.Schemas))

	// 5. 配置下载器二进制路径
	binMap := getBinaryMap(cfg)
	for dt, path := range binMap {
		logger.Infof("%s downloader: %s", dt, path)
	}

	// 6. 创建核心组件
	r := runner.NewPTYRunner()
	downloader := core.NewDownloader(binMap, r, schemas, cfg)
	queue := core.NewTaskQueue(downloader, cfg.MaxRunner)
	taskLogs := tasklog.NewManager(filepath.Join(cfg.LogDir, "tasks"))

	logger.Infof("Task queue initialized (maxRunner=%d)", cfg.MaxRunner)
	logger.Infof("Task logs will be stored in %s", filepath.Join(cfg.LogDir, "tasks"))

	// 7. 启动 HTTP 服务器
	server := api.NewServer(queue, taskLogs)
	addr := cfg.Host + ":" + cfg.Port
	gin.SetMode(cfg.GinMode)
	logger.Infof("Starting HTTP server on %s", addr)

	if err := server.Run(addr); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}

// initConfig 初始化配置，遵循优先级：命令行 > 环境变量 > JSON字符串 > 默认值
func initConfig() *AppConfig {
	// 默认配置
	cfg := &AppConfig{
		GinMode:        "release",
		Host:           "0.0.0.0",
		Port:           "8080",
		LogLevel:       "info",
		LogDir:         "./logs",
		SchemaPath:     "", // 稍后计算默认值
		M3U8Bin:        "",
		BilibiliBin:    "",
		DirectBin:      "",
		MaxRunner:      2,
		LocalDir:       "./downloads",
		DeleteSegments: true,
		Proxy:          "",
		UseProxy:       false,
	}

	// 1. 定义其他命令行标志
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug/info/warn/error)")
	flag.StringVar(&cfg.LogDir, "log-dir", cfg.LogDir, "Log directory")
	flag.StringVar(&cfg.M3U8Bin, "m3u8-bin", cfg.M3U8Bin, "M3U8 downloader binary path")
	flag.StringVar(&cfg.BilibiliBin, "bilibili-bin", cfg.BilibiliBin, "Bilibili downloader binary path")
	flag.StringVar(&cfg.DirectBin, "direct-bin", cfg.DirectBin, "Direct downloader binary path")
	flag.StringVar(&cfg.SchemaPath, "schema-path", cfg.SchemaPath, "Path to the download schema config.json")
	flag.StringVar(&cfg.Port, "port", cfg.Port, "Server port")
	flag.StringVar(&cfg.LocalDir, "local-dir", cfg.LocalDir, "Default download directory")
	flag.BoolVar(&cfg.DeleteSegments, "delete-segments", cfg.DeleteSegments, "Delete segments after download")
	flag.StringVar(&cfg.Proxy, "proxy", cfg.Proxy, "Proxy for downloader")
	flag.BoolVar(&cfg.UseProxy, "use-proxy", cfg.UseProxy, "Enable proxy")
	flag.IntVar(&cfg.MaxRunner, "max-runner", cfg.MaxRunner, "Maximum concurrent download runners")

	flag.Parse()

	// 3. 从环境变量加载（会覆盖 JSON 和默认值）
	cfg.GinMode = getEnv("GIN_MODE", cfg.GinMode)
	cfg.Host = getEnv("HOST", cfg.Host)
	cfg.Port = getEnv("PORT", cfg.Port)

	// 如果 SchemaPath 仍然为空，则计算其默认值
	if cfg.SchemaPath == "" {
		cfg.SchemaPath = getDefaultSchemaPath()
	}

	return cfg
}

// getDefaultSchemaPath 获取配置文件的默认路径
func getDefaultSchemaPath() string {
	// 默认路径：优先使用可执行文件所在目录下的 config.json
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	localConfig := filepath.Join(execDir, "config.json")
	if _, err := os.Stat(localConfig); err == nil {
		return localConfig
	}
	// 回退到仓库内的配置文件路径
	return "configs/config.json"
}

// getBinaryMap 从配置中获取下载器二进制路径映射
func getBinaryMap(cfg *AppConfig) map[core.DownloadType]string {
	return map[core.DownloadType]string{
		core.TypeM3U8:     cfg.M3U8Bin,
		core.TypeBilibili: cfg.BilibiliBin,
		core.TypeDirect:   cfg.DirectBin,
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
