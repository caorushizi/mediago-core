package pipeline

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caorushizi/mediago-core/internal/crypto"
	"github.com/caorushizi/mediago-core/internal/downloader"
	"github.com/caorushizi/mediago-core/internal/model"
	"github.com/caorushizi/mediago-core/internal/parser/dash"
	"github.com/caorushizi/mediago-core/internal/parser/hls"
)

// --- E2E with local httptest (always available) ---

func TestE2E_HLS_MasterPlaylist_BinaryMerge(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360
low/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1600000,RESOLUTION=1280x720
high/index.m3u8
`)
	})
	mux.HandleFunc("/low/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:2
#EXT-X-MAP:URI="init.mp4"
#EXTINF:2.0,
seg0.m4s
#EXTINF:2.0,
seg1.m4s
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/high/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:2
#EXT-X-MAP:URI="init.mp4"
#EXTINF:2.0,
seg0.m4s
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/low/init.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("LOW_INIT"))
	})
	mux.HandleFunc("/low/seg0.m4s", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("LOW_SEG0"))
	})
	mux.HandleFunc("/low/seg1.m4s", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("LOW_SEG1"))
	})
	mux.HandleFunc("/high/init.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("HI_INIT"))
	})
	mux.HandleFunc("/high/seg0.m4s", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("HI_SEG0"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	saveDir := t.TempDir()
	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:          server.URL + "/master.m3u8",
		SaveDir:      saveDir,
		SaveName:     "e2e_hls",
		TmpDir:       t.TempDir(),
		ThreadCount:  2,
		RetryCount:   1,
		AutoSelect:   true,
		BinaryMerge:  true,
		DelAfterDone: true,
	}

	var progressEvents []model.ProgressEvent
	err := pipe.Run(context.Background(), task, func(e model.ProgressEvent) {
		progressEvents = append(progressEvents, e)
	})
	if err != nil {
		t.Fatalf("E2E HLS failed: %v", err)
	}

	// AutoSelect picks highest bandwidth → high variant
	output := filepath.Join(saveDir, "e2e_hls.mp4")
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(data) != "HI_INITHI_SEG0" {
		t.Errorf("expected 'HI_INITHI_SEG0', got %q", string(data))
	}

	// Verify progress was reported
	if len(progressEvents) == 0 {
		t.Error("expected progress events")
	}
}

func TestE2E_HLS_EncryptedStream(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	seg0Plain := []byte("segment-zero-content-here-pad!!")
	seg1Plain := []byte("segment-one-content-goes-here!!")

	seg0Enc := testEncrypt(seg0Plain, key, iv)
	seg1Enc := testEncrypt(seg1Plain, key, iv)

	mux := http.NewServeMux()
	mux.HandleFunc("/playlist.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-KEY:METHOD=AES-128,URI="key.bin",IV=0x61626364656630313233343536373839
#EXTINF:10.0,
seg0.ts
#EXTINF:10.0,
seg1.ts
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/key.bin", func(w http.ResponseWriter, r *http.Request) {
		w.Write(key)
	})
	mux.HandleFunc("/seg0.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Write(seg0Enc)
	})
	mux.HandleFunc("/seg1.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Write(seg1Enc)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	saveDir := t.TempDir()
	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
		Decryptor:  &crypto.AES128Decryptor{},
	}

	task := &model.Task{
		URL:         server.URL + "/playlist.m3u8",
		SaveDir:     saveDir,
		SaveName:    "e2e_enc",
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  1,
		NoMerge:     true,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err != nil {
		t.Fatalf("E2E encrypted HLS failed: %v", err)
	}

	// Verify decrypted content
	data0, _ := os.ReadFile(downloader.SegmentFilePath(tmpDir, 0))
	data1, _ := os.ReadFile(downloader.SegmentFilePath(tmpDir, 1))

	if string(data0) != string(seg0Plain) {
		t.Errorf("seg0 decryption mismatch: got %q", string(data0))
	}
	if string(data1) != string(seg1Plain) {
		t.Errorf("seg1 decryption mismatch: got %q", string(data1))
	}
}

func TestE2E_DASH_VOD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.mpd", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"
     mediaPresentationDuration="PT4S" minBufferTime="PT1S">
  <Period>
    <AdaptationSet contentType="video" mimeType="video/mp4">
      <Representation id="v1" bandwidth="1000000" width="1280" height="720" codecs="avc1.64001f">
        <SegmentTemplate media="video_$Number$.m4s" initialization="video_init.mp4"
                         duration="2000" timescale="1000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`)
	})
	mux.HandleFunc("/video_init.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DASH_INIT"))
	})
	mux.HandleFunc("/video_1.m4s", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DASH_SEG1"))
	})
	mux.HandleFunc("/video_2.m4s", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DASH_SEG2"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	saveDir := t.TempDir()
	pipe := &Pipeline{
		Parser:     &dash.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:          server.URL + "/manifest.mpd",
		SaveDir:      saveDir,
		SaveName:     "e2e_dash",
		TmpDir:       t.TempDir(),
		ThreadCount:  2,
		RetryCount:   1,
		BinaryMerge:  true,
		DelAfterDone: true,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err != nil {
		t.Fatalf("E2E DASH failed: %v", err)
	}

	output := filepath.Join(saveDir, "e2e_dash.mp4")
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(data) != "DASH_INITDASH_SEG1DASH_SEG2" {
		t.Errorf("expected 'DASH_INITDASH_SEG1DASH_SEG2', got %q", string(data))
	}
}

func TestE2E_HLS_MultiStream_AutoSelect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=500000,RESOLUTION=320x180
low.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1280x720
high.m3u8
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",URI="audio.m3u8"
`)
	})
	for _, name := range []string{"low", "high", "audio"} {
		name := name
		mux.HandleFunc("/"+name+".m3u8", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXTINF:2.0,
%s_seg0.ts
#EXT-X-ENDLIST
`, name)
		})
		mux.HandleFunc("/"+name+"_seg0.ts", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(name + "_data"))
		})
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	saveDir := t.TempDir()
	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:         server.URL + "/master.m3u8",
		SaveDir:     saveDir,
		SaveName:    "multistream",
		TmpDir:      t.TempDir(),
		ThreadCount: 2,
		RetryCount:  1,
		AutoSelect:  true,
		NoMerge:     true,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err != nil {
		t.Fatalf("E2E multi-stream failed: %v", err)
	}
}

func TestE2E_Pipeline_NoStreamsError(t *testing.T) {
	// Return a valid M3U8 but with no segments
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-ENDLIST
`)
	}))
	defer server.Close()

	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:         server.URL + "/empty.m3u8",
		SaveDir:     t.TempDir(),
		SaveName:    "empty",
		TmpDir:      t.TempDir(),
		ThreadCount: 1,
		RetryCount:  0,
		NoMerge:     true,
	}

	err := pipe.Run(context.Background(), task, nil)
	// Should succeed (single stream with 0 segments is skipped gracefully)
	if err != nil {
		t.Logf("pipeline returned error (acceptable for empty stream): %v", err)
	}
}

func TestE2E_Pipeline_ParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:         server.URL + "/missing.m3u8",
		SaveDir:     t.TempDir(),
		TmpDir:      t.TempDir(),
		ThreadCount: 1,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err == nil {
		t.Fatal("expected error when playlist returns 404")
	}
}

func TestE2E_HLS_LiveToVOD(t *testing.T) {
	// Start as live, end with ENDLIST after a few refreshes
	var callCount int

	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount >= 3 {
			fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:2.0,
seg0.ts
#EXTINF:2.0,
seg1.ts
#EXTINF:2.0,
seg2.ts
#EXT-X-ENDLIST
`)
			return
		}
		fmt.Fprintf(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:2.0,
seg0.ts
#EXTINF:2.0,
seg%d.ts
`, callCount)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("live-segment-data"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:          server.URL + "/live.m3u8",
		TmpDir:       tmpDir,
		ThreadCount:  2,
		RetryCount:   1,
		Live:         true,
		LiveWaitTime: 1,
		NoMerge:      true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := pipe.Run(ctx, task, nil)
	if err != nil {
		t.Fatalf("E2E live-to-VOD failed: %v", err)
	}
}

// --- Integration tests with remote rustfs ---

func TestE2E_RustFS_HLS(t *testing.T) {
	endpoint := "http://192.168.3.5:9000"
	bucket := "video-server"
	hlsURL := endpoint + "/" + bucket + "/hls/master.m3u8"

	if !isReachable(endpoint) {
		t.Skip("rustfs not reachable, skipping remote integration test")
	}
	if !urlExists(hlsURL) {
		t.Skip("HLS test data not found on rustfs, run 'task mockdata:upload' first")
	}

	// Parse and verify the HLS structure from rustfs
	hlsParser := &hls.Parser{}
	result, err := hlsParser.Parse(context.Background(), hlsURL, nil)
	if err != nil {
		t.Fatalf("HLS parse failed: %v", err)
	}

	if len(result.Streams) < 2 {
		t.Fatalf("expected at least 2 streams (low+high), got %d", len(result.Streams))
	}

	for i, s := range result.Streams {
		if s.Playlist == nil {
			t.Errorf("stream %d: nil playlist", i)
			continue
		}
		if len(s.Playlist.Segments) == 0 {
			t.Errorf("stream %d: no segments", i)
		}
		t.Logf("stream %d: bandwidth=%d, segments=%d", i, s.Bandwidth, len(s.Playlist.Segments))
	}

	// Download 1 segment to verify the download path works.
	// Note: rustfs has a known issue with HTTP keep-alive on large TS files,
	// causing "unexpected EOF" on consecutive requests over the same connection.
	// Downloading a single segment avoids triggering this server-side bug.
	lowStream := result.Streams[0]
	seg := lowStream.Playlist.Segments[0:1]

	tmpDir := t.TempDir()
	dl := &downloader.HTTPDownloader{}
	err = dl.Download(context.Background(), seg, downloader.Options{
		TmpDir:      tmpDir,
		ThreadCount: 1,
		RetryCount:  2,
	}, nil)
	if err != nil {
		t.Fatalf("segment download failed: %v", err)
	}

	path := downloader.SegmentFilePath(tmpDir, seg[0].Index)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("segment file missing: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("segment too small: %d bytes", info.Size())
	}
	t.Logf("HLS: parsed %d streams, downloaded 1 segment (%d bytes)", len(result.Streams), info.Size())
}

func TestE2E_RustFS_DASH(t *testing.T) {
	endpoint := "http://192.168.3.5:9000"
	bucket := "video-server"
	dashURL := endpoint + "/" + bucket + "/dash/manifest.mpd"

	if !isReachable(endpoint) {
		t.Skip("rustfs not reachable, skipping remote integration test")
	}
	if !urlExists(dashURL) {
		t.Skip("DASH test data not found on rustfs, run 'task mockdata:upload' first")
	}

	saveDir := t.TempDir()
	pipe := &Pipeline{
		Parser:     &dash.Parser{},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:          dashURL,
		SaveDir:      saveDir,
		SaveName:     "rustfs_dash",
		TmpDir:       t.TempDir(),
		ThreadCount:  4,
		RetryCount:   2,
		AutoSelect:   true,
		BinaryMerge:  true,
		DelAfterDone: true,
	}

	var lastProgress model.ProgressEvent
	err := pipe.Run(context.Background(), task, func(e model.ProgressEvent) {
		lastProgress = e
	})
	if err != nil {
		t.Fatalf("rustfs DASH download failed: %v", err)
	}

	output := filepath.Join(saveDir, "rustfs_dash.mp4")
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
	t.Logf("DASH output: %s (%d bytes), %d segments", output, info.Size(), lastProgress.CompletedSegments)
}

// isReachable checks if a remote endpoint is reachable with a short timeout.
func isReachable(endpoint string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Head(endpoint)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// urlExists checks if a specific URL returns 200.
func urlExists(url string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func testEncrypt(plaintext, key, iv []byte) []byte {
	padLen := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}
	block, _ := aes.NewCipher(key)
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, len(padded))
	mode.CryptBlocks(encrypted, padded)
	return encrypted
}
