package merger

import (
	"context"
	"fmt"
	"io"
	"os"
)

// BinaryMerger merges segment files by sequential binary concatenation.
// Used for fMP4 segments where init + media segments are simply joined.
type BinaryMerger struct{}

func (m *BinaryMerger) Merge(ctx context.Context, segmentFiles []string, output string) error {
	if len(segmentFiles) == 0 {
		return fmt.Errorf("no segment files to merge")
	}

	outFile, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	buf := make([]byte, 64*1024) // 64KB buffer
	for _, path := range segmentFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := appendFile(outFile, path, buf); err != nil {
			return fmt.Errorf("append %s: %w", path, err)
		}
	}

	return nil
}

func appendFile(dst *os.File, src string, buf []byte) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.CopyBuffer(dst, f, buf)
	return err
}
