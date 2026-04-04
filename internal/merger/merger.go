package merger

import (
	"context"
)

// Merger combines downloaded segment files into a single output file.
type Merger interface {
	Merge(ctx context.Context, segmentFiles []string, output string) error
}
