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

func TestLiveRecorder_BasicRecording(t *testing.T) {
	// Simulate a live stream that produces new segments each refresh
	var sequence atomic.Int32
	sequence.Store(0)

	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		seq := sequence.Load()
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		fmt.Fprintf(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:%d
#EXTINF:2.0,
seg%d.ts
#EXTINF:2.0,
seg%d.ts
`, seq, seq, seq+1)
		sequence.Add(2)
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
			MaxDuration: 3 * time.Second,
			WaitTime:    1 * time.Second,
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
				{Index: 0, URL: server.URL + "/seg_init_0.ts", Duration: 2},
				{Index: 1, URL: server.URL + "/seg_init_1.ts", Duration: 2},
			},
		},
	}

	var progressCount int
	err := recorder.Record(context.Background(), task, stream, func(e model.ProgressEvent) {
		progressCount++
		if !e.IsLive {
			t.Error("expected IsLive=true in progress")
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify some segments were downloaded
	entries, _ := os.ReadDir(tmpDir)
	if len(entries) < 2 {
		t.Errorf("expected at least 2 segment files, got %d", len(entries))
	}
}

func TestLiveRecorder_StreamEnds(t *testing.T) {
	var callCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n >= 3 {
			// Stream ends (VOD-style with ENDLIST)
			fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:2.0,
final0.ts
#EXTINF:2.0,
final1.ts
#EXT-X-ENDLIST
`)
		} else {
			fmt.Fprintf(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:%d
#EXTINF:2.0,
seg%d.ts
`, (n-1)*1, (n-1)*1)
		}
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
			WaitTime: 500 * time.Millisecond,
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
				{Index: 0, URL: server.URL + "/seg_first.ts", Duration: 2},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := recorder.Record(ctx, task, stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have exited because stream ended, not timeout
	if callCount.Load() < 3 {
		t.Errorf("expected at least 3 playlist fetches, got %d", callCount.Load())
	}
}

func TestLiveRecorder_DeduplicatesSegments(t *testing.T) {
	// Return the same segments every time — should not re-download
	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:2.0,
seg0.ts
#EXTINF:2.0,
seg1.ts
`)
	})

	var downloadCount atomic.Int32
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		downloadCount.Add(1)
		w.Write([]byte("data"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()

	recorder := &LiveRecorder{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
		Opts: LiveOptions{
			MaxDuration: 2 * time.Second,
			WaitTime:    500 * time.Millisecond,
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

	err := recorder.Record(context.Background(), task, stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only download each segment once despite multiple refreshes
	count := downloadCount.Load()
	if count != 2 {
		t.Errorf("expected exactly 2 downloads (dedup), got %d", count)
	}
}

func TestParseLiveDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"01:30:00", 90 * time.Minute},
		{"00:05:30", 5*time.Minute + 30*time.Second},
		{"1h30m", 90 * time.Minute},
		{"30s", 30 * time.Second},
	}
	for _, tt := range tests {
		got, err := parseLiveDuration(tt.input)
		if err != nil {
			t.Errorf("parseLiveDuration(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseLiveDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
