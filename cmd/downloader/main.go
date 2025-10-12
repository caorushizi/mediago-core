// MediaGo 下载服务主程序
package main

import (
	"log"
	"os"
	"path/filepath"

	"caorushizi.cn/mediago/internal/api"
	"caorushizi.cn/mediago/internal/core"

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

// @tag.name Tasks
// @tag.description 下载任务管理相关接口
// @tag.name Config
// @tag.description 系统配置相关接口
// @tag.name Events
// @tag.description 实时事件推送相关接口

func main() {
	log.Println("🚀 MediaGo Downloader Service Starting...")

	// 1. 加载 JSON Schema 配置
	schemaPath := getConfigPath()
	log.Printf("📄 Loading schemas from: %s", schemaPath)

	schemas, err := core.LoadSchemasFromJSON(schemaPath)
	if err != nil {
		log.Fatalf("❌ Failed to load schemas: %v", err)
	}
	log.Printf("✅ Loaded %d download schemas", len(schemas.Schemas))

	// 2. 配置下载器二进制路径
	binMap := getBinaryMap()
	for dt, path := range binMap {
		log.Printf("🔧 %s downloader: %s", dt, path)
	}

	// 3. 创建核心组件
	runner := core.NewExecRunner()
	downloader := core.NewDownloader(binMap, runner, schemas)
	queue := core.NewTaskQueue(downloader, 2) // 默认并发数：2

	log.Println("⚙️  Task queue initialized (maxRunner=2)")

	// 4. 启动 HTTP 服务器
	server := api.NewServer(queue)
	addr := getServerAddr()
	log.Printf("🌐 Starting HTTP server on %s", addr)
	log.Println("📡 API Endpoints:")
	log.Println("   POST /api/tasks          - Create download task")
	log.Println("   POST /api/tasks/:id/stop - Stop task")
	log.Println("   POST /api/config         - Update config")
	log.Println("   GET  /api/events         - SSE event stream")
	log.Println("📖 Swagger Documentation:")
	log.Printf("   http://localhost%s/swagger/index.html\n", addr)

	if err := server.Run(addr); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}

// getConfigPath 获取配置文件路径
func getConfigPath() string {
	if path := os.Getenv("MEDIAGO_SCHEMA_PATH"); path != "" {
		return path
	}
	// 默认路径：相对于可执行文件的 configs 目录
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	return filepath.Join(execDir, "..", "..", "configs", "download_schemas.json")
}

// getBinaryMap 获取下载器二进制路径映射
func getBinaryMap() map[core.DownloadType]string {
	binMap := make(map[core.DownloadType]string)

	// 从环境变量读取，或使用默认路径
	binMap[core.TypeM3U8] = getEnvOrDefault("MEDIAGO_M3U8_BIN", "/usr/local/bin/N_m3u8DL-RE")
	binMap[core.TypeBilibili] = getEnvOrDefault("MEDIAGO_BILIBILI_BIN", "/usr/local/bin/BBDown")
	binMap[core.TypeDirect] = getEnvOrDefault("MEDIAGO_DIRECT_BIN", "/usr/local/bin/aria2c")

	return binMap
}

// getServerAddr 获取服务器监听地址
func getServerAddr() string {
	return getEnvOrDefault("MEDIAGO_SERVER_ADDR", ":8080")
}

// getEnvOrDefault 获取环境变量或返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
