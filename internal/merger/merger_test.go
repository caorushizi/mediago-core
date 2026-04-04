package merger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryMerger_Merge(t *testing.T) {
	tmpDir := t.TempDir()

	// Create segment files
	files := make([]string, 3)
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, "seg_%d")
		path = strings.Replace(path, "%d", string(rune('0'+i)), 1)
		files[i] = filepath.Join(tmpDir, segName(i))
		os.WriteFile(files[i], []byte{byte(i), byte(i + 1), byte(i + 2)}, 0o644)
	}

	output := filepath.Join(tmpDir, "output.mp4")
	m := &BinaryMerger{}
	err := m.Merge(context.Background(), files, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	expected := []byte{0, 1, 2, 1, 2, 3, 2, 3, 4}
	if len(data) != len(expected) {
		t.Fatalf("expected %d bytes, got %d", len(expected), len(data))
	}
	for i, b := range data {
		if b != expected[i] {
			t.Errorf("byte %d: expected %d, got %d", i, expected[i], b)
		}
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

	// Init segment comes first
	files := []string{initFile, seg0, seg1}
	output := filepath.Join(tmpDir, "output.mp4")

	m := &BinaryMerger{}
	err := m.Merge(context.Background(), files, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(output)
	if string(data) != "INITSEG0SEG1" {
		t.Errorf("expected 'INITSEG0SEG1', got %q", string(data))
	}
}

func TestWriteConcatList(t *testing.T) {
	tmpDir := t.TempDir()
	listPath := filepath.Join(tmpDir, "list.txt")

	files := []string{"/tmp/seg_00000", "/tmp/seg_00001"}
	err := writeConcatList(listPath, files)
	if err != nil {
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

func segName(i int) string {
	return "seg_" + string(rune('0'+i))
}
