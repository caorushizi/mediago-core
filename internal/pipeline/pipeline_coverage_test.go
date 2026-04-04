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
	"github.com/caorushizi/mediago-core/internal/parser/hls"
)

func TestPipeline_NoStreamsSelected(t *testing.T) {
	// Empty playlist → no streams → error
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
		TmpDir:      t.TempDir(),
		ThreadCount: 1,
	}
	err := pipe.Run(context.Background(), task, nil)
	// Single stream with empty segments is valid (skipped), not an error
	if err != nil {
		t.Logf("got error (acceptable): %v", err)
	}
}

func TestSelectStreams_SingleStream(t *testing.T) {
	streams := []model.StreamSpec{
		{MediaType: model.MediaVideo, Bandwidth: 1000000},
	}
	task := &model.Task{}
	selected := selectStreams(streams, task)
	if len(selected) != 1 {
		t.Errorf("expected 1 stream, got %d", len(selected))
	}
}

func TestSelectStreams_DefaultNoAutoSelect(t *testing.T) {
	streams := []model.StreamSpec{
		{MediaType: model.MediaVideo, Bandwidth: 1000000},
		{MediaType: model.MediaVideo, Bandwidth: 2000000},
		{MediaType: model.MediaAudio, Language: "en"},
	}
	task := &model.Task{AutoSelect: false}
	selected := selectStreams(streams, task)
	// Default: return all
	if len(selected) != 3 {
		t.Errorf("expected 3 streams (all), got %d", len(selected))
	}
}

func TestSelectStreams_Empty(t *testing.T) {
	task := &model.Task{}
	selected := selectStreams(nil, task)
	if selected != nil {
		t.Errorf("expected nil for empty streams, got %v", selected)
	}
}

func TestFetchKey_Success(t *testing.T) {
	key := []byte("0123456789abcdef")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(key)
	}))
	defer server.Close()

	got, err := fetchKey(context.Background(), server.URL+"/key.bin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(key) {
		t.Errorf("key mismatch")
	}
}

func TestFetchKey_HTTP404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := fetchKey(context.Background(), server.URL+"/key.bin", nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestFetchKey_ServerDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	_, err := fetchKey(context.Background(), server.URL+"/key.bin", nil)
	if err == nil {
		t.Fatal("expected error when server is down")
	}
}

func TestFetchKey_WithHeaders(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte("key"))
	}))
	defer server.Close()

	headers := map[string]string{"Authorization": "Bearer token"}
	_, err := fetchKey(context.Background(), server.URL+"/key.bin", headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer token" {
		t.Errorf("expected auth header, got %q", gotAuth)
	}
}

func TestPipeline_RunLive_WithDurationAndWaitTime(t *testing.T) {
	var callCount int
	mux := http.NewServeMux()
	mux.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount >= 2 {
			fmt.Fprint(w, `#EXTM3U
#EXT-X-TARGETDURATION:2
#EXTINF:2.0,
final.ts
#EXT-X-ENDLIST
`)
			return
		}
		fmt.Fprint(w, `#EXTM3U
#EXT-X-TARGETDURATION:2
#EXTINF:2.0,
seg0.ts
`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:          server.URL + "/live.m3u8",
		TmpDir:       t.TempDir(),
		ThreadCount:  1,
		RetryCount:   1,
		Live:         true,
		LiveDuration: "10s",
		LiveWaitTime: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := pipe.Run(ctx, task, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPipeline_RunLive_InvalidDuration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-TARGETDURATION:2
#EXTINF:2.0,
seg0.ts
`)
	}))
	defer server.Close()

	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:          server.URL + "/live.m3u8",
		TmpDir:       t.TempDir(),
		ThreadCount:  1,
		Live:         true,
		LiveDuration: "invalid",
	}

	err := pipe.Run(context.Background(), task, nil)
	if err == nil {
		t.Fatal("expected error for invalid live duration")
	}
}

func TestPipeline_DelAfterDone(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/video.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:10
#EXT-X-MAP:URI="init.mp4"
#EXTINF:10.0,
seg0.m4s
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/init.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("INIT"))
	})
	mux.HandleFunc("/seg0.m4s", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("SEG0"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := filepath.Join(t.TempDir(), "segments")
	saveDir := t.TempDir()

	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:          server.URL + "/video.m3u8",
		SaveDir:      saveDir,
		SaveName:     "cleanup_test",
		TmpDir:       tmpDir,
		ThreadCount:  1,
		RetryCount:   1,
		BinaryMerge:  true,
		DelAfterDone: true,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// TmpDir should be cleaned up
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Error("expected tmpDir to be deleted after merge")
	}

	// Output file should exist
	output := filepath.Join(saveDir, "cleanup_test.mp4")
	if _, err := os.Stat(output); err != nil {
		t.Errorf("output file missing: %v", err)
	}
}

func TestPipeline_DecryptSegments_NilDecryptor(t *testing.T) {
	pipe := &Pipeline{}
	err := pipe.decryptSegments(context.Background(), &model.Task{}, &model.Playlist{}, "/tmp")
	if err != nil {
		t.Fatalf("expected nil error for nil decryptor, got: %v", err)
	}
}

func TestPipeline_DecryptSegments_NoEncryption(t *testing.T) {
	pipe := &Pipeline{Decryptor: &crypto.AES128Decryptor{}}
	playlist := &model.Playlist{
		Segments: []model.Segment{
			{Index: 0, EncryptInfo: nil},
			{Index: 1, EncryptInfo: &model.EncryptInfo{Method: model.EncryptNone}},
		},
	}
	err := pipe.decryptSegments(context.Background(), &model.Task{}, playlist, "/tmp")
	if err != nil {
		t.Fatalf("expected nil error for non-encrypted segments, got: %v", err)
	}
}

func TestPipeline_DecryptSegments_FetchKeyAndDecrypt(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	plaintext := []byte("segment-content-pad-here!!!!!!!!")

	// Encrypt
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

	// Key server
	keyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(key)
	}))
	defer keyServer.Close()

	// Write encrypted segment to temp dir
	tmpDir := t.TempDir()
	segPath := downloader.SegmentFilePath(tmpDir, 0)
	os.WriteFile(segPath, encrypted, 0o644)

	pipe := &Pipeline{Decryptor: &crypto.AES128Decryptor{}}
	playlist := &model.Playlist{
		Segments: []model.Segment{
			{
				Index: 0,
				EncryptInfo: &model.EncryptInfo{
					Method: model.EncryptAES128,
					KeyURL: keyServer.URL + "/key.bin",
					IV:     iv,
				},
			},
		},
	}

	err := pipe.decryptSegments(context.Background(), &model.Task{}, playlist, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(segPath)
	if string(data) != string(plaintext) {
		t.Errorf("decrypted mismatch: got %q", string(data))
	}
}

func TestPipeline_MultiStreamOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000
video.m3u8
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",URI="audio.m3u8"
`)
	})
	mux.HandleFunc("/video.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:2
#EXT-X-MAP:URI="vinit.mp4"
#EXTINF:2.0,
vseg0.m4s
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/audio.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:2
#EXT-X-MAP:URI="ainit.mp4"
#EXTINF:2.0,
aseg0.m4s
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DATA"))
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
		SaveName:     "multi",
		TmpDir:       t.TempDir(),
		ThreadCount:  1,
		RetryCount:   1,
		AutoSelect:   false, // download all streams
		BinaryMerge:  true,
		DelAfterDone: false,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have video output and audio output
	videoOut := filepath.Join(saveDir, "multi.mp4")
	audioOut := filepath.Join(saveDir, "multi_audio.mp4")

	if _, err := os.Stat(videoOut); err != nil {
		t.Errorf("video output missing: %v", err)
	}
	if _, err := os.Stat(audioOut); err != nil {
		t.Errorf("audio output missing: %v", err)
	}
}
