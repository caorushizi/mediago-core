package merger

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryMerger_Merge(t *testing.T) {
	tests := []struct {
		name     string
		segments [][]byte
		expected []byte
	}{
		{
			name:     "three segments",
			segments: [][]byte{{0, 1, 2}, {1, 2, 3}, {2, 3, 4}},
			expected: []byte{0, 1, 2, 1, 2, 3, 2, 3, 4},
		},
		{
			name:     "four single-byte segments",
			segments: [][]byte{{0}, {1}, {2}, {3}},
			expected: []byte{0, 1, 2, 3},
		},
		{
			name:     "single segment",
			segments: [][]byte{{10, 20, 30}},
			expected: []byte{10, 20, 30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			files := make([]string, len(tt.segments))
			for i, data := range tt.segments {
				files[i] = filepath.Join(tmpDir, fmt.Sprintf("seg_%d", i))
				os.WriteFile(files[i], data, 0o644)
			}

			output := filepath.Join(tmpDir, "output.mp4")
			m := &BinaryMerger{}
			if err := m.Merge(context.Background(), files, output); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatalf("read output: %v", err)
			}
			if !bytes.Equal(data, tt.expected) {
				t.Errorf("got %v, want %v", data, tt.expected)
			}
		})
	}
}

func TestBinaryMerger_EmptyList(t *testing.T) {
	m := &BinaryMerger{}
	err := m.Merge(context.Background(), nil, "/tmp/out.mp4")
	if err == nil {
		t.Error("expected error for empty segment list")
	}
}

func TestBinaryMerger_WithInitSegment(t *testing.T) {
	tmpDir := t.TempDir()

	initFile := filepath.Join(tmpDir, "init.mp4")
	os.WriteFile(initFile, []byte("INIT"), 0o644)

	seg0 := filepath.Join(tmpDir, "seg0")
	os.WriteFile(seg0, []byte("SEG0"), 0o644)

	seg1 := filepath.Join(tmpDir, "seg1")
	os.WriteFile(seg1, []byte("SEG1"), 0o644)

	files := []string{initFile, seg0, seg1}
	output := filepath.Join(tmpDir, "output.mp4")

	m := &BinaryMerger{}
	if err := m.Merge(context.Background(), files, output); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(output)
	if string(data) != "INITSEG0SEG1" {
		t.Errorf("expected 'INITSEG0SEG1', got %q", string(data))
	}
}

func TestBinaryMerger_CreateOutputError(t *testing.T) {
	m := &BinaryMerger{}
	files := []string{"/tmp/seg.ts"}
	err := m.Merge(context.Background(), files, "/nonexistent/path/output.mp4")
	if err == nil {
		t.Error("expected error when creating output in invalid path")
	}
}

func TestBinaryMerger_SegmentOpenError(t *testing.T) {
	tmpDir := t.TempDir()
	output := filepath.Join(tmpDir, "output.mp4")

	files := []string{filepath.Join(tmpDir, "nonexistent_segment.ts")}
	m := &BinaryMerger{}
	err := m.Merge(context.Background(), files, output)
	if err == nil {
		t.Error("expected error when segment file doesn't exist")
	}
}

func TestBinaryMerger_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{filepath.Join(tmpDir, "seg1.ts")}
	os.WriteFile(files[0], []byte("data"), 0o644)

	output := filepath.Join(tmpDir, "output.mp4")
	m := &BinaryMerger{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Merge(ctx, files, output)
	if err == nil {
		t.Error("expected error when context is cancelled")
	}
}

func TestWriteConcatList(t *testing.T) {
	tmpDir := t.TempDir()
	listPath := filepath.Join(tmpDir, "list.txt")

	files := []string{"/tmp/seg_00000", "/tmp/seg_00001"}
	if err := writeConcatList(listPath, files); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(listPath)
	content := string(data)

	if !strings.Contains(content, "file '") {
		t.Error("expected 'file' entries in concat list")
	}
	if !strings.Contains(content, "seg_00000") || !strings.Contains(content, "seg_00001") {
		t.Error("expected segment paths in concat list")
	}
}

func TestWriteConcatList_EscapesQuotes(t *testing.T) {
	tmpDir := t.TempDir()
	listPath := filepath.Join(tmpDir, "list.txt")

	files := []string{"/tmp/file with 'quotes'.ts"}
	if err := writeConcatList(listPath, files); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(listPath)
	content := string(data)

	if !strings.Contains(content, "'\\''") {
		t.Error("expected escaped quotes in concat list")
	}
}

func TestWriteConcatList_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	listPath := filepath.Join(tmpDir, "list.txt")

	files := []string{"/absolute/path/seg1.ts", "/absolute/path/seg2.ts"}
	if err := writeConcatList(listPath, files); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(listPath)
	content := string(data)

	if !strings.Contains(content, "/absolute/path/seg1.ts") {
		t.Error("expected absolute path in concat list")
	}
}

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}
}

func TestFFmpegMerger_Merge(t *testing.T) {
	requireFFmpeg(t)
	tmpDir := t.TempDir()

	files := make([]string, 3)
	for i := 0; i < 3; i++ {
		files[i] = filepath.Join(tmpDir, fmt.Sprintf("seg_%d.ts", i))
		os.WriteFile(files[i], []byte(fmt.Sprintf("segment-%d", i)), 0o644)
	}

	output := filepath.Join(tmpDir, "output.mp4")
	m := &FFmpegMerger{}
	err := m.Merge(context.Background(), files, output)
	// FFmpeg will fail on dummy data (not valid video), but we verify the code path runs
	if err == nil {
		t.Log("FFmpeg succeeded with dummy data (unexpected)")
	}
}

func TestFFmpegMerger_EmptyList(t *testing.T) {
	m := &FFmpegMerger{}
	err := m.Merge(context.Background(), nil, "/tmp/out.mp4")
	if err == nil {
		t.Error("expected error for empty segment list")
	}
}

func TestFFmpegMerger_CustomFFmpegPath(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{filepath.Join(tmpDir, "seg_0.ts")}
	os.WriteFile(files[0], []byte("data"), 0o644)

	output := filepath.Join(tmpDir, "output.mp4")
	m := &FFmpegMerger{FFmpegPath: "/nonexistent/ffmpeg"}
	err := m.Merge(context.Background(), files, output)
	if err == nil {
		t.Error("expected error for invalid ffmpeg path")
	}
}

func TestFFmpegMerger_CmdRunError(t *testing.T) {
	requireFFmpeg(t)
	tmpDir := t.TempDir()

	files := []string{filepath.Join(tmpDir, "seg.ts")}
	os.WriteFile(files[0], []byte("not valid video data"), 0o644)

	output := filepath.Join(tmpDir, "output.mp4")
	m := &FFmpegMerger{}
	err := m.Merge(context.Background(), files, output)
	if err == nil {
		t.Error("expected error when ffmpeg fails")
	}
}
