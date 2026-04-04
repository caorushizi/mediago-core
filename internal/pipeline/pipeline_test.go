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

	"github.com/caorushizi/mediago-core/internal/crypto"
	"github.com/caorushizi/mediago-core/internal/downloader"
	"github.com/caorushizi/mediago-core/internal/model"
	"github.com/caorushizi/mediago-core/internal/parser/hls"
)

func TestPipeline_BasicVOD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/video.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
seg0.ts
#EXTINF:10.0,
seg1.ts
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/seg0.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("segment-0-data"))
	})
	mux.HandleFunc("/seg1.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("segment-1-data"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	saveDir := t.TempDir()

	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:          server.URL + "/video.m3u8",
		SaveDir:      saveDir,
		SaveName:     "test_video",
		TmpDir:       tmpDir,
		ThreadCount:  2,
		RetryCount:   1,
		DelAfterDone: false,
		NoMerge:      true,
	}

	var lastProgress model.ProgressEvent
	err := pipe.Run(context.Background(), task, func(e model.ProgressEvent) {
		lastProgress = e
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify segments downloaded
	data0, err := os.ReadFile(downloader.SegmentFilePath(tmpDir, 0))
	if err != nil {
		t.Fatalf("seg0 missing: %v", err)
	}
	if string(data0) != "segment-0-data" {
		t.Errorf("seg0: got %q", string(data0))
	}

	data1, err := os.ReadFile(downloader.SegmentFilePath(tmpDir, 1))
	if err != nil {
		t.Fatalf("seg1 missing: %v", err)
	}
	if string(data1) != "segment-1-data" {
		t.Errorf("seg1: got %q", string(data1))
	}

	if lastProgress.CompletedSegments != 2 {
		t.Errorf("expected completed 2, got %d", lastProgress.CompletedSegments)
	}
}

func TestPipeline_BinaryMerge(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/video.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:10
#EXT-X-MAP:URI="init.mp4"
#EXTINF:10.0,
seg0.m4s
#EXTINF:10.0,
seg1.m4s
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/init.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("INIT"))
	})
	mux.HandleFunc("/seg0.m4s", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("SEG0"))
	})
	mux.HandleFunc("/seg1.m4s", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("SEG1"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir := t.TempDir()
	saveDir := t.TempDir()

	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
	}

	task := &model.Task{
		URL:          server.URL + "/video.m3u8",
		SaveDir:      saveDir,
		SaveName:     "test_fmp4",
		TmpDir:       tmpDir,
		ThreadCount:  2,
		RetryCount:   1,
		DelAfterDone: false,
		BinaryMerge:  true,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := filepath.Join(saveDir, "test_fmp4.mp4")
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("output missing: %v", err)
	}
	if string(data) != "INITSEG0SEG1" {
		t.Errorf("expected 'INITSEG0SEG1', got %q", string(data))
	}
}

func TestPipeline_WithDecryption(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	plaintext := []byte("decrypted-segment-content-here!!")

	encrypted := testAESEncrypt(plaintext, key, iv)

	mux := http.NewServeMux()
	mux.HandleFunc("/video.m3u8", func(w http.ResponseWriter, r *http.Request) {
		// IV = hex of "abcdef0123456789"
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-KEY:METHOD=AES-128,URI="key.bin",IV=0x61626364656630313233343536373839
#EXTINF:10.0,
seg0.ts
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/key.bin", func(w http.ResponseWriter, r *http.Request) {
		w.Write(key)
	})
	mux.HandleFunc("/seg0.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Write(encrypted)
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
		URL:          server.URL + "/video.m3u8",
		SaveDir:      saveDir,
		SaveName:     "test_enc",
		TmpDir:       tmpDir,
		ThreadCount:  1,
		RetryCount:   1,
		DelAfterDone: false,
		NoMerge:      true,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	segPath := downloader.SegmentFilePath(tmpDir, 0)
	data, err := os.ReadFile(segPath)
	if err != nil {
		t.Fatalf("seg0 missing: %v", err)
	}
	if string(data) != string(plaintext) {
		t.Errorf("decrypted mismatch:\ngot:  %q\nwant: %q", string(data), string(plaintext))
	}
}

func TestAutoSelect(t *testing.T) {
	streams := []model.StreamSpec{
		{MediaType: model.MediaVideo, Bandwidth: 1000000},
		{MediaType: model.MediaVideo, Bandwidth: 5000000},
		{MediaType: model.MediaVideo, Bandwidth: 2500000},
		{MediaType: model.MediaAudio, Language: "en"},
		{MediaType: model.MediaAudio, Language: "ja"},
	}

	selected := autoSelect(streams)

	if len(selected) != 2 {
		t.Fatalf("expected 2, got %d", len(selected))
	}
	if selected[0].Bandwidth != 5000000 {
		t.Errorf("expected best video 5000000, got %d", selected[0].Bandwidth)
	}
	if selected[1].MediaType != model.MediaAudio {
		t.Error("expected audio as second stream")
	}
}

func testAESEncrypt(plaintext, key, iv []byte) []byte {
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
