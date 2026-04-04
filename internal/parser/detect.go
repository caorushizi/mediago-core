package parser

import "strings"

// StreamType represents the detected protocol type.
type StreamType int

const (
	StreamUnknown StreamType = iota
	StreamHLS
	StreamDASH
)

// DetectType detects the stream type from a URL.
func DetectType(url string) StreamType {
	lower := strings.ToLower(url)

	// Remove query string for extension matching
	if idx := strings.Index(lower, "?"); idx >= 0 {
		lower = lower[:idx]
	}

	switch {
	case strings.HasSuffix(lower, ".m3u8") || strings.HasSuffix(lower, ".m3u"):
		return StreamHLS
	case strings.HasSuffix(lower, ".mpd"):
		return StreamDASH
	default:
		return StreamUnknown
	}
}
