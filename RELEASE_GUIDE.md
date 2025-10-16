# MediaGo 发布指南

## 发布任务说明

### 1. 开发阶段
- `task dev` - 启动开发服务器（自动使用本地下载器工具）
- `task dev:env` - 查看当前环境配置
- `task dev:build` - 快速编译当前平台

### 2. 发布阶段

#### 2.1 构建所有平台二进制文件
```bash
task release:build
```
这将编译所有平台的二进制文件到 `./bin/` 目录：
- mediago-core-linux-amd64
- mediago-core-linux-arm64
- mediago-core-darwin-amd64
- mediago-core-darwin-arm64
- mediago-core-windows-amd64.exe
- mediago-core-windows-arm64.exe

#### 2.2 打包完整发布包（推荐）
```bash
task release:package
```
这将为每个平台创建完整的发布包，包含：
- 主程序二进制文件
- 对应平台的下载器工具（从 `.bin/<platform>/<arch>/` 复制）
- 配置文件
- 说明文档
- 启动脚本（Linux/macOS）

发布包位置：`./release/packages/`

发布包结构示例：
```
release/packages/mediago-core-windows-amd64/
├── mediago-core.exe    # 主程序
├── bin/                        # 下载器工具
│   ├── N_m3u8DL-RE.exe
│   ├── BBDown.exe
│   ├── gopeed.exe
│   └── ffmpeg.exe
├── configs/                    # 配置文件
│   └── download_schemas.json
├── logs/                       # 日志目录（空）
└── README.txt                  # 使用说明
```

#### 2.3 压缩发布包
```bash
task release:package:zip
```
将所有发布包压缩为 zip 文件，位于 `./release/archives/`

#### 2.4 完整发布流程（一键发布）
```bash
task release:prepare
```
这将依次执行：
1. 清理旧的发布产物
2. 整理依赖
3. 运行完整测试
4. 构建所有平台
5. 打包所有平台
6. 压缩所有平台

### 3. NPM 发布
```bash
# 演练（不实际发布）
task release:npm:dry-run VERSION=1.0.0

# 正式发布
task release:npm VERSION=1.0.0 PUBLISH=true
```

## 发布包的使用

### Windows
1. 解压 zip 文件
2. 双击运行 `mediago-core.exe`

### Linux/macOS
1. 解压 zip 文件
2. 在终端中运行：
   ```bash
   cd mediago-core-xxx
   chmod +x mediago-core
   ./start.sh
   ```

## 环境变量说明

发布包中的程序会自动使用相对路径查找下载器工具：
- `./bin/N_m3u8DL-RE` (或 `.exe`)
- `./bin/BBDown` (或 `.exe`)
- `./bin/gopeed` (或 `.exe`)

用户无需配置任何环境变量即可运行！

## 常见问题

Q: 如何只打包特定平台？
A: 目前需要打包所有平台。如需单独平台，建议修改 `Taskfile.yml` 中的 `release:package` 任务。

Q: 下载器工具从哪里来？
A: 从项目根目录的 `.bin/<platform>/<arch>/` 目录复制。确保这些文件存在且可执行。

Q: 如何添加新的下载器工具？
A: 将工具放入 `.bin/<platform>/<arch>/` 目录，它会自动被复制到发布包中。
