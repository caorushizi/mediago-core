package dash

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1h30m", 90 * time.Minute},
		{"30s", 30 * time.Second},
		{"01:30:00", 90 * time.Minute},
		{"00:05:30", 5*time.Minute + 30*time.Second},
	}
	for _, tt := range tests {
		got, err := ParseDuration(tt.input)
		if err != nil {
			t.Errorf("ParseDuration(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	_, err := ParseDuration("invalid")
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestParseRange_Invalid(t *testing.T) {
	start, end := parseRange("invalid")
	if start != 0 || end != 0 {
		t.Errorf("expected 0,0 for invalid range, got %d,%d", start, end)
	}
}

func TestParseMPD_SegmentListWithRanges(t *testing.T) {
	mpd := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"
     mediaPresentationDuration="PT6S">
  <Period>
    <AdaptationSet contentType="video" mimeType="video/mp4">
      <Representation id="v1" bandwidth="2000000" width="1280" height="720">
        <SegmentList duration="2000" timescale="1000">
          <Initialization sourceURL="init.mp4" range="0-999"/>
          <SegmentURL media="seg1.m4s" mediaRange="1000-1999"/>
          <SegmentURL media="seg2.m4s" mediaRange="2000-2999"/>
        </SegmentList>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

	result, err := ParseMPD(mpd, "https://example.com/manifest.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pl := result.Streams[0].Playlist
	if pl.MediaInit == nil {
		t.Fatal("expected init segment")
	}
	if pl.MediaInit.StartRange != 0 || pl.MediaInit.StopRange != 999 {
		t.Errorf("init range: expected 0-999, got %d-%d", pl.MediaInit.StartRange, pl.MediaInit.StopRange)
	}

	if len(pl.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(pl.Segments))
	}
	if pl.Segments[0].StartRange != 1000 || pl.Segments[0].StopRange != 1999 {
		t.Errorf("seg0 range: expected 1000-1999, got %d-%d", pl.Segments[0].StartRange, pl.Segments[0].StopRange)
	}
}

func TestParseMPD_SegmentListNoInit(t *testing.T) {
	mpd := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"
     mediaPresentationDuration="PT4S">
  <Period>
    <AdaptationSet contentType="video" mimeType="video/mp4">
      <Representation id="v1" bandwidth="2000000" width="1280" height="720">
        <SegmentList duration="2000" timescale="1000">
          <SegmentURL media="seg1.m4s"/>
          <SegmentURL media="seg2.m4s"/>
        </SegmentList>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

	result, err := ParseMPD(mpd, "https://example.com/manifest.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pl := result.Streams[0].Playlist
	if pl.MediaInit != nil {
		t.Error("expected no init segment")
	}
	if len(pl.Segments) != 2 {
		t.Errorf("expected 2 segments, got %d", len(pl.Segments))
	}
}
