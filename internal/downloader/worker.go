package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/caorushizi/mediago-core/internal/model"
)

// HTTPDownloader implements the Downloader interface using net/http.
type HTTPDownloader struct {
}

// Download downloads all segments concurrently.
func (d *HTTPDownloader) Download(ctx context.Context, segments []model.Segment, opts Options, onProgress func(model.ProgressEvent)) error {
	if err := os.MkdirAll(opts.TmpDir, 0o755); err != nil {
		return fmt.Errorf("create tmp dir: %w", err)
	}

	client := d.buildClient(opts)
	tracker := NewSpeedTracker()
	defer tracker.Stop()

	total := len(segments)
	var completed atomic.Int32

	// Semaphore for concurrency control
	sem := make(chan struct{}, opts.ThreadCount)
	var wg sync.WaitGroup
	var firstErr atomic.Value

	for i := range segments {
		seg := &segments[i]
		wg.Add(1)

		go func() {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// Check if already failed
			if firstErr.Load() != nil {
				return
			}

			outPath := SegmentFilePath(opts.TmpDir, seg.Index)
			err := WithRetry(opts.RetryCount, func() error {
				return d.downloadSegment(ctx, client, seg, outPath, opts.Headers, tracker)
			})
			if err != nil {
				firstErr.CompareAndSwap(nil, err)
				return
			}

			n := completed.Add(1)
			if onProgress != nil {
				onProgress(model.ProgressEvent{
					TotalSegments:     total,
					CompletedSegments: int(n),
					Percent:           float64(n) / float64(total) * 100,
					Speed:             tracker.Speed(),
				})
			}
		}()
	}

	wg.Wait()

	if err, ok := firstErr.Load().(error); ok && err != nil {
		return err
	}
	return nil
}

// downloadSegment downloads a single segment to a file.
func (d *HTTPDownloader) downloadSegment(ctx context.Context, client *http.Client, seg *model.Segment, outPath string, headers map[string]string, tracker *SpeedTracker) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, seg.URL, nil)
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Byte-range request
	if seg.HasRange() {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", seg.StartRange, seg.StopRange))
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("HTTP %d for segment %d", resp.StatusCode, seg.Index)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			tracker.Add(int64(n))
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	return nil
}

// buildClient creates an http.Client with the given options.
func (d *HTTPDownloader) buildClient(opts Options) *http.Client {
	transport := &http.Transport{}

	if opts.Proxy != "" {
		proxyURL, err := url.Parse(opts.Proxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   0, // per-segment timeout handled via context
	}
}

// SegmentFilePath returns the file path for a segment in the tmp directory.
func SegmentFilePath(tmpDir string, index int) string {
	return filepath.Join(tmpDir, fmt.Sprintf("seg_%05d", index))
}
