package hls

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caorushizi/mediago-core/internal/model"
)

func TestParser_Parse_MasterPlaylist(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		fmt.Fprint(w, `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360
low/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1600000,RESOLUTION=1280x720
high/index.m3u8
`)
	})
	mux.HandleFunc("/low/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXTINF:2.0,
seg0.ts
#EXTINF:2.0,
seg1.ts
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/high/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXTINF:2.0,
seg0.ts
#EXT-X-ENDLIST
`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := &Parser{Client: server.Client()}
	result, err := p.Parse(context.Background(), server.URL+"/master.m3u8", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(result.Streams))
	}
	if result.Streams[0].Playlist == nil {
		t.Fatal("expected playlist for stream 0")
	}
	if len(result.Streams[0].Playlist.Segments) != 2 {
		t.Errorf("expected 2 segments in low, got %d", len(result.Streams[0].Playlist.Segments))
	}
	if len(result.Streams[1].Playlist.Segments) != 1 {
		t.Errorf("expected 1 segment in high, got %d", len(result.Streams[1].Playlist.Segments))
	}
}

func TestParser_Parse_SingleMediaPlaylist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
seg0.ts
#EXTINF:10.0,
seg1.ts
#EXT-X-ENDLIST
`)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	result, err := p.Parse(context.Background(), server.URL+"/playlist.m3u8", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(result.Streams))
	}
	if result.Streams[0].MediaType != model.MediaVideo {
		t.Error("expected video media type")
	}
	if result.MergeType != model.MergeFFmpeg {
		t.Error("expected FFmpeg merge type for TS segments")
	}
}

func TestParser_Parse_fMP4_DetectsBinaryMerge(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/playlist.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:10
#EXT-X-MAP:URI="init.mp4"
#EXTINF:10.0,
seg0.m4s
#EXT-X-ENDLIST
`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := &Parser{Client: server.Client()}
	result, err := p.Parse(context.Background(), server.URL+"/playlist.m3u8", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MergeType != model.MergeBinary {
		t.Errorf("expected Binary merge for fMP4, got %d", result.MergeType)
	}
}

func TestParser_Parse_HTTP404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	_, err := p.Parse(context.Background(), server.URL+"/missing.m3u8", nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestParser_Parse_HTTP403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	_, err := p.Parse(context.Background(), server.URL+"/forbidden.m3u8", nil)
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestParser_Parse_HTTP502(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	_, err := p.Parse(context.Background(), server.URL+"/bad.m3u8", nil)
	if err == nil {
		t.Fatal("expected error for 502")
	}
}

func TestParser_Parse_MediaPlaylistFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000
low/index.m3u8
`)
	})
	mux.HandleFunc("/low/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := &Parser{Client: server.Client()}
	_, err := p.Parse(context.Background(), server.URL+"/master.m3u8", nil)
	if err == nil {
		t.Fatal("expected error when media playlist fetch fails")
	}
}

func TestParser_Parse_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		fmt.Fprint(w, `#EXTM3U`)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Parse(ctx, server.URL+"/slow.m3u8", nil)
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}

func TestParser_Parse_CustomHeaders(t *testing.T) {
	var gotReferer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("Referer")
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
seg0.ts
#EXT-X-ENDLIST
`)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	headers := map[string]string{"Referer": "https://example.com"}
	_, err := p.Parse(context.Background(), server.URL+"/playlist.m3u8", headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotReferer != "https://example.com" {
		t.Errorf("expected referer 'https://example.com', got %q", gotReferer)
	}
}

func TestParser_Parse_LivePlaylist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:5
#EXT-X-MEDIA-SEQUENCE:100
#EXTINF:5.0,
seg100.ts
#EXTINF:5.0,
seg101.ts
`)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	result, err := p.Parse(context.Background(), server.URL+"/live.m3u8", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsLive {
		t.Error("expected live playlist")
	}
}

func TestParser_Parse_ServerDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close() // Close immediately

	p := &Parser{}
	_, err := p.Parse(context.Background(), server.URL+"/playlist.m3u8", nil)
	if err == nil {
		t.Fatal("expected error when server is down")
	}
}
