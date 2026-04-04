package downloader

import (
	"fmt"
	"time"
)

// WithRetry executes fn up to maxRetries times, returning the first nil error or the last error.
func WithRetry(maxRetries int, fn func() error) error {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if i < maxRetries {
				time.Sleep(time.Second * time.Duration(i+1))
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries+1, lastErr)
}
