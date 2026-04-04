package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/caorushizi/mediago-core/internal/crypto"
	"github.com/caorushizi/mediago-core/internal/downloader"
	"github.com/caorushizi/mediago-core/internal/model"
	"github.com/caorushizi/mediago-core/internal/parser"
	"github.com/caorushizi/mediago-core/internal/parser/dash"
	"github.com/caorushizi/mediago-core/internal/parser/hls"
	"github.com/caorushizi/mediago-core/internal/pipeline"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	task := &model.Task{}
	var headers []string

	rootCmd := &cobra.Command{
		Use:     "mediago [url]",
		Short:   "A streaming media downloader supporting HLS and DASH",
		Version: version,
		Args:    cobra.ExactArgs(1),
		Long: `mediago - Streaming media downloader

Supports HLS (m3u8) and DASH (mpd) protocols with concurrent segment
downloading, AES-128 decryption, and automatic merging.

DISCLAIMER: This software is for educational and research purposes only.
Users are responsible for ensuring compliance with applicable laws.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			task.URL = args[0]
			task.Headers = parseHeaders(headers)
			return run(task)
		},
	}

	f := rootCmd.Flags()

	// Basic
	f.StringVarP(&task.SaveDir, "save-dir", "d", ".", "Save directory")
	f.StringVarP(&task.SaveName, "save-name", "n", "", "Output filename (without extension)")
	f.StringVar(&task.TmpDir, "tmp-dir", "", "Temporary directory for segments")

	// Network
	f.StringArrayVarP(&headers, "header", "H", nil, "Custom HTTP header (can be specified multiple times)")
	f.StringVar(&task.Proxy, "proxy", "", "Proxy URL (http/socks5)")
	f.IntVar(&task.Timeout, "timeout", 30, "HTTP request timeout in seconds")

	// Download
	f.IntVarP(&task.ThreadCount, "thread-count", "t", 8, "Concurrent download threads")
	f.IntVarP(&task.RetryCount, "retry-count", "r", 3, "Retry count per segment")

	// Stream selection
	f.BoolVar(&task.AutoSelect, "auto-select", false, "Auto-select best quality")
	f.StringVar(&task.SelectVideo, "select-video", "", "Video stream filter")
	f.StringVar(&task.SelectAudio, "select-audio", "", "Audio stream filter")

	// Merge
	f.BoolVar(&task.NoMerge, "no-merge", false, "Download only, skip merge")
	f.BoolVar(&task.DelAfterDone, "del-after-done", true, "Delete temp files after merge")
	f.StringVar(&task.FfmpegPath, "ffmpeg-path", "ffmpeg", "Path to ffmpeg binary")
	f.BoolVar(&task.BinaryMerge, "binary-merge", false, "Force binary concatenation")

	// Decrypt
	f.StringArrayVar(&task.Key, "key", nil, "Decryption key in HEX (can be specified multiple times)")
	f.StringVar(&task.CustomHLSMethod, "custom-hls-method", "", "Force HLS encryption method")
	f.StringVar(&task.CustomHLSKey, "custom-hls-key", "", "Force HLS key (HEX)")
	f.StringVar(&task.CustomHLSIV, "custom-hls-iv", "", "Force HLS IV (HEX)")

	// Live
	f.BoolVar(&task.Live, "live", false, "Force live mode")
	f.StringVar(&task.LiveDuration, "live-duration", "", "Recording duration (HH:mm:ss)")
	f.IntVar(&task.LiveWaitTime, "live-wait-time", 0, "Playlist refresh interval in seconds")

	// Output
	f.StringVar(&task.LogLevel, "log-level", "info", "Log level (debug/info/warn/error)")
	f.BoolVar(&task.NoLog, "no-log", false, "Disable logging")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(task *model.Task) error {
	// Detect stream type and select parser
	var p parser.Parser
	switch parser.DetectType(task.URL) {
	case parser.StreamDASH:
		p = &dash.Parser{}
	default:
		p = &hls.Parser{}
	}

	pipe := &pipeline.Pipeline{
		Parser:     p,
		Downloader: &downloader.HTTPDownloader{},
		Decryptor:  &crypto.AES128Decryptor{},
	}

	ctx := context.Background()

	fmt.Printf("Downloading: %s\n", task.URL)
	if task.SaveName != "" {
		fmt.Printf("Save as: %s\n", task.SaveName)
	}

	err := pipe.Run(ctx, task, func(e model.ProgressEvent) {
		fmt.Printf("\r[%d/%d] %.1f%% | %s",
			e.CompletedSegments, e.TotalSegments, e.Percent, formatSpeed(e.Speed))
	})
	if err != nil {
		return err
	}

	fmt.Println("\nDone!")
	return nil
}

// parseHeaders converts ["Key: Value", ...] to map[string]string.
func parseHeaders(raw []string) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	m := make(map[string]string, len(raw))
	for _, h := range raw {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}

// formatSpeed formats bytes/sec to human-readable string.
func formatSpeed(bytesPerSec int64) string {
	switch {
	case bytesPerSec >= 1024*1024:
		return fmt.Sprintf("%.1f MB/s", float64(bytesPerSec)/(1024*1024))
	case bytesPerSec >= 1024:
		return fmt.Sprintf("%.1f KB/s", float64(bytesPerSec)/1024)
	default:
		return fmt.Sprintf("%d B/s", bytesPerSec)
	}
}

func init() {
	log.SetFlags(0)
}
