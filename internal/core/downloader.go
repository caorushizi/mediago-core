// Package core 包含下载器服务实现
package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

var (
	ErrUnsupportedType = errors.New("unsupported download type")
	ErrBinNotFound     = errors.New("binary not found for type")
)

// DownloaderSvc 下载器服务
type DownloaderSvc struct {
	binMap  map[DownloadType]string // 下载类型到可执行文件路径的映射
	runner  Runner                  // 命令执行器
	schemas SchemaList              // Schema 配置列表
	tracker *progressTracker        // 进度节流器
}

// NewDownloader 创建下载器服务实例
func NewDownloader(binMap map[DownloadType]string, runner Runner, schemas SchemaList) *DownloaderSvc {
	return &DownloaderSvc{
		binMap:  binMap,
		runner:  runner,
		schemas: schemas,
		tracker: newTracker(),
	}
}

// buildArgs 根据 Schema 构建命令行参数
func (d *DownloaderSvc) buildArgs(p DownloadParams, s Schema) []string {
	var out []string

	// pushKV 辅助函数：将键值对展开到参数列表
	pushKV := func(keys []string, val string) {
		for _, k := range keys {
			out = append(out, k, val)
		}
	}

	// 遍历 Schema 中的参数映射
	for key, spec := range s.Args {
		switch key {
		case "url":
			// URL 参数：先添加参数名，再添加 URL 值
			if len(spec.ArgsName) > 0 {
				out = append(out, spec.ArgsName...)
			}
			out = append(out, p.URL)

		case "localDir":
			// 本地目录参数：可能需要拼接子文件夹
			final := p.LocalDir
			if p.Folder != "" {
				final = filepath.Join(final, p.Folder)
			}
			pushKV(spec.ArgsName, final)

		case "name":
			// 文件名参数：处理后缀
			name := p.Name
			if spec.Postfix == "@@AUTO@@" {
				// 自动推断扩展名
				name = name + "." + guessExtFromURL(p.URL)
			} else if spec.Postfix != "" {
				// 添加指定后缀
				name = name + spec.Postfix
			}
			pushKV(spec.ArgsName, name)

		case "headers":
			// HTTP 头参数：多值展开
			for _, h := range p.Headers {
				for _, k := range spec.ArgsName {
					out = append(out, k, h)
				}
			}

		case "deleteSegments":
			// 删除分段文件参数：显式传递 true/false
			if p.DeleteSegments {
				pushKV(spec.ArgsName, "true")
			} else {
				pushKV(spec.ArgsName, "false")
			}

		case "proxy":
			// 代理参数：仅在设置时添加
			if p.Proxy != "" {
				pushKV(spec.ArgsName, p.Proxy)
			}

		case "__common__":
			// 通用参数：直接展开
			out = append(out, spec.ArgsName...)
		}
	}

	return out
}

// guessExtFromURL 从 URL 推断文件扩展名
func guessExtFromURL(u string) string {
	l := strings.ToLower(u)
	switch {
	case strings.Contains(l, ".m3u8"):
		return "m3u8"
	case strings.Contains(l, ".mp4"):
		return "mp4"
	case strings.Contains(l, ".flv"):
		return "flv"
	case strings.Contains(l, ".mkv"):
		return "mkv"
	default:
		return "mp4"
	}
}

// Download 执行下载任务
func (d *DownloaderSvc) Download(ctx context.Context, p DownloadParams, cb Callbacks) error {
	// 获取对应下载类型的 Schema
	schema, ok := d.schemas.GetByType(p.Type)
	if !ok {
		return ErrUnsupportedType
	}

	// 获取对应下载类型的可执行文件路径
	bin, ok := d.binMap[p.Type]
	if !ok || bin == "" {
		return ErrBinNotFound
	}

	// 创建控制台行解析器
	lp, err := newLineParser(schema.ConsoleReg)
	if err != nil {
		return err
	}

	// 构建命令行参数
	args := d.buildArgs(p, schema)

	// 初始化解析状态
	st := &parseState{}

	// 逐行处理控制台输出
	onLine := func(line string) {
		line = strings.TrimSpace(line)

		// 发送消息事件
		if cb.OnMessage != nil {
			cb.OnMessage(MessageEvent{ID: p.ID, Message: line})
		}

		// 解析控制台输出
		evt, errStr := lp.parse(line, st)
		_ = errStr // 错误由底层 Wait 返回，这里仅做提示

		// 处理 ready 事件
		if evt == "ready" {
			st.ready = true
			if cb.OnProgress != nil {
				cb.OnProgress(ProgressEvent{
					ID:     p.ID,
					Type:   "ready",
					IsLive: st.isLive,
				})
			}
		}

		// 处理进度更新（应用节流策略）
		if st.ready && (st.percent > 0 || st.speed != "") {
			if cb.OnProgress != nil && d.tracker.shouldUpdate(p.ID, st.percent, st.speed) {
				cb.OnProgress(ProgressEvent{
					ID:      p.ID,
					Type:    "progress",
					Percent: st.percent,
					Speed:   st.speed,
					IsLive:  st.isLive,
				})
				d.tracker.update(p.ID, st.percent, st.speed)
			}
		}
	}

	// 执行命令
	err = d.runner.Run(ctx, bin, args, onLine)

	// 清理进度记录
	d.tracker.remove(p.ID)

	return err
}
