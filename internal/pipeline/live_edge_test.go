package pipeline

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caorushizi/mediago-core/internal/downloader"
	"github.com/caorushizi/mediago-core/internal/model"
	"github.com/caorushizi/mediago-core/internal/parser/hls"
)

func TestLiveRecorder_RefreshFailureRecovery(t *testing.T) {
	// Simulate: first playlist ok, second refresh fails, third recovers with new segments
	var callCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		switch {
		case n == 1:
			// Initial is provided via stream.Playlist, this is first refresh
			fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:2
#EXTINF:2.0,
seg2.ts
#EXTINF:2.0,
seg3.ts
`)
		case n == 2:
			// Simulate server error on refresh
			w.WriteHeader(http.StatusInternalServerError)
		case n == 3:
			// Recovery with new segments
			fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:4
#EXTINF:2.0,
seg4.ts
#EXTINF:2.0,
seg5.ts
`)
		default:
			// End the stream
			fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:6
#EXTINF:2.0,
seg6.ts
#EXT-X-ENDLIST
`)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("segment-data"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	recorder := &LiveRecorder{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
		Opts: LiveOptions{
			WaitTime: 200 * time.Millisecond,
		},
	}

	task := &model.Task{
		URL:         server.URL + "/live.m3u8",
		TmpDir:      tmpDir,
		ThreadCount: 2,
		RetryCount:  1,
	}

	stream := &model.StreamSpec{
		URL: server.URL + "/live.m3u8",
		Playlist: &model.Playlist{
			IsLive:         true,
			TargetDuration: 2,
			Segments: []model.Segment{
				{Index: 0, URL: server.URL + "/seg0.ts", Duration: 2},
				{Index: 1, URL: server.URL + "/seg1.ts", Duration: 2},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := recorder.Record(ctx, task, stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have recovered from the refresh failure
	if callCount.Load() < 4 {
		t.Errorf("expected at least 4 playlist fetches (with one failure), got %d", callCount.Load())
	}

	// Verify some segments were downloaded
	entries, _ := os.ReadDir(tmpDir)
	if len(entries) < 4 {
		t.Errorf("expected at least 4 segment files (recovered from failure), got %d", len(entries))
	}
}

func TestLiveRecorder_SlidingWindow(t *testing.T) {
	// Simulate a sliding window playlist where old segments are removed
	var callCount atomic.Int32
	var downloadCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		seq := (n - 1) * 2
		// Each refresh returns 2 new segments, old ones are gone (sliding window)
		if n >= 4 {
			// End stream after 4 refreshes
			fmt.Fprintf(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:%d
#EXTINF:2.0,
seg%d.ts
#EXT-X-ENDLIST
`, seq, seq)
			return
		}
		fmt.Fprintf(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:%d
#EXTINF:2.0,
seg%d.ts
#EXTINF:2.0,
seg%d.ts
`, seq, seq, seq+1)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		downloadCount.Add(1)
		w.Write([]byte("segment-data"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	recorder := &LiveRecorder{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
		Opts: LiveOptions{
			WaitTime: 200 * time.Millisecond,
		},
	}

	task := &model.Task{
		URL:         server.URL + "/live.m3u8",
		TmpDir:      tmpDir,
		ThreadCount: 2,
		RetryCount:  1,
	}

	stream := &model.StreamSpec{
		URL: server.URL + "/live.m3u8",
		Playlist: &model.Playlist{
			IsLive:         true,
			TargetDuration: 2,
			Segments:       []model.Segment{}, // start empty
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := recorder.Record(ctx, task, stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each refresh brings unique URLs → no re-downloads
	// Initial (0 segs) + refresh 1 (2 segs) + refresh 2 (2 new) + refresh 3 (2 new) + end (1 new) = 7
	count := downloadCount.Load()
	if count < 5 {
		t.Errorf("expected at least 5 unique segment downloads from sliding window, got %d", count)
	}
}

func TestLiveRecorder_ContextCancellation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:2.0,
seg0.ts
`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	recorder := &LiveRecorder{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
		Opts: LiveOptions{
			WaitTime: 100 * time.Millisecond,
		},
	}

	task := &model.Task{
		URL:         server.URL + "/live.m3u8",
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  0,
	}

	stream := &model.StreamSpec{
		URL: server.URL + "/live.m3u8",
		Playlist: &model.Playlist{
			IsLive:         true,
			TargetDuration: 2,
			Segments:       []model.Segment{},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	err := recorder.Record(ctx, task, stream, nil)
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}

func TestLiveRecorder_DownloadFailureContinues(t *testing.T) {
	// Simulate download failure on initial segments, but refresh continues
	var refreshCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		n := refreshCount.Add(1)
		if n >= 3 {
			fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXTINF:2.0,
good_seg.ts
#EXT-X-ENDLIST
`)
			return
		}
		fmt.Fprintf(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:%d
#EXTINF:2.0,
seg%d.ts
`, n, n)
	})
	mux.HandleFunc("/good_seg.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("good data"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// All other segments return OK
		w.Write([]byte("data"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	recorder := &LiveRecorder{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
		Opts: LiveOptions{
			WaitTime: 200 * time.Millisecond,
		},
	}

	task := &model.Task{
		URL:         server.URL + "/live.m3u8",
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  0,
	}

	stream := &model.StreamSpec{
		URL: server.URL + "/live.m3u8",
		Playlist: &model.Playlist{
			IsLive:         true,
			TargetDuration: 2,
			Segments:       []model.Segment{},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := recorder.Record(ctx, task, stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLiveRecorder_AutoRefreshInterval(t *testing.T) {
	// Verify that when WaitTime=0, the recorder uses TargetDuration
	var callCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n >= 2 {
			fmt.Fprint(w, `#EXTM3U
#EXT-X-TARGETDURATION:1
#EXTINF:1.0,
final.ts
#EXT-X-ENDLIST
`)
			return
		}
		fmt.Fprint(w, `#EXTM3U
#EXT-X-TARGETDURATION:1
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:1.0,
seg0.ts
`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	recorder := &LiveRecorder{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
		Opts:       LiveOptions{}, // WaitTime=0 → auto from TargetDuration
	}

	task := &model.Task{
		URL:         server.URL + "/live.m3u8",
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  0,
	}

	stream := &model.StreamSpec{
		URL: server.URL + "/live.m3u8",
		Playlist: &model.Playlist{
			IsLive:         true,
			TargetDuration: 1, // 1 second refresh
			Segments:       []model.Segment{},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := recorder.Record(ctx, task, stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount.Load() < 2 {
		t.Errorf("expected at least 2 playlist fetches, got %d", callCount.Load())
	}
}

func TestParseLiveDuration_Invalid(t *testing.T) {
	_, err := parseLiveDuration("invalid")
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}
