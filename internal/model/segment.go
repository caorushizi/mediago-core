package model

// Segment represents a single downloadable piece of a stream.
type Segment struct {
	Index      int
	URL        string
	Duration   float64
	Title      string
	StartRange int64
	StopRange  int64
	EncryptInfo *EncryptInfo
}

// HasRange returns true if this segment uses byte-range requests.
func (s *Segment) HasRange() bool {
	return s.StartRange > 0 || s.StopRange > 0
}

// EncryptInfo holds encryption details for a segment.
type EncryptInfo struct {
	Method EncryptMethod
	KeyURL string
	Key    []byte
	IV     []byte
}

// EncryptMethod defines the encryption algorithm.
type EncryptMethod int

const (
	EncryptNone EncryptMethod = iota
	EncryptAES128
)
