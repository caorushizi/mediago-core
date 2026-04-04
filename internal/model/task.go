package model

// Task represents a download task with all user-provided parameters.
type Task struct {
	URL     string
	SaveDir string
	SaveName string
	TmpDir  string
	Headers map[string]string
	Proxy   string
	Timeout int

	ThreadCount int
	RetryCount  int

	AutoSelect  bool
	SelectVideo string
	SelectAudio string

	NoMerge      bool
	DelAfterDone bool
	FfmpegPath   string
	BinaryMerge  bool

	Key             []string
	CustomHLSMethod string
	CustomHLSKey    string
	CustomHLSIV     string

	Live         bool
	LiveDuration string
	LiveWaitTime int

	LogLevel string
	NoLog    bool
}

// ProgressEvent is emitted during download to report status.
type ProgressEvent struct {
	TotalSegments     int
	CompletedSegments int
	Percent           float64
	Speed             int64
	IsLive            bool
}

// MergeType defines how segments should be merged.
type MergeType int

const (
	MergeNone MergeType = iota
	MergeBinary
	MergeFFmpeg
)

// ParseResult holds the output of a parser.
type ParseResult struct {
	Streams   []StreamSpec
	MergeType MergeType
	IsLive    bool
}
