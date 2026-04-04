package pipeline

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/caorushizi/mediago-core/internal/crypto"
	"github.com/caorushizi/mediago-core/internal/downloader"
	"github.com/caorushizi/mediago-core/internal/model"
	"github.com/caorushizi/mediago-core/internal/parser"
)

// LiveRecorder handles live stream recording by periodically refreshing
// the playlist and downloading new segments.
type LiveRecorder struct {
	Parser     parser.Parser
	Downloader downloader.Downloader
	Decryptor  *crypto.AES128Decryptor
	Opts       LiveOptions
}

// LiveOptions configures live recording behavior.
type LiveOptions struct {
	MaxDuration time.Duration // 0 = unlimited
	WaitTime    time.Duration // playlist refresh interval, 0 = auto
}

// Record starts live recording. It refreshes the playlist periodically,
// downloads new segments, and appends them to the output.
func (r *LiveRecorder) Record(ctx context.Context, task *model.Task, stream *model.StreamSpec, onProgress func(model.ProgressEvent)) error {
	tmpDir := task.TmpDir
	if tmpDir == "" {
		tmpDir = os.TempDir() + "/mediago_live"
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("create tmp dir: %w", err)
	}

	// Track which segments we've already downloaded by URL
	downloaded := make(map[string]bool)
	var totalDownloaded int
	startTime := time.Now()

	// Determine refresh interval
	refreshInterval := r.Opts.WaitTime
	if refreshInterval == 0 {
		if stream.Playlist != nil && stream.Playlist.TargetDuration > 0 {
			refreshInterval = time.Duration(stream.Playlist.TargetDuration) * time.Second
		} else {
			refreshInterval = 5 * time.Second
		}
	}

	// Apply max duration via context
	if r.Opts.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Opts.MaxDuration)
		defer cancel()
	}

	// Download initial segments
	if stream.Playlist != nil {
		n, err := r.downloadNewSegments(ctx, task, stream.Playlist, tmpDir, downloaded, totalDownloaded, onProgress)
		if err != nil {
			return err
		}
		totalDownloaded += n
	}

	// Refresh loop
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				elapsed := time.Since(startTime).Truncate(time.Second)
				fmt.Printf("\nLive recording duration reached (%s), recorded %d segments\n", elapsed, totalDownloaded)
				return nil
			}
			return ctx.Err()

		case <-ticker.C:
			// Refresh playlist
			result, err := r.Parser.Parse(ctx, stream.URL, task.Headers)
			if err != nil {
				fmt.Printf("\nWarning: refresh failed: %v, retrying...\n", err)
				continue
			}

			if len(result.Streams) == 0 || result.Streams[0].Playlist == nil {
				continue
			}

			playlist := result.Streams[0].Playlist

			// Check if stream ended
			if !playlist.IsLive {
				n, _ := r.downloadNewSegments(ctx, task, playlist, tmpDir, downloaded, totalDownloaded, onProgress)
				totalDownloaded += n
				elapsed := time.Since(startTime).Truncate(time.Second)
				fmt.Printf("\nLive stream ended. Recorded %d segments in %s\n", totalDownloaded, elapsed)
				return nil
			}

			n, err := r.downloadNewSegments(ctx, task, playlist, tmpDir, downloaded, totalDownloaded, onProgress)
			if err != nil {
				fmt.Printf("\nWarning: download failed: %v\n", err)
				continue
			}
			totalDownloaded += n
		}
	}
}

// downloadNewSegments downloads segments that haven't been downloaded yet.
// Returns the number of newly downloaded segments.
func (r *LiveRecorder) downloadNewSegments(ctx context.Context, task *model.Task, playlist *model.Playlist, tmpDir string, downloaded map[string]bool, baseIndex int, onProgress func(model.ProgressEvent)) (int, error) {
	var newSegments []model.Segment

	for _, seg := range playlist.Segments {
		if !downloaded[seg.URL] {
			seg.Index = baseIndex + len(newSegments)
			newSegments = append(newSegments, seg)
		}
	}

	if len(newSegments) == 0 {
		return 0, nil
	}

	err := r.Downloader.Download(ctx, newSegments, downloader.Options{
		TmpDir:      tmpDir,
		Headers:     task.Headers,
		Proxy:       task.Proxy,
		Timeout:     task.Timeout,
		ThreadCount: task.ThreadCount,
		RetryCount:  task.RetryCount,
	}, func(e model.ProgressEvent) {
		if onProgress != nil {
			e.IsLive = true
			e.TotalSegments = baseIndex + e.TotalSegments
			e.CompletedSegments = baseIndex + e.CompletedSegments
			onProgress(e)
		}
	})
	if err != nil {
		return 0, err
	}

	for _, seg := range newSegments {
		downloaded[seg.URL] = true
	}

	return len(newSegments), nil
}
