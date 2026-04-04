package downloader

import (
	"sync/atomic"
	"time"
)

// SpeedTracker tracks download speed in real-time.
type SpeedTracker struct {
	downloaded atomic.Int64 // bytes downloaded in current window
	speed      atomic.Int64 // current speed in bytes/sec
	done       chan struct{}
}

// NewSpeedTracker creates and starts a speed tracker.
func NewSpeedTracker() *SpeedTracker {
	s := &SpeedTracker{
		done: make(chan struct{}),
	}
	go s.run()
	return s
}

// Add records downloaded bytes.
func (s *SpeedTracker) Add(n int64) {
	s.downloaded.Add(n)
}

// Speed returns the current download speed in bytes/sec.
func (s *SpeedTracker) Speed() int64 {
	return s.speed.Load()
}

// Stop stops the speed tracker.
func (s *SpeedTracker) Stop() {
	close(s.done)
}

func (s *SpeedTracker) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			bytes := s.downloaded.Swap(0)
			s.speed.Store(bytes)
		case <-s.done:
			return
		}
	}
}
