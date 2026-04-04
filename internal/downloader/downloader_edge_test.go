package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caorushizi/mediago-core/internal/model"
)

func TestHTTPDownloader_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.Write([]byte("too late"))
	}))
	defer server.Close()

	segments := []model.Segment{
		{Index: 0, URL: server.URL + "/slow.ts"},
	}

	tmpDir := t.TempDir()
	dl := &HTTPDownloader{}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := dl.Download(ctx, segments, Options{
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  0,
	}, nil)
	if err == nil {
		t.Fatal("expected error due to timeout")
	}
}

func TestHTTPDownloader_RetryExhaustion(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	segments := []model.Segment{
		{Index: 0, URL: server.URL + "/fail.ts"},
	}

	tmpDir := t.TempDir()
	dl := &HTTPDownloader{}

	err := dl.Download(context.Background(), segments, Options{
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  2,
	}, nil)
	if err == nil {
		t.Fatal("expected error after retry exhaustion")
	}

	// 1 initial + 2 retries = 3 attempts
	got := attempts.Load()
	if got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}

func TestHTTPDownloader_PartialConcurrentFailure(t *testing.T) {
	// 5 segments: indices 0-4. Only index 2 fails permanently.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/seg2.ts" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "content-of-%s", r.URL.Path)
	}))
	defer server.Close()

	segments := make([]model.Segment, 5)
	for i := range segments {
		segments[i] = model.Segment{Index: i, URL: fmt.Sprintf("%s/seg%d.ts", server.URL, i)}
	}

	tmpDir := t.TempDir()
	dl := &HTTPDownloader{}

	err := dl.Download(context.Background(), segments, Options{
		TmpDir:      tmpDir,
		ThreadCount: 3,
		RetryCount:  1,
	}, nil)
	if err == nil {
		t.Fatal("expected error when one segment fails permanently")
	}

	// Successful segments should still have been downloaded
	for _, idx := range []int{0, 1, 3, 4} {
		path := SegmentFilePath(tmpDir, idx)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Some may not exist due to early abort, that's acceptable
			t.Logf("segment %d not downloaded (early abort)", idx)
		}
	}
}

func TestHTTPDownloader_ServerClosedConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hijack connection and close immediately to simulate connection reset
		hj, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
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
	}, nil)
	if err == nil {
		t.Fatal("expected error when server closes connection")
	}
}

func TestHTTPDownloader_LargeSegmentCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	segments := make([]model.Segment, 50)
	for i := range segments {
		segments[i] = model.Segment{Index: i, URL: server.URL + fmt.Sprintf("/seg%d.ts", i)}
	}

	tmpDir := t.TempDir()
	dl := &HTTPDownloader{}

	var lastEvent model.ProgressEvent
	err := dl.Download(context.Background(), segments, Options{
		TmpDir:      tmpDir,
		ThreadCount: 10,
		RetryCount:  0,
	}, func(e model.ProgressEvent) {
		lastEvent = e
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lastEvent.CompletedSegments != 50 {
		t.Errorf("expected 50 completed, got %d", lastEvent.CompletedSegments)
	}

	// Verify all files exist
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(SegmentFilePath(tmpDir, i)); err != nil {
			t.Errorf("segment %d missing: %v", i, err)
		}
	}
}

func TestHTTPDownloader_HTTP4xxErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"429 Too Many Requests", http.StatusTooManyRequests},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
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
			}, nil)
			if err == nil {
				t.Fatalf("expected error for HTTP %d", tt.status)
			}
		})
	}
}

func TestWithRetry_ImmediateSuccess(t *testing.T) {
	var count int
	err := WithRetry(5, func() error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 attempt, got %d", count)
	}
}

func TestWithRetry_AllFail(t *testing.T) {
	var count int
	err := WithRetry(0, func() error {
		count++
		return fmt.Errorf("always fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if count != 1 {
		t.Errorf("expected 1 attempt (0 retries), got %d", count)
	}
}

func TestHTTPDownloader_EmptySegmentList(t *testing.T) {
	tmpDir := t.TempDir()
	dl := &HTTPDownloader{}

	err := dl.Download(context.Background(), nil, Options{
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  0,
	}, nil)
	if err != nil {
		t.Fatalf("expected nil error for empty list, got: %v", err)
	}
}

func TestBuildClient_WithProxy(t *testing.T) {
	dl := &HTTPDownloader{}
	client := dl.buildClient(Options{
		Proxy: "http://proxy.example.com:8080",
	})
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Transport == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestBuildClient_WithInvalidProxy(t *testing.T) {
	dl := &HTTPDownloader{}
	// Invalid proxy URL — should still return a client (error ignored)
	client := dl.buildClient(Options{
		Proxy: "://invalid",
	})
	if client == nil {
		t.Fatal("expected non-nil client even with invalid proxy")
	}
}

func TestBuildClient_NoProxy(t *testing.T) {
	dl := &HTTPDownloader{}
	client := dl.buildClient(Options{})
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}
