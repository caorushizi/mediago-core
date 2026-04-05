package pipeline

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/caorushizi/mediago-core/internal/downloader"
	"github.com/caorushizi/mediago-core/internal/model"
	"github.com/caorushizi/mediago-core/internal/parser/dash"
	"github.com/caorushizi/mediago-core/internal/parser/hls"
)

var update = flag.Bool("update", false, "update golden files")

// speedPattern matches speed=... in log lines for deterministic golden comparison.
var speedPattern = regexp.MustCompile(`speed=\S+`)

func goldenCompare(t *testing.T, name string, got string) {
	t.Helper()
	golden := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden file: %v", err)
		}
		t.Logf("updated %s", golden)
		return
	}

	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file: %v (run with -update to create)", err)
	}

	if got != string(expected) {
		t.Errorf("output mismatch with %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, string(expected))
	}
}

// saveTestOutput writes the test output to the testoutput/ directory at the project root.
func saveTestOutput(t *testing.T, name string, content string) {
	t.Helper()
	outDir := filepath.Join("..", "..", "testoutput")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create testoutput dir: %v", err)
	}
	outPath := filepath.Join(outDir, name+".log")
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write test output: %v", err)
	}
	t.Logf("saved %s", outPath)
}

// newLogCapture returns a log function that captures output to a builder.
// serverURL is stripped from log lines to make output deterministic.
func newLogCapture(b *strings.Builder, serverURL string) func(string, ...any) {
	return func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		// Strip httptest server address for deterministic output
		line = strings.ReplaceAll(line, serverURL, "")
		// Normalize speed values for deterministic comparison
		line = speedPattern.ReplaceAllString(line, "speed=<SPEED>")
		fmt.Fprintln(b, line)
	}
}

// TestGolden_HLS mirrors the mock data from scripts/mockdata/generate.sh:
// - 6s source video, hls_time=2 → 3 .ts segments per variant
// - 2 variants: low (640x360, 800kbps) + high (1280x720, 1600kbps)
// - Master playlist with CODECS
// - AutoSelect picks highest bandwidth
func TestGolden_HLS(t *testing.T) {
	// Simulated segment data (~188 bytes each like real TS packets)
	segData := strings.Repeat("X", 188)

	mux := http.NewServeMux()
	mux.HandleFunc("/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360,CODECS="avc1.640028,mp4a.40.2"
low/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1600000,RESOLUTION=1280x720,CODECS="avc1.640028,mp4a.40.2"
high/index.m3u8
`)
	})
	mux.HandleFunc("/low/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXTINF:2.0,
segment0.ts
#EXTINF:2.0,
segment1.ts
#EXTINF:2.0,
segment2.ts
#EXT-X-ENDLIST
`)
	})
	mux.HandleFunc("/high/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:2
#EXTINF:2.0,
segment0.ts
#EXTINF:2.0,
segment1.ts
#EXTINF:2.0,
segment2.ts
#EXT-X-ENDLIST
`)
	})
	// Low variant segments
	for i := 0; i < 3; i++ {
		mux.HandleFunc(fmt.Sprintf("/low/segment%d.ts", i), func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(segData))
		})
	}
	// High variant segments
	for i := 0; i < 3; i++ {
		mux.HandleFunc(fmt.Sprintf("/high/segment%d.ts", i), func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(segData))
		})
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	var b strings.Builder
	saveDir := t.TempDir()

	pipe := &Pipeline{
		Parser:     &hls.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
		OnLog:      newLogCapture(&b, server.URL),
	}

	task := &model.Task{
		URL:          server.URL + "/master.m3u8",
		SaveDir:      saveDir,
		SaveName:     "golden_hls",
		TmpDir:       t.TempDir(),
		ThreadCount:  1,
		RetryCount:   1,
		AutoSelect:   true,
		BinaryMerge:  true,
		DelAfterDone: true,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}

	goldenCompare(t, "e2e_hls", b.String())
	saveTestOutput(t, "e2e_hls", b.String())
}

// TestGolden_DASH mirrors the mock data from scripts/mockdata/generate.sh:
// - 6s source video, seg_duration=2 → 3 segments per representation
// - 2 video representations: low (640x360, 800kbps) + high (1280x720, 1600kbps)
// - 2 audio representations: low (96kbps) + high (128kbps)
// - SegmentTemplate with $RepresentationID$ and $Number$
func TestGolden_DASH(t *testing.T) {
	segData := strings.Repeat("Y", 256)
	initData := strings.Repeat("I", 128)

	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.mpd", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"
     mediaPresentationDuration="PT6S" minBufferTime="PT1S">
  <Period>
    <AdaptationSet id="0" contentType="video" mimeType="video/mp4">
      <Representation id="0" bandwidth="800000" width="640" height="360" codecs="avc1.640028">
        <SegmentTemplate media="chunk-stream$RepresentationID$-$Number%05d$.m4s"
                         initialization="init-stream$RepresentationID$.m4s"
                         duration="2000" timescale="1000" startNumber="1"/>
      </Representation>
      <Representation id="1" bandwidth="1600000" width="1280" height="720" codecs="avc1.640028">
        <SegmentTemplate media="chunk-stream$RepresentationID$-$Number%05d$.m4s"
                         initialization="init-stream$RepresentationID$.m4s"
                         duration="2000" timescale="1000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
    <AdaptationSet id="1" contentType="audio" mimeType="audio/mp4">
      <Representation id="2" bandwidth="96000" codecs="mp4a.40.2">
        <SegmentTemplate media="chunk-stream$RepresentationID$-$Number%05d$.m4s"
                         initialization="init-stream$RepresentationID$.m4s"
                         duration="2000" timescale="1000" startNumber="1"/>
      </Representation>
      <Representation id="3" bandwidth="128000" codecs="mp4a.40.2">
        <SegmentTemplate media="chunk-stream$RepresentationID$-$Number%05d$.m4s"
                         initialization="init-stream$RepresentationID$.m4s"
                         duration="2000" timescale="1000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`)
	})
	// Init segments for each representation
	for i := 0; i < 4; i++ {
		mux.HandleFunc(fmt.Sprintf("/init-stream%d.m4s", i), func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(initData))
		})
	}
	// Media segments: 3 segments per representation, 4 representations
	for rep := 0; rep < 4; rep++ {
		for seg := 1; seg <= 3; seg++ {
			mux.HandleFunc(fmt.Sprintf("/chunk-stream%d-%05d.m4s", rep, seg), func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(segData))
			})
		}
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	var b strings.Builder
	saveDir := t.TempDir()

	pipe := &Pipeline{
		Parser:     &dash.Parser{Client: server.Client()},
		Downloader: &downloader.HTTPDownloader{},
		OnLog:      newLogCapture(&b, server.URL),
	}

	task := &model.Task{
		URL:          server.URL + "/manifest.mpd",
		SaveDir:      saveDir,
		SaveName:     "golden_dash",
		TmpDir:       t.TempDir(),
		ThreadCount:  1,
		RetryCount:   1,
		BinaryMerge:  true,
		DelAfterDone: true,
	}

	err := pipe.Run(context.Background(), task, nil)
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}

	goldenCompare(t, "e2e_dash", b.String())
	saveTestOutput(t, "e2e_dash", b.String())
}
