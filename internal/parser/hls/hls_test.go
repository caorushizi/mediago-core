package hls

import (
	"testing"

	"github.com/caorushizi/mediago-core/internal/model"
)

const masterPlaylist = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1280000,RESOLUTION=720x480,CODECS="avc1.640028,mp4a.40.2"
low/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2560000,RESOLUTION=1280x720,CODECS="avc1.640028,mp4a.40.2"
mid/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=7680000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2"
high/index.m3u8

#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",LANGUAGE="en",NAME="English",URI="audio/en.m3u8"
`

const mediaPlaylist = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:9.009,
segment0.ts
#EXTINF:9.009,
segment1.ts
#EXTINF:3.003,
segment2.ts
#EXT-X-ENDLIST
`

const encryptedPlaylist = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-KEY:METHOD=AES-128,URI="https://example.com/key.bin",IV=0x00000000000000000000000000000001
#EXTINF:10.0,
seg0.ts
#EXTINF:10.0,
seg1.ts
#EXT-X-ENDLIST
`

const byteRangePlaylist = `#EXTM3U
#EXT-X-VERSION:4
#EXT-X-TARGETDURATION:10
#EXT-X-MAP:URI="init.mp4"
#EXTINF:10.0,
#EXT-X-BYTERANGE:1000@0
media.mp4
#EXTINF:10.0,
#EXT-X-BYTERANGE:1000@1000
media.mp4
#EXT-X-ENDLIST
`

const livePlaylist = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:5
#EXT-X-MEDIA-SEQUENCE:100
#EXTINF:5.0,
seg100.ts
#EXTINF:5.0,
seg101.ts
`

func TestIsMasterPlaylist(t *testing.T) {
	if !IsMasterPlaylist(masterPlaylist) {
		t.Error("expected master playlist")
	}
	if IsMasterPlaylist(mediaPlaylist) {
		t.Error("expected not master playlist")
	}
}

func TestParseMasterPlaylist(t *testing.T) {
	streams := ParseMasterPlaylist(masterPlaylist, "https://example.com/master.m3u8")

	if len(streams) != 4 { // 3 video + 1 audio
		t.Fatalf("expected 4 streams, got %d", len(streams))
	}

	// First video variant
	s := streams[0]
	if s.Bandwidth != 1280000 {
		t.Errorf("expected bandwidth 1280000, got %d", s.Bandwidth)
	}
	if s.Resolution != "720x480" {
		t.Errorf("expected resolution 720x480, got %s", s.Resolution)
	}
	if s.URL != "https://example.com/low/index.m3u8" {
		t.Errorf("unexpected URL: %s", s.URL)
	}

	// Highest variant
	s = streams[2]
	if s.Bandwidth != 7680000 {
		t.Errorf("expected bandwidth 7680000, got %d", s.Bandwidth)
	}

	// Audio rendition
	s = streams[3]
	if s.MediaType != model.MediaAudio {
		t.Errorf("expected audio media type, got %d", s.MediaType)
	}
	if s.Language != "en" {
		t.Errorf("expected language en, got %s", s.Language)
	}
	if s.URL != "https://example.com/audio/en.m3u8" {
		t.Errorf("unexpected audio URL: %s", s.URL)
	}
}

func TestParseMediaPlaylist(t *testing.T) {
	playlist, err := ParseMediaPlaylist(mediaPlaylist, "https://example.com/playlist.m3u8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if playlist.TargetDuration != 10 {
		t.Errorf("expected target duration 10, got %f", playlist.TargetDuration)
	}
	if playlist.IsLive {
		t.Error("expected VOD, got live")
	}
	if len(playlist.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(playlist.Segments))
	}

	seg := playlist.Segments[0]
	if seg.Index != 0 {
		t.Errorf("expected index 0, got %d", seg.Index)
	}
	if seg.Duration != 9.009 {
		t.Errorf("expected duration 9.009, got %f", seg.Duration)
	}
	if seg.URL != "https://example.com/segment0.ts" {
		t.Errorf("unexpected URL: %s", seg.URL)
	}
}

func TestParseEncryptedPlaylist(t *testing.T) {
	playlist, err := ParseMediaPlaylist(encryptedPlaylist, "https://example.com/enc.m3u8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(playlist.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(playlist.Segments))
	}

	seg := playlist.Segments[0]
	if seg.EncryptInfo == nil {
		t.Fatal("expected encrypt info")
	}
	if seg.EncryptInfo.Method != model.EncryptAES128 {
		t.Errorf("expected AES-128, got %d", seg.EncryptInfo.Method)
	}
	if seg.EncryptInfo.KeyURL != "https://example.com/key.bin" {
		t.Errorf("unexpected key URL: %s", seg.EncryptInfo.KeyURL)
	}
	if len(seg.EncryptInfo.IV) != 16 {
		t.Errorf("expected 16-byte IV, got %d bytes", len(seg.EncryptInfo.IV))
	}
}

func TestParseByteRangePlaylist(t *testing.T) {
	playlist, err := ParseMediaPlaylist(byteRangePlaylist, "https://example.com/br.m3u8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if playlist.MediaInit == nil {
		t.Fatal("expected init segment")
	}
	if playlist.MediaInit.URL != "https://example.com/init.mp4" {
		t.Errorf("unexpected init URL: %s", playlist.MediaInit.URL)
	}

	if len(playlist.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(playlist.Segments))
	}

	seg0 := playlist.Segments[0]
	if seg0.StartRange != 0 || seg0.StopRange != 999 {
		t.Errorf("expected range 0-999, got %d-%d", seg0.StartRange, seg0.StopRange)
	}

	seg1 := playlist.Segments[1]
	if seg1.StartRange != 1000 || seg1.StopRange != 1999 {
		t.Errorf("expected range 1000-1999, got %d-%d", seg1.StartRange, seg1.StopRange)
	}
}

func TestParseLivePlaylist(t *testing.T) {
	playlist, err := ParseMediaPlaylist(livePlaylist, "https://example.com/live.m3u8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !playlist.IsLive {
		t.Error("expected live playlist")
	}
	if len(playlist.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(playlist.Segments))
	}
	if playlist.Segments[0].Index != 100 {
		t.Errorf("expected start index 100, got %d", playlist.Segments[0].Index)
	}
}

func TestGetAttribute(t *testing.T) {
	tests := []struct {
		line, key, want string
	}{
		{`#EXT-X-KEY:METHOD=AES-128,URI="key.bin",IV=0x01`, "METHOD", "AES-128"},
		{`#EXT-X-KEY:METHOD=AES-128,URI="key.bin",IV=0x01`, "URI", "key.bin"},
		{`#EXT-X-KEY:METHOD=AES-128,URI="key.bin",IV=0x01`, "IV", "0x01"},
		{`#EXT-X-TARGETDURATION:10`, "", "10"},
		{`#EXT-X-STREAM-INF:BANDWIDTH=1280000,RESOLUTION=720x480`, "BANDWIDTH", "1280000"},
	}
	for _, tt := range tests {
		got := GetAttribute(tt.line, tt.key)
		if got != tt.want {
			t.Errorf("GetAttribute(%q, %q) = %q, want %q", tt.line, tt.key, got, tt.want)
		}
	}
}

func TestResolveURL(t *testing.T) {
	tests := []struct {
		base, ref, want string
	}{
		{"https://example.com/path/master.m3u8", "low/index.m3u8", "https://example.com/path/low/index.m3u8"},
		{"https://example.com/path/master.m3u8", "/abs/index.m3u8", "https://example.com/abs/index.m3u8"},
		{"https://example.com/path/master.m3u8", "https://cdn.example.com/index.m3u8", "https://cdn.example.com/index.m3u8"},
	}
	for _, tt := range tests {
		got := ResolveURL(tt.base, tt.ref)
		if got != tt.want {
			t.Errorf("ResolveURL(%q, %q) = %q, want %q", tt.base, tt.ref, got, tt.want)
		}
	}
}
