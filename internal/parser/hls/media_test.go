package hls

import (
	"testing"

	"github.com/caorushizi/mediago-core/internal/model"
)

func TestSegIndexToIV(t *testing.T) {
	iv := segIndexToIV(0)
	if len(iv) != 16 {
		t.Fatalf("expected 16 bytes, got %d", len(iv))
	}
	// Index 0 → all zeros
	for i, b := range iv {
		if b != 0 {
			t.Errorf("byte %d: expected 0, got %d", i, b)
		}
	}

	iv1 := segIndexToIV(1)
	if iv1[15] != 1 {
		t.Errorf("expected last byte 1 for index 1, got %d", iv1[15])
	}

	iv255 := segIndexToIV(255)
	if iv255[15] != 0xff {
		t.Errorf("expected last byte 0xff for index 255, got %d", iv255[15])
	}
}

func TestCopyEncryptInfo(t *testing.T) {
	// nil input
	if copyEncryptInfo(nil) != nil {
		t.Error("expected nil for nil input")
	}

	// Full copy
	src := &model.EncryptInfo{
		Method: model.EncryptAES128,
		KeyURL: "https://example.com/key.bin",
		Key:    []byte("0123456789abcdef"),
		IV:     []byte("abcdef0123456789"),
	}
	dst := copyEncryptInfo(src)
	if dst.Method != src.Method {
		t.Error("method mismatch")
	}
	if dst.KeyURL != src.KeyURL {
		t.Error("keyURL mismatch")
	}
	if string(dst.Key) != string(src.Key) {
		t.Error("key mismatch")
	}
	if string(dst.IV) != string(src.IV) {
		t.Error("IV mismatch")
	}
	// Verify it's a deep copy
	dst.Key[0] = 0xFF
	if src.Key[0] == 0xFF {
		t.Error("key should be a deep copy")
	}

	// Without Key/IV
	src2 := &model.EncryptInfo{Method: model.EncryptAES128, KeyURL: "url"}
	dst2 := copyEncryptInfo(src2)
	if dst2.Key != nil || dst2.IV != nil {
		t.Error("expected nil Key/IV when source has nil")
	}
}

func TestParseEncryptInfo_MethodNone(t *testing.T) {
	info, err := parseEncryptInfo(`#EXT-X-KEY:METHOD=NONE`, "https://example.com/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil for METHOD=NONE")
	}
}

func TestParseEncryptInfo_EmptyMethod(t *testing.T) {
	info, err := parseEncryptInfo(`#EXT-X-KEY:URI="key.bin"`, "https://example.com/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil for empty method")
	}
}

func TestParseEncryptInfo_UnsupportedMethod(t *testing.T) {
	_, err := parseEncryptInfo(`#EXT-X-KEY:METHOD=SAMPLE-AES,URI="key.bin"`, "https://example.com/")
	if err == nil {
		t.Fatal("expected error for unsupported method")
	}
}

func TestParseEncryptInfo_InvalidIV(t *testing.T) {
	_, err := parseEncryptInfo(`#EXT-X-KEY:METHOD=AES-128,URI="key.bin",IV=0xZZZZ`, "https://example.com/")
	if err == nil {
		t.Fatal("expected error for invalid hex IV")
	}
}

func TestParseMediaPlaylist_EncryptWithoutIV(t *testing.T) {
	// When no IV is specified, segIndexToIV should be used
	content := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-KEY:METHOD=AES-128,URI="key.bin"
#EXTINF:10.0,
seg0.ts
#EXT-X-ENDLIST
`
	playlist, err := ParseMediaPlaylist(content, "https://example.com/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	seg := playlist.Segments[0]
	if seg.EncryptInfo == nil {
		t.Fatal("expected encrypt info")
	}
	if seg.EncryptInfo.IV == nil {
		t.Fatal("expected auto-generated IV")
	}
	if len(seg.EncryptInfo.IV) != 16 {
		t.Errorf("expected 16-byte IV, got %d", len(seg.EncryptInfo.IV))
	}
}

func TestParseMediaPlaylist_ByteRangeWithoutOffset(t *testing.T) {
	// BYTERANGE without @ uses previous range end
	content := `#EXTM3U
#EXT-X-VERSION:4
#EXT-X-TARGETDURATION:10
#EXT-X-MAP:URI="init.mp4"
#EXTINF:10.0,
#EXT-X-BYTERANGE:500@0
media.mp4
#EXTINF:10.0,
#EXT-X-BYTERANGE:500
media.mp4
#EXT-X-ENDLIST
`
	playlist, err := ParseMediaPlaylist(content, "https://example.com/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(playlist.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(playlist.Segments))
	}

	// First: 500@0 → 0-499
	seg0 := playlist.Segments[0]
	if seg0.StartRange != 0 || seg0.StopRange != 499 {
		t.Errorf("seg0: expected 0-499, got %d-%d", seg0.StartRange, seg0.StopRange)
	}

	// Second: 500 (no @, uses prevEnd=500) → 500-999
	seg1 := playlist.Segments[1]
	if seg1.StartRange != 500 || seg1.StopRange != 999 {
		t.Errorf("seg1: expected 500-999, got %d-%d", seg1.StartRange, seg1.StopRange)
	}
}

func TestParseMediaPlaylist_PlaylistTypeVOD(t *testing.T) {
	content := `#EXTM3U
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
seg0.ts
`
	playlist, err := ParseMediaPlaylist(content, "https://example.com/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if playlist.IsLive {
		t.Error("expected VOD (not live) for PLAYLIST-TYPE:VOD")
	}
}

func TestParseMediaPlaylist_MapWithEncrypt(t *testing.T) {
	// Init segment should copy encryption info
	content := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:10
#EXT-X-KEY:METHOD=AES-128,URI="key.bin",IV=0x00000000000000000000000000000001
#EXT-X-MAP:URI="init.mp4"
#EXTINF:10.0,
seg0.m4s
#EXT-X-ENDLIST
`
	playlist, err := ParseMediaPlaylist(content, "https://example.com/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if playlist.MediaInit == nil {
		t.Fatal("expected init segment")
	}
	if playlist.MediaInit.EncryptInfo == nil {
		t.Error("expected init segment to have encryption info")
	}
}

func TestParseMediaPlaylist_DiscontinuityTag(t *testing.T) {
	content := `#EXTM3U
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
seg0.ts
#EXT-X-DISCONTINUITY
#EXTINF:10.0,
seg1.ts
#EXT-X-ENDLIST
`
	playlist, err := ParseMediaPlaylist(content, "https://example.com/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(playlist.Segments) != 2 {
		t.Errorf("expected 2 segments, got %d", len(playlist.Segments))
	}
}

func TestGetAttribute_UnclosedQuote(t *testing.T) {
	got := GetAttribute(`#EXT-X-KEY:URI="unclosed`, "URI")
	if got != "unclosed" {
		t.Errorf("expected 'unclosed', got %q", got)
	}
}

func TestGetAttribute_MissingKey(t *testing.T) {
	got := GetAttribute(`#EXT-X-STREAM-INF:BANDWIDTH=1000`, "CODECS")
	if got != "" {
		t.Errorf("expected empty for missing key, got %q", got)
	}
}

func TestGetAttribute_NoColon(t *testing.T) {
	got := GetAttribute(`#EXTM3U`, "")
	if got != "" {
		t.Errorf("expected empty for no colon, got %q", got)
	}
}

func TestResolveURL_EmptyRef(t *testing.T) {
	got := ResolveURL("https://example.com/path.m3u8", "")
	if got != "https://example.com/path.m3u8" {
		t.Errorf("expected base URL for empty ref, got %q", got)
	}
}

func TestResolveURL_ParentPath(t *testing.T) {
	got := ResolveURL("https://example.com/a/b/master.m3u8", "../c/seg.ts")
	want := "https://example.com/a/c/seg.ts"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseMasterPlaylist_SubtitleMedia(t *testing.T) {
	content := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000
video.m3u8
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English",LANGUAGE="en",URI="subs/en.m3u8"
`
	streams := ParseMasterPlaylist(content, "https://example.com/master.m3u8")

	var found bool
	for _, s := range streams {
		if s.MediaType == model.MediaSubtitle {
			found = true
			if s.Language != "en" {
				t.Errorf("expected language en, got %s", s.Language)
			}
		}
	}
	if !found {
		t.Error("expected subtitle stream")
	}
}

func TestParseMasterPlaylist_DefaultRenditionNoURI(t *testing.T) {
	// Media tag without URI should be skipped
	content := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000
video.m3u8
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",DEFAULT=YES
`
	streams := ParseMasterPlaylist(content, "https://example.com/")
	// Only the video stream, no audio (no URI)
	if len(streams) != 1 {
		t.Errorf("expected 1 stream (no URI audio skipped), got %d", len(streams))
	}
}

func TestParseMasterPlaylist_UnknownMediaType(t *testing.T) {
	content := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000
video.m3u8
#EXT-X-MEDIA:TYPE=CLOSED-CAPTIONS,GROUP-ID="cc",NAME="English",URI="cc.m3u8"
`
	streams := ParseMasterPlaylist(content, "https://example.com/")
	// CLOSED-CAPTIONS hits default:continue, should be skipped
	if len(streams) != 1 {
		t.Errorf("expected 1 stream (unknown media type skipped), got %d", len(streams))
	}
}

func TestParseMasterPlaylist_AverageBandwidth(t *testing.T) {
	content := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1000000,AVERAGE-BANDWIDTH=800000
video.m3u8
`
	streams := ParseMasterPlaylist(content, "https://example.com/")
	// AVERAGE-BANDWIDTH overrides BANDWIDTH
	if streams[0].Bandwidth != 800000 {
		t.Errorf("expected 800000 (avg bandwidth), got %d", streams[0].Bandwidth)
	}
}
