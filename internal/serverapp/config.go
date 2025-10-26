package serverapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"caorushizi.cn/mediago/internal/core"
	"caorushizi.cn/mediago/internal/logger"
)

type appConfig struct {
	Log        logConfig        `json:"log"`
	SchemaPath string           `json:"schemaPath"`
	Queue      queueConfigInput `json:"queue"`
	Binaries   binaryConfig     `json:"binaries"`
}

type logConfig struct {
	Level          string `json:"level"`
	Dir            string `json:"dir"`
	File           string `json:"file"`
	DownloaderFile string `json:"downloaderFile"`
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

func defaultAppConfig() appConfig {
	return appConfig{
		Log: logConfig{
			Level:          "info",
			Dir:            "./logs",
			File:           "mediago.log",
			DownloaderFile: "downloader.log",
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
	if strings.TrimSpace(c.Log.Level) == "" {
		c.Log.Level = "info"
	}
	if strings.TrimSpace(c.Log.Dir) == "" {
		c.Log.Dir = "./logs"
	}
	if strings.TrimSpace(c.Log.File) == "" {
		c.Log.File = "mediago.log"
	}
	if strings.TrimSpace(c.Log.DownloaderFile) == "" {
		c.Log.DownloaderFile = "downloader.log"
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
