// MediaGo 下载服务主程序
package main

import (
	"os"
	"path/filepath"

	"caorushizi.cn/mediago/internal/api"
	"caorushizi.cn/mediago/internal/core"
	"caorushizi.cn/mediago/internal/core/runner"
	"caorushizi.cn/mediago/internal/core/schema"
	"caorushizi.cn/mediago/internal/logger"
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

func main() {
	// Read config from env/flags
	mode := getEnv("GIN_MODE", "release") // "debug" / "release" / "test"
	defaultHost := getEnv("HOST", "0.0.0.0")
	defaultPort := getEnv("PORT", "8080")

	// 1. 初始化日志系统
	logCfg := logger.DefaultConfig()
	logCfg.Level = getEnv("MEDIAGO_LOG_LEVEL", "info")
	logCfg.LogDir = getEnv("MEDIAGO_LOG_DIR", "./logs")

	if err := logger.Init(logCfg); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("MediaGo Downloader Service Starting...")

	// 2. 加载 JSON Schema 配置
	schemaPath := getConfigPath()
	logger.Infof("Loading schemas from: %s", schemaPath)

	schemas, err := schema.LoadSchemasFromJSON(schemaPath)
	if err != nil {
		logger.Fatalf("Failed to load schemas: %v", err)
	}
	logger.Infof("Loaded %d download schemas", len(schemas.Schemas))

	// 3. 配置下载器二进制路径
	binMap := getBinaryMap()
	for dt, path := range binMap {
		logger.Infof("%s downloader: %s", dt, path)
	}

	// 4. 创建核心组件
	r := runner.NewPTYRunner()
	downloader := core.NewDownloader(binMap, r, schemas)
	queue := core.NewTaskQueue(downloader, 2) // 默认并发数：2

	logger.Info("Task queue initialized (maxRunner=2)")

	// 5. 启动 HTTP 服务器
	server := api.NewServer(queue)
	addr := defaultHost + ":" + defaultPort
	gin.SetMode(mode)
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

// getConfigPath 获取配置文件路径
func getConfigPath() string {
	if path := os.Getenv("MEDIAGO_SCHEMA_PATH"); path != "" {
		return path
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

// getBinaryMap 获取下载器二进制路径映射
func getBinaryMap() map[core.DownloadType]string {
	binMap := make(map[core.DownloadType]string)

	// 从环境变量读取，或使用默认路径
	binMap[core.TypeM3U8] = getEnv("MEDIAGO_M3U8_BIN", "/usr/local/bin/N_m3u8DL-RE")
	binMap[core.TypeBilibili] = getEnv("MEDIAGO_BILIBILI_BIN", "/usr/local/bin/BBDown")
	binMap[core.TypeDirect] = getEnv("MEDIAGO_DIRECT_BIN", "/usr/local/bin/aria2c")

	return binMap
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
