package dash

import (
	"testing"

	"github.com/caorushizi/mediago-core/internal/model"
)

const vodMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"
     mediaPresentationDuration="PT30S" minBufferTime="PT1.5S">
  <Period>
    <AdaptationSet contentType="video" mimeType="video/mp4">
      <Representation id="720p" bandwidth="2000000" width="1280" height="720" codecs="avc1.64001f">
        <SegmentTemplate media="video_720p_$Number$.m4s" initialization="video_720p_init.mp4"
                         duration="2000" timescale="1000" startNumber="1"/>
      </Representation>
      <Representation id="1080p" bandwidth="5000000" width="1920" height="1080" codecs="avc1.640028">
        <SegmentTemplate media="video_1080p_$Number$.m4s" initialization="video_1080p_init.mp4"
                         duration="2000" timescale="1000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
    <AdaptationSet contentType="audio" mimeType="audio/mp4" lang="en">
      <Representation id="audio_en" bandwidth="128000" codecs="mp4a.40.2">
        <SegmentTemplate media="audio_en_$Number$.m4s" initialization="audio_en_init.mp4"
                         duration="2000" timescale="1000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

const timelineMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"
     mediaPresentationDuration="PT10S">
  <Period>
    <AdaptationSet contentType="video" mimeType="video/mp4">
      <SegmentTemplate media="seg_$Number$.m4s" initialization="init.mp4"
                       timescale="1000" startNumber="0">
        <SegmentTimeline>
          <S t="0" d="2000" r="4"/>
        </SegmentTimeline>
      </SegmentTemplate>
      <Representation id="v1" bandwidth="3000000" width="1920" height="1080"/>
    </AdaptationSet>
  </Period>
</MPD>`

const segmentListMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"
     mediaPresentationDuration="PT6S">
  <Period>
    <AdaptationSet contentType="video" mimeType="video/mp4">
      <Representation id="v1" bandwidth="2000000" width="1280" height="720">
        <SegmentList duration="2000" timescale="1000">
          <Initialization sourceURL="init.mp4"/>
          <SegmentURL media="seg1.m4s"/>
          <SegmentURL media="seg2.m4s"/>
          <SegmentURL media="seg3.m4s"/>
        </SegmentList>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

const segmentBaseMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"
     mediaPresentationDuration="PT60S">
  <Period>
    <AdaptationSet contentType="video" mimeType="video/mp4">
      <Representation id="v1" bandwidth="2000000" width="1280" height="720">
        <BaseURL>video.mp4</BaseURL>
        <SegmentBase>
          <Initialization range="0-1000"/>
        </SegmentBase>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

const baseURLMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"
     mediaPresentationDuration="PT10S">
  <BaseURL>https://cdn.example.com/</BaseURL>
  <Period>
    <BaseURL>content/</BaseURL>
    <AdaptationSet contentType="video" mimeType="video/mp4">
      <Representation id="v1" bandwidth="2000000" width="1280" height="720">
        <SegmentTemplate media="seg_$Number$.m4s" initialization="init.mp4"
                         duration="2000" timescale="1000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

func TestParseMPD_VOD(t *testing.T) {
	result, err := ParseMPD(vodMPD, "https://example.com/manifest.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsLive {
		t.Error("expected VOD")
	}
	if len(result.Streams) != 3 { // 2 video + 1 audio
		t.Fatalf("expected 3 streams, got %d", len(result.Streams))
	}

	// 720p video
	v720 := result.Streams[0]
	if v720.Bandwidth != 2000000 {
		t.Errorf("expected bandwidth 2000000, got %d", v720.Bandwidth)
	}
	if v720.Resolution != "1280x720" {
		t.Errorf("expected 1280x720, got %s", v720.Resolution)
	}
	if v720.Playlist == nil {
		t.Fatal("expected playlist")
	}
	if v720.Playlist.MediaInit == nil {
		t.Fatal("expected init segment")
	}
	// 30s / 2s = 15 segments
	if len(v720.Playlist.Segments) != 15 {
		t.Errorf("expected 15 segments, got %d", len(v720.Playlist.Segments))
	}
	if v720.Playlist.Segments[0].URL != "https://example.com/video_720p_1.m4s" {
		t.Errorf("unexpected seg URL: %s", v720.Playlist.Segments[0].URL)
	}

	// Audio
	audio := result.Streams[2]
	if audio.MediaType != model.MediaAudio {
		t.Error("expected audio type")
	}
	if audio.Language != "en" {
		t.Errorf("expected language en, got %s", audio.Language)
	}
}

func TestParseMPD_Timeline(t *testing.T) {
	result, err := ParseMPD(timelineMPD, "https://example.com/manifest.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(result.Streams))
	}

	pl := result.Streams[0].Playlist
	// S t=0 d=2000 r=4 → 5 segments (r=4 means 4 repeats + 1 original)
	if len(pl.Segments) != 5 {
		t.Fatalf("expected 5 segments, got %d", len(pl.Segments))
	}

	// Each segment 2 seconds
	if pl.Segments[0].Duration != 2.0 {
		t.Errorf("expected duration 2.0, got %f", pl.Segments[0].Duration)
	}

	// Verify init segment
	if pl.MediaInit == nil {
		t.Fatal("expected init segment")
	}
	if pl.MediaInit.URL != "https://example.com/init.mp4" {
		t.Errorf("unexpected init URL: %s", pl.MediaInit.URL)
	}
}

func TestParseMPD_SegmentList(t *testing.T) {
	result, err := ParseMPD(segmentListMPD, "https://example.com/manifest.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pl := result.Streams[0].Playlist
	if len(pl.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(pl.Segments))
	}
	if pl.MediaInit == nil {
		t.Fatal("expected init segment")
	}
	if pl.MediaInit.URL != "https://example.com/init.mp4" {
		t.Errorf("unexpected init URL: %s", pl.MediaInit.URL)
	}
	if pl.Segments[0].URL != "https://example.com/seg1.m4s" {
		t.Errorf("unexpected seg URL: %s", pl.Segments[0].URL)
	}
}

func TestParseMPD_SegmentBase(t *testing.T) {
	result, err := ParseMPD(segmentBaseMPD, "https://example.com/manifest.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pl := result.Streams[0].Playlist
	if pl.MediaInit == nil {
		t.Fatal("expected init segment")
	}
	if pl.MediaInit.StartRange != 0 || pl.MediaInit.StopRange != 1000 {
		t.Errorf("expected range 0-1000, got %d-%d", pl.MediaInit.StartRange, pl.MediaInit.StopRange)
	}
	if len(pl.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(pl.Segments))
	}
	if pl.Segments[0].URL != "https://example.com/video.mp4" {
		t.Errorf("unexpected seg URL: %s", pl.Segments[0].URL)
	}
}

func TestParseMPD_BaseURLResolution(t *testing.T) {
	result, err := ParseMPD(baseURLMPD, "https://origin.example.com/manifest.mpd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pl := result.Streams[0].Playlist
	if pl.MediaInit.URL != "https://cdn.example.com/content/init.mp4" {
		t.Errorf("unexpected init URL: %s", pl.MediaInit.URL)
	}
	if pl.Segments[0].URL != "https://cdn.example.com/content/seg_1.m4s" {
		t.Errorf("unexpected seg URL: %s", pl.Segments[0].URL)
	}
}

func TestParseISO8601Duration(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"PT30S", 30},
		{"PT1M30S", 90},
		{"PT1H30M45S", 5445},
		{"PT10M", 600},
		{"PT0.5S", 0.5},
		{"P1DT2H", 93600},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseISO8601Duration(tt.input)
		if got != tt.want {
			t.Errorf("parseISO8601Duration(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestDetectMediaType(t *testing.T) {
	tests := []struct {
		contentType, mimeType, codecs string
		want                          model.MediaType
	}{
		{"video", "video/mp4", "avc1", model.MediaVideo},
		{"audio", "audio/mp4", "mp4a", model.MediaAudio},
		{"text", "text/vtt", "", model.MediaSubtitle},
		{"", "audio/mp4", "", model.MediaAudio},
		{"", "", "stpp", model.MediaSubtitle},
	}
	for _, tt := range tests {
		got := detectMediaType(tt.contentType, tt.mimeType, tt.codecs)
		if got != tt.want {
			t.Errorf("detectMediaType(%q,%q,%q) = %d, want %d", tt.contentType, tt.mimeType, tt.codecs, got, tt.want)
		}
	}
}

func TestReplaceVars(t *testing.T) {
	vars := map[string]string{
		"$RepresentationID$": "720p",
		"$Bandwidth$":        "2000000",
		"$Number$":           "5",
	}

	got := replaceVars("video_$RepresentationID$_$Number$.m4s", vars)
	want := "video_720p_5.m4s"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Formatted number
	got = replaceVars("seg_$Number%05d$.m4s", vars)
	want = "seg_00005.m4s"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
