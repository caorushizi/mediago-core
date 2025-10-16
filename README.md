# MediaGo core

> 多任务下载系统 - Go (Gin) 实现

## 🚀 快速导航

### 📖 核心文档

- **[快速开始（5 分钟）](QUICKSTART.md)** - 新手必读
- **[Taskfile 使用指南](TASKFILE_GUIDE.md)** ⭐ - 推荐构建工具
- **[完整使用文档](README_core.md)** - API 与配置详解
- **[最终交付总结](FINAL_SUMMARY.md)** - 项目概览

### 🔧 技术文档

- **[实现总结](IMPLEMENTATION_SUMMARY.md)** - 技术细节与核心特性
- **[项目结构](PROJECT_STRUCTURE.md)** - 架构与模块关系
- **[交付清单](DELIVERY_CHECKLIST.md)** - 验收标准

---

## ⚡ 快速开始

### 使用 Taskfile（推荐）

```bash
# 安装 Task
brew install go-task  # macOS
choco install go-task # Windows

# 查看所有任务
task --list

# 运行服务
task run

# 编译应用
task build
```

### 传统方式

```bash
# 安装依赖
go mod tidy

# 运行服务
go run ./cmd/core
```

### Docker 部署

```bash
# Docker Compose
docker-compose up -d

# 或使用 Taskfile
task docker:build
task docker:run
```

---

## 🎯 核心特性

- ✅ **JSON 配置**（从 YAML 迁移）
- ✅ **进度节流**（200ms + 0.5%）
- ✅ **并发控制**（动态调整）
- ✅ **事件驱动**（SSE 实时推送）
- ✅ **Swagger 文档**（完整的 API 文档）
- ✅ **Taskfile 集成**（15 个精简任务）
- ✅ **Docker 支持**（一键部署）

---

## 📖 API 文档

项目集成了 **Swagger API 文档**，提供了完整的接口说明和在线测试功能。

### 访问 Swagger UI

```bash
# 1. 启动服务
task run

# 2. 打开 Swagger UI
task swagger:open

# 或直接访问: http://localhost:8080/swagger/index.html
```

### 生成 Swagger 文档

```bash
# 安装 swag 工具
task tools

# 生成文档
task swagger

# 或手动生成
swag init -g cmd/core/main.go -o docs --parseDependency --parseInternal
```

### API 端点列表

- `POST /api/tasks` - 创建下载任务
- `POST /api/tasks/:id/stop` - 停止任务
- `POST /api/config` - 更新配置
- `GET /api/events` - SSE 事件流
- `GET /swagger/*any` - Swagger UI

---

## 📦 支持的下载类型

1. **M3U8** - HLS 流媒体下载
2. **Bilibili** - B站视频下载
3. **Direct** - 直接文件下载（aria2c）

---

## 🛠️ Taskfile 任务

```bash
# 常用命令
task run          # 运行服务
task build        # 编译当前平台
task test         # 运行测试
task fmt          # 格式化代码
task clean        # 清理构建产物

# 构建和发布
task build:all    # 交叉编译所有平台
task release      # 构建发布版本

# Swagger 文档
task swagger      # 生成 Swagger 文档
task swagger:open # 打开 Swagger UI

# 其他
task tools        # 安装开发工具
task --list       # 查看所有任务
```

---

## 📚 完整文档列表

| 文档 | 说明 | 适合人群 |
|------|------|----------|
| [QUICKSTART.md](QUICKSTART.md) | 5 分钟快速启动 | 所有人 |
| [SWAGGER_GUIDE.md](SWAGGER_GUIDE.md) | Swagger API 文档指南 | API 开发者 |
| [TASKFILE_GUIDE.md](TASKFILE_GUIDE.md) | Taskfile 使用指南 | 开发者 |
| [TASKFILE_SIMPLIFICATION.md](TASKFILE_SIMPLIFICATION.md) | Taskfile 简化说明 | 开发者 |
| [README_core.md](README_core.md) | 完整使用文档 | API 使用者 |
| [FINAL_SUMMARY.md](FINAL_SUMMARY.md) | 最终交付总结 | 项目经理 |
| [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) | 实现总结 | 架构师 |
| [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) | 项目结构 | 维护者 |
| [DELIVERY_CHECKLIST.md](DELIVERY_CHECKLIST.md) | 交付清单 | QA 测试 |

---

## 🎉 项目状态

**✅ 已完成，可直接运行**

- 代码：~700 行（8+ 个 Go 文件）
- 文档：~70 KB（7+ 份文档）
- 任务：32+ 个 Taskfile 任务
- API 文档：完整 Swagger 支持
- 测试：完整客户端页面

---

## 📞 获取帮助

有问题？查看文档：

1. **新手入门** → [QUICKSTART.md](QUICKSTART.md)
2. **API 文档** → [SWAGGER_GUIDE.md](SWAGGER_GUIDE.md) 或访问 http://localhost:8080/swagger/index.html
3. **使用 Taskfile** → [TASKFILE_GUIDE.md](TASKFILE_GUIDE.md)
4. **API 调用** → [README_core.md](README_core.md)
5. **故障排查** → [README_core.md#故障排查](README_core.md#故障排查)

---

**版本：** v1.0.0
**最后更新：** 2025-10-12
