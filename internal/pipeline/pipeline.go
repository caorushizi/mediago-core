package pipeline

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/caorushizi/mediago-core/internal/crypto"
	"github.com/caorushizi/mediago-core/internal/downloader"
	"github.com/caorushizi/mediago-core/internal/merger"
	"github.com/caorushizi/mediago-core/internal/model"
	"github.com/caorushizi/mediago-core/internal/parser"
)

// Pipeline orchestrates the full download flow: parse → download → decrypt → merge → cleanup.
type Pipeline struct {
	Parser     parser.Parser
	Downloader downloader.Downloader
	Decryptor  *crypto.AES128Decryptor
	Merger     merger.Merger
	OnLog      func(format string, args ...any) // nil = silent
}

func (p *Pipeline) logf(format string, args ...any) {
	if p.OnLog != nil {
		p.OnLog(format, args...)
	}
}

// formatSpeed formats bytes/sec to human-readable string.
func formatSpeed(bytesPerSec int64) string {
	switch {
	case bytesPerSec >= 1024*1024:
		return fmt.Sprintf("%.1fMB/s", float64(bytesPerSec)/(1024*1024))
	case bytesPerSec >= 1024:
		return fmt.Sprintf("%.1fKB/s", float64(bytesPerSec)/1024)
	default:
		return fmt.Sprintf("%dB/s", bytesPerSec)
	}
}

// mediaTypeName returns a human-readable name for a MediaType.
func mediaTypeName(mt model.MediaType) string {
	switch mt {
	case model.MediaVideo:
		return "video"
	case model.MediaAudio:
		return "audio"
	case model.MediaSubtitle:
		return "subtitle"
	default:
		return "unknown"
	}
}

// Run executes the full pipeline for a given task.
func (p *Pipeline) Run(ctx context.Context, task *model.Task, onProgress func(model.ProgressEvent)) error {
	// 1. Parse
	p.logf("[parse] url=%s", task.URL)
	result, err := p.Parser.Parse(ctx, task.URL, task.Headers)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	p.logf("[parse] streams: %d, merge_type: %d, is_live: %v", len(result.Streams), result.MergeType, result.IsLive)
	for i, s := range result.Streams {
		segCount := 0
		hasInit := false
		if s.Playlist != nil {
			segCount = len(s.Playlist.Segments)
			hasInit = s.Playlist.MediaInit != nil
		}
		p.logf("[parse]   stream[%d]: type=%s bandwidth=%d segments=%d has_init=%v", i, mediaTypeName(s.MediaType), s.Bandwidth, segCount, hasInit)
	}

	// 2. Select streams
	streams := selectStreams(result.Streams, task)
	if len(streams) == 0 {
		return fmt.Errorf("no streams selected")
	}

	if task.AutoSelect && len(result.Streams) > 1 {
		for i, s := range streams {
			p.logf("[select] auto_select: stream[%d] type=%s (bandwidth=%d)", i, mediaTypeName(s.MediaType), s.Bandwidth)
		}
	}

	// 3. Live recording mode
	if result.IsLive || task.Live {
		p.logf("[live] starting live recording")
		return p.runLive(ctx, task, &streams[0], onProgress)
	}

	// 4. Process each selected stream
	for i, stream := range streams {
		if stream.Playlist == nil || len(stream.Playlist.Segments) == 0 {
			continue
		}

		outputName := task.SaveName
		if outputName == "" {
			outputName = "output"
		}
		// Append stream type suffix for multi-stream
		if len(streams) > 1 {
			switch stream.MediaType {
			case model.MediaAudio:
				outputName += "_audio"
			case model.MediaSubtitle:
				outputName += "_sub"
			default:
				if i > 0 {
					outputName += fmt.Sprintf("_%d", i)
				}
			}
		}

		if err := p.processStream(ctx, task, &stream, result.MergeType, outputName, onProgress); err != nil {
			return fmt.Errorf("stream %d: %w", i, err)
		}
	}

	return nil
}

func (p *Pipeline) processStream(ctx context.Context, task *model.Task, stream *model.StreamSpec, mergeType model.MergeType, outputName string, onProgress func(model.ProgressEvent)) error {
	playlist := stream.Playlist

	// Setup tmp dir
	tmpDir := task.TmpDir
	if tmpDir == "" {
		tmpDir = filepath.Join(os.TempDir(), "mediago", outputName)
	}

	// Download init segment if present (fMP4)
	if playlist.MediaInit != nil {
		p.logf("[download] init segment")
		initSegs := []model.Segment{*playlist.MediaInit}
		err := p.Downloader.Download(ctx, initSegs, downloader.Options{
			TmpDir:      tmpDir,
			Headers:     task.Headers,
			Proxy:       task.Proxy,
			Timeout:     task.Timeout,
			ThreadCount: 1,
			RetryCount:  task.RetryCount,
		}, nil)
		if err != nil {
			return fmt.Errorf("download init segment: %w", err)
		}
	}

	// Download segments
	p.logf("[download] %d segments, thread_count=%d", len(playlist.Segments), task.ThreadCount)
	err := p.Downloader.Download(ctx, playlist.Segments, downloader.Options{
		TmpDir:      tmpDir,
		Headers:     task.Headers,
		Proxy:       task.Proxy,
		Timeout:     task.Timeout,
		ThreadCount: task.ThreadCount,
		RetryCount:  task.RetryCount,
	}, func(e model.ProgressEvent) {
		p.logf("[download] progress: %d/%d (%.1f%%) speed=%s", e.CompletedSegments, e.TotalSegments, e.Percent, formatSpeed(e.Speed))
		if onProgress != nil {
			onProgress(e)
		}
	})
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	p.logf("[download] complete: %d segments", len(playlist.Segments))

	// Decrypt if needed
	if err := p.decryptSegments(ctx, task, playlist, tmpDir); err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	// Merge
	if !task.NoMerge {
		if err := p.mergeSegments(ctx, task, playlist, mergeType, tmpDir, outputName); err != nil {
			return fmt.Errorf("merge: %w", err)
		}
	}

	// Cleanup
	if task.DelAfterDone && !task.NoMerge {
		os.RemoveAll(tmpDir)
		p.logf("[cleanup] removed tmp dir")
	}

	return nil
}

func (p *Pipeline) decryptSegments(ctx context.Context, task *model.Task, playlist *model.Playlist, tmpDir string) error {
	if p.Decryptor == nil {
		return nil
	}

	// Count encrypted segments
	encCount := 0
	for _, seg := range playlist.Segments {
		if seg.EncryptInfo != nil && seg.EncryptInfo.Method != model.EncryptNone {
			encCount++
		}
	}
	if encCount > 0 {
		p.logf("[decrypt] %d segments, method=AES-128", encCount)
	}

	for _, seg := range playlist.Segments {
		if seg.EncryptInfo == nil || seg.EncryptInfo.Method == model.EncryptNone {
			continue
		}

		key := seg.EncryptInfo.Key
		// If key not yet fetched, download it
		if key == nil && seg.EncryptInfo.KeyURL != "" {
			var err error
			key, err = fetchKey(ctx, seg.EncryptInfo.KeyURL, task.Headers)
			if err != nil {
				return fmt.Errorf("fetch key for segment %d: %w", seg.Index, err)
			}
			seg.EncryptInfo.Key = key
		}

		if key == nil {
			continue
		}

		segPath := downloader.SegmentFilePath(tmpDir, seg.Index)
		data, err := os.ReadFile(segPath)
		if err != nil {
			return fmt.Errorf("read segment %d: %w", seg.Index, err)
		}

		decrypted, err := p.Decryptor.Decrypt(data, key, seg.EncryptInfo.IV)
		if err != nil {
			return fmt.Errorf("decrypt segment %d: %w", seg.Index, err)
		}

		if err := os.WriteFile(segPath, decrypted, 0o644); err != nil {
			return fmt.Errorf("write decrypted segment %d: %w", seg.Index, err)
		}
	}

	return nil
}

func (p *Pipeline) mergeSegments(ctx context.Context, task *model.Task, playlist *model.Playlist, mergeType model.MergeType, tmpDir string, outputName string) error {
	// Build ordered file list
	var files []string

	// Init segment first (for fMP4)
	if playlist.MediaInit != nil {
		initPath := downloader.SegmentFilePath(tmpDir, playlist.MediaInit.Index)
		files = append(files, initPath)
	}

	// Sort segments by index
	sorted := make([]model.Segment, len(playlist.Segments))
	copy(sorted, playlist.Segments)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Index < sorted[j].Index
	})

	for _, seg := range sorted {
		files = append(files, downloader.SegmentFilePath(tmpDir, seg.Index))
	}

	// Determine output extension and merger
	var m merger.Merger
	var ext string
	var mergeTypeName string

	if task.BinaryMerge || mergeType == model.MergeBinary {
		m = &merger.BinaryMerger{}
		ext = ".mp4"
		mergeTypeName = "binary"
	} else {
		m = &merger.FFmpegMerger{FFmpegPath: task.FfmpegPath}
		ext = ".mp4"
		mergeTypeName = "ffmpeg"
	}

	p.logf("[merge] type=%s, files=%d", mergeTypeName, len(files))

	saveDir := task.SaveDir
	if saveDir == "" {
		saveDir = "."
	}
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		return fmt.Errorf("create save dir: %w", err)
	}

	output := filepath.Join(saveDir, outputName+ext)

	if err := m.Merge(ctx, files, output); err != nil {
		return err
	}

	// Log output file size
	if info, statErr := os.Stat(output); statErr == nil {
		p.logf("[merge] output: %s (%d bytes)", outputName+ext, info.Size())
	}

	return nil
}

// runLive handles live stream recording.
func (p *Pipeline) runLive(ctx context.Context, task *model.Task, stream *model.StreamSpec, onProgress func(model.ProgressEvent)) error {
	opts := LiveOptions{}

	if task.LiveDuration != "" {
		d, err := parseLiveDuration(task.LiveDuration)
		if err != nil {
			return fmt.Errorf("parse live-duration: %w", err)
		}
		opts.MaxDuration = d
	}

	if task.LiveWaitTime > 0 {
		opts.WaitTime = time.Duration(task.LiveWaitTime) * time.Second
	}

	recorder := &LiveRecorder{
		Parser:     p.Parser,
		Downloader: p.Downloader,
		Decryptor:  p.Decryptor,
		Opts:       opts,
	}

	return recorder.Record(ctx, task, stream, onProgress)
}

// parseLiveDuration parses "HH:mm:ss" or "1h30m" style duration.
func parseLiveDuration(s string) (time.Duration, error) {
	// Try Go duration first (e.g. "1h30m")
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// Try HH:mm:ss
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
	}
	return 0, fmt.Errorf("invalid duration format: %s (use HH:mm:ss or Go duration like 1h30m)", s)
}

// selectStreams filters streams based on user selection.
func selectStreams(streams []model.StreamSpec, task *model.Task) []model.StreamSpec {
	if len(streams) == 0 {
		return nil
	}

	// Single stream, no selection needed
	if len(streams) == 1 {
		return streams
	}

	// Auto-select: pick highest bandwidth video + first audio
	if task.AutoSelect {
		return autoSelect(streams)
	}

	// TODO: implement --select-video / --select-audio filtering

	// Default: return all streams
	return streams
}

// autoSelect picks the best video (highest bandwidth) and first audio stream.
func autoSelect(streams []model.StreamSpec) []model.StreamSpec {
	var videos, audios []model.StreamSpec
	for _, s := range streams {
		switch s.MediaType {
		case model.MediaVideo:
			videos = append(videos, s)
		case model.MediaAudio:
			audios = append(audios, s)
		}
	}

	var selected []model.StreamSpec

	if len(videos) > 0 {
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].Bandwidth > videos[j].Bandwidth
		})
		selected = append(selected, videos[0])
	}

	if len(audios) > 0 {
		selected = append(selected, audios[0])
	}

	return selected
}

// fetchKey downloads an encryption key from a URL.
func fetchKey(ctx context.Context, keyURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, keyURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching key", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
