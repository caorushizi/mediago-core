package merger

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FFmpegMerger merges TS segment files into a single output using ffmpeg concat demuxer.
type FFmpegMerger struct {
	FFmpegPath string // path to ffmpeg binary, defaults to "ffmpeg"
}

func (m *FFmpegMerger) Merge(ctx context.Context, segmentFiles []string, output string) error {
	if len(segmentFiles) == 0 {
		return fmt.Errorf("no segment files to merge")
	}

	ffmpeg := m.FFmpegPath
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}

	// Create concat list file
	listPath := filepath.Join(filepath.Dir(output), "_concat_list.txt")
	if err := writeConcatList(listPath, segmentFiles); err != nil {
		return fmt.Errorf("write concat list: %w", err)
	}
	defer os.Remove(listPath)

	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-movflags", "+faststart",
		output,
	}

	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}

	return nil
}

// writeConcatList writes an ffmpeg concat demuxer input file.
func writeConcatList(path string, files []string) error {
	var b strings.Builder
	for _, f := range files {
		absPath, err := filepath.Abs(f)
		if err != nil {
			return err
		}
		// Escape single quotes for ffmpeg
		escaped := strings.ReplaceAll(absPath, "'", "'\\''")
		fmt.Fprintf(&b, "file '%s'\n", escaped)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
