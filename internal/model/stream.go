package model

// StreamSpec represents a single variant stream (video, audio, etc.).
type StreamSpec struct {
	MediaType  MediaType
	GroupID    string
	Name       string
	Language   string
	Bandwidth  int64
	Codecs     string
	Resolution string
	FrameRate  float64
	Channels   string
	URL        string
	Playlist   *Playlist
}

// MediaType identifies the type of stream.
type MediaType int

const (
	MediaVideo MediaType = iota
	MediaAudio
	MediaSubtitle
)

// Playlist holds parsed segment information for a stream.
type Playlist struct {
	IsLive          bool
	TargetDuration  float64
	TotalDuration   float64
	MediaInit       *Segment
	Segments        []Segment
}
