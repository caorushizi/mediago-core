package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caorushizi/mediago-core/internal/model"
)

func TestHTTPDownloader_Download(t *testing.T) {
	// Create a test server serving segment content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "content-of-%s", r.URL.Path)
	}))
	defer server.Close()

	segments := []model.Segment{
		{Index: 0, URL: server.URL + "/seg0.ts"},
		{Index: 1, URL: server.URL + "/seg1.ts"},
		{Index: 2, URL: server.URL + "/seg2.ts"},
	}

	tmpDir := t.TempDir()
	dl := &HTTPDownloader{}

	var lastEvent model.ProgressEvent
	err := dl.Download(context.Background(), segments, Options{
		TmpDir:      tmpDir,
		ThreadCount: 2,
		RetryCount:  1,
	}, func(e model.ProgressEvent) {
		lastEvent = e
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all files exist
	for _, seg := range segments {
		path := SegmentFilePath(tmpDir, seg.Index)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("segment %d file missing: %v", seg.Index, err)
		}
		if len(data) == 0 {
			t.Errorf("segment %d is empty", seg.Index)
		}
	}

	// Verify progress
	if lastEvent.TotalSegments != 3 {
		t.Errorf("expected total 3, got %d", lastEvent.TotalSegments)
	}
	if lastEvent.CompletedSegments != 3 {
		t.Errorf("expected completed 3, got %d", lastEvent.CompletedSegments)
	}
}

func TestHTTPDownloader_ByteRange(t *testing.T) {
	content := "0123456789ABCDEF"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Write([]byte(content))
			return
		}
		// Parse "bytes=start-stop"
		var start, stop int
		fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &stop)
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte(content[start : stop+1]))
	}))
	defer server.Close()

	segments := []model.Segment{
		{Index: 0, URL: server.URL + "/media.mp4", StartRange: 0, StopRange: 7},
		{Index: 1, URL: server.URL + "/media.mp4", StartRange: 8, StopRange: 15},
	}

	tmpDir := t.TempDir()
	dl := &HTTPDownloader{}

	err := dl.Download(context.Background(), segments, Options{
		TmpDir:      tmpDir,
		ThreadCount: 2,
		RetryCount:  1,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data0, _ := os.ReadFile(SegmentFilePath(tmpDir, 0))
	data1, _ := os.ReadFile(SegmentFilePath(tmpDir, 1))

	if string(data0) != "01234567" {
		t.Errorf("expected '01234567', got %q", string(data0))
	}
	if string(data1) != "89ABCDEF" {
		t.Errorf("expected '89ABCDEF', got %q", string(data1))
	}
}

func TestHTTPDownloader_Retry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	segments := []model.Segment{
		{Index: 0, URL: server.URL + "/seg.ts"},
	}

	tmpDir := t.TempDir()
	dl := &HTTPDownloader{}

	err := dl.Download(context.Background(), segments, Options{
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  3,
	}, nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}

	data, _ := os.ReadFile(SegmentFilePath(tmpDir, 0))
	if string(data) != "ok" {
		t.Errorf("expected 'ok', got %q", string(data))
	}
}

func TestHTTPDownloader_CustomHeaders(t *testing.T) {
	var gotReferer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("Referer")
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	segments := []model.Segment{
		{Index: 0, URL: server.URL + "/seg.ts"},
	}

	tmpDir := t.TempDir()
	dl := &HTTPDownloader{}

	err := dl.Download(context.Background(), segments, Options{
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  0,
		Headers:     map[string]string{"Referer": "https://example.com"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotReferer != "https://example.com" {
		t.Errorf("expected referer 'https://example.com', got %q", gotReferer)
	}
}

func TestSpeedTracker(t *testing.T) {
	tracker := NewSpeedTracker()
	defer tracker.Stop()

	tracker.Add(1024)
	tracker.Add(2048)

	// Wait for one tick
	time.Sleep(1100 * time.Millisecond)

	speed := tracker.Speed()
	if speed != 3072 {
		t.Errorf("expected speed 3072, got %d", speed)
	}
}

func TestWithRetry(t *testing.T) {
	var count int
	err := WithRetry(2, func() error {
		count++
		if count < 3 {
			return fmt.Errorf("fail %d", count)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 attempts, got %d", count)
	}
}

func TestSegmentFilePath(t *testing.T) {
	got := SegmentFilePath("/tmp/dl", 42)
	want := filepath.Join("/tmp/dl", "seg_00042")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
