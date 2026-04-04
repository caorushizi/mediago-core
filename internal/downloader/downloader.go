package downloader

import (
	"context"

	"github.com/caorushizi/mediago-core/internal/model"
)

// Downloader downloads a list of segments concurrently.
type Downloader interface {
	Download(ctx context.Context, segments []model.Segment, opts Options, onProgress func(model.ProgressEvent)) error
}

// Options configures the download behavior.
type Options struct {
	TmpDir      string
	Headers     map[string]string
	Proxy       string
	Timeout     int
	ThreadCount int
	RetryCount  int
}
