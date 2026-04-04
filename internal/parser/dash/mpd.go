package dash

import (
	"encoding/xml"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/caorushizi/mediago-core/internal/model"
	"github.com/caorushizi/mediago-core/internal/parser/hls"
)

// MPD XML structures

type MPD struct {
	XMLName                   xml.Name `xml:"MPD"`
	Type                      string   `xml:"type,attr"`
	MediaPresentationDuration string   `xml:"mediaPresentationDuration,attr"`
	MinBufferTime             string   `xml:"minBufferTime,attr"`
	BaseURL                   string   `xml:"BaseURL"`
	Periods                   []Period `xml:"Period"`
}

type Period struct {
	ID              string          `xml:"id,attr"`
	Duration        string          `xml:"duration,attr"`
	BaseURL         string          `xml:"BaseURL"`
	AdaptationSets  []AdaptationSet `xml:"AdaptationSet"`
}

type AdaptationSet struct {
	ID                   string               `xml:"id,attr"`
	ContentType          string               `xml:"contentType,attr"`
	MimeType             string               `xml:"mimeType,attr"`
	Lang                 string               `xml:"lang,attr"`
	Codecs               string               `xml:"codecs,attr"`
	FrameRate            string               `xml:"frameRate,attr"`
	BaseURL              string               `xml:"BaseURL"`
	SegmentTemplate      *SegmentTemplate     `xml:"SegmentTemplate"`
	ContentProtection    []ContentProtection  `xml:"ContentProtection"`
	Representations      []Representation     `xml:"Representation"`
}

type Representation struct {
	ID              string           `xml:"id,attr"`
	Bandwidth       int64            `xml:"bandwidth,attr"`
	Codecs          string           `xml:"codecs,attr"`
	Width           int              `xml:"width,attr"`
	Height          int              `xml:"height,attr"`
	MimeType        string           `xml:"mimeType,attr"`
	BaseURL         string           `xml:"BaseURL"`
	SegmentTemplate *SegmentTemplate `xml:"SegmentTemplate"`
	SegmentList     *SegmentList     `xml:"SegmentList"`
	SegmentBase     *SegmentBase     `xml:"SegmentBase"`
}

type SegmentTemplate struct {
	Media              string          `xml:"media,attr"`
	Initialization     string          `xml:"initialization,attr"`
	Duration           int64           `xml:"duration,attr"`
	Timescale          int64           `xml:"timescale,attr"`
	StartNumber        int64           `xml:"startNumber,attr"`
	SegmentTimeline    *SegmentTimeline `xml:"SegmentTimeline"`
}

type SegmentTimeline struct {
	S []SegmentTimelineS `xml:"S"`
}

type SegmentTimelineS struct {
	T int64 `xml:"t,attr"`
	D int64 `xml:"d,attr"`
	R int64 `xml:"r,attr"`
}

type SegmentList struct {
	Duration       int64           `xml:"duration,attr"`
	Timescale      int64           `xml:"timescale,attr"`
	Initialization *Initialization `xml:"Initialization"`
	SegmentURLs    []SegmentURL    `xml:"SegmentURL"`
}

type SegmentURL struct {
	Media      string `xml:"media,attr"`
	MediaRange string `xml:"mediaRange,attr"`
}

type SegmentBase struct {
	Initialization *Initialization `xml:"Initialization"`
	IndexRange     string          `xml:"indexRange,attr"`
}

type Initialization struct {
	SourceURL string `xml:"sourceURL,attr"`
	Range     string `xml:"range,attr"`
}

type ContentProtection struct {
	SchemeIdUri string `xml:"schemeIdUri,attr"`
}

// ParseMPD parses an MPD manifest and returns streams with segment info.
func ParseMPD(content string, baseURL string) (*model.ParseResult, error) {
	var mpd MPD
	if err := xml.Unmarshal([]byte(content), &mpd); err != nil {
		return nil, fmt.Errorf("unmarshal MPD: %w", err)
	}

	isLive := mpd.Type == "dynamic"
	mpdDuration := parseISO8601Duration(mpd.MediaPresentationDuration)

	// Resolve MPD-level BaseURL
	mpdBaseURL := baseURL
	if mpd.BaseURL != "" {
		mpdBaseURL = hls.ResolveURL(baseURL, mpd.BaseURL)
	}

	var streams []model.StreamSpec

	for _, period := range mpd.Periods {
		periodBaseURL := mpdBaseURL
		if period.BaseURL != "" {
			periodBaseURL = hls.ResolveURL(mpdBaseURL, period.BaseURL)
		}

		periodDuration := parseISO8601Duration(period.Duration)
		if periodDuration == 0 {
			periodDuration = mpdDuration
		}

		for _, as := range period.AdaptationSets {
			asBaseURL := periodBaseURL
			if as.BaseURL != "" {
				asBaseURL = hls.ResolveURL(periodBaseURL, as.BaseURL)
			}

			mediaType := detectMediaType(as.ContentType, as.MimeType, as.Codecs)

			for _, rep := range as.Representations {
				repBaseURL := asBaseURL
				if rep.BaseURL != "" {
					repBaseURL = hls.ResolveURL(asBaseURL, rep.BaseURL)
				}

				codecs := rep.Codecs
				if codecs == "" {
					codecs = as.Codecs
				}

				resolution := ""
				if rep.Width > 0 && rep.Height > 0 {
					resolution = fmt.Sprintf("%dx%d", rep.Width, rep.Height)
				}

				spec := model.StreamSpec{
					MediaType:  mediaType,
					GroupID:    rep.ID,
					Language:   as.Lang,
					Bandwidth:  rep.Bandwidth,
					Codecs:     codecs,
					Resolution: resolution,
				}

				// Build segment list from template, list, or base
				playlist, err := buildPlaylist(rep, as, repBaseURL, periodDuration, isLive)
				if err != nil {
					return nil, fmt.Errorf("build playlist for rep %s: %w", rep.ID, err)
				}
				spec.Playlist = playlist

				streams = append(streams, spec)
			}
		}
	}

	result := &model.ParseResult{
		Streams:   streams,
		IsLive:    isLive,
		MergeType: model.MergeBinary, // DASH uses fMP4 typically
	}

	return result, nil
}

// buildPlaylist constructs a Playlist from the representation's segment info.
func buildPlaylist(rep Representation, as AdaptationSet, baseURL string, periodDuration float64, isLive bool) (*model.Playlist, error) {
	playlist := &model.Playlist{
		IsLive: isLive,
	}

	// Prefer representation-level SegmentTemplate, fall back to AdaptationSet-level
	tmpl := rep.SegmentTemplate
	if tmpl == nil {
		tmpl = as.SegmentTemplate
	}

	vars := map[string]string{
		"$RepresentationID$": rep.ID,
		"$Bandwidth$":        strconv.FormatInt(rep.Bandwidth, 10),
	}

	switch {
	case tmpl != nil:
		return buildFromTemplate(tmpl, baseURL, periodDuration, isLive, vars)
	case rep.SegmentList != nil:
		return buildFromSegmentList(rep.SegmentList, baseURL, periodDuration)
	case rep.SegmentBase != nil:
		return buildFromSegmentBase(rep.SegmentBase, baseURL, periodDuration)
	default:
		// Single segment: BaseURL is the content
		playlist.Segments = []model.Segment{
			{Index: 0, URL: baseURL, Duration: periodDuration},
		}
		playlist.TotalDuration = periodDuration
		return playlist, nil
	}
}

func buildFromTemplate(tmpl *SegmentTemplate, baseURL string, periodDuration float64, isLive bool, vars map[string]string) (*model.Playlist, error) {
	playlist := &model.Playlist{IsLive: isLive}

	timescale := tmpl.Timescale
	if timescale == 0 {
		timescale = 1
	}

	// Init segment
	if tmpl.Initialization != "" {
		initURL := replaceVars(tmpl.Initialization, vars)
		playlist.MediaInit = &model.Segment{
			Index: -1,
			URL:   hls.ResolveURL(baseURL, initURL),
		}
	}

	startNumber := tmpl.StartNumber
	if startNumber == 0 {
		startNumber = 1
	}

	if tmpl.SegmentTimeline != nil {
		// Timeline-based segments
		var currentTime int64
		segIndex := 0

		for _, s := range tmpl.SegmentTimeline.S {
			if s.T > 0 {
				currentTime = s.T
			}
			repeatCount := s.R
			if repeatCount < 0 && periodDuration > 0 {
				repeatCount = int64(math.Ceil(periodDuration*float64(timescale)/float64(s.D))) - 1
			}

			for j := int64(0); j <= repeatCount; j++ {
				segVars := copyVars(vars)
				segVars["$Number$"] = strconv.FormatInt(startNumber+int64(segIndex), 10)
				segVars["$Time$"] = strconv.FormatInt(currentTime, 10)

				mediaURL := replaceVars(tmpl.Media, segVars)
				duration := float64(s.D) / float64(timescale)

				playlist.Segments = append(playlist.Segments, model.Segment{
					Index:    segIndex,
					URL:      hls.ResolveURL(baseURL, mediaURL),
					Duration: duration,
				})
				playlist.TotalDuration += duration

				currentTime += s.D
				segIndex++
			}
		}
	} else if tmpl.Duration > 0 {
		// Duration-based segments
		segDuration := float64(tmpl.Duration) / float64(timescale)
		totalSegments := int(math.Ceil(periodDuration / segDuration))

		for i := 0; i < totalSegments; i++ {
			segVars := copyVars(vars)
			segVars["$Number$"] = strconv.FormatInt(startNumber+int64(i), 10)

			mediaURL := replaceVars(tmpl.Media, segVars)

			dur := segDuration
			// Last segment may be shorter
			remaining := periodDuration - float64(i)*segDuration
			if remaining < segDuration {
				dur = remaining
			}

			playlist.Segments = append(playlist.Segments, model.Segment{
				Index:    i,
				URL:      hls.ResolveURL(baseURL, mediaURL),
				Duration: dur,
			})
			playlist.TotalDuration += dur
		}
	}

	return playlist, nil
}

func buildFromSegmentList(sl *SegmentList, baseURL string, periodDuration float64) (*model.Playlist, error) {
	playlist := &model.Playlist{}

	timescale := sl.Timescale
	if timescale == 0 {
		timescale = 1
	}
	segDuration := float64(sl.Duration) / float64(timescale)

	// Init segment
	if sl.Initialization != nil && sl.Initialization.SourceURL != "" {
		initSeg := &model.Segment{
			Index: -1,
			URL:   hls.ResolveURL(baseURL, sl.Initialization.SourceURL),
		}
		if sl.Initialization.Range != "" {
			start, stop := parseRange(sl.Initialization.Range)
			initSeg.StartRange = start
			initSeg.StopRange = stop
		}
		playlist.MediaInit = initSeg
	}

	for i, su := range sl.SegmentURLs {
		seg := model.Segment{
			Index:    i,
			URL:      hls.ResolveURL(baseURL, su.Media),
			Duration: segDuration,
		}
		if su.MediaRange != "" {
			start, stop := parseRange(su.MediaRange)
			seg.StartRange = start
			seg.StopRange = stop
		}
		playlist.Segments = append(playlist.Segments, seg)
		playlist.TotalDuration += segDuration
	}

	return playlist, nil
}

func buildFromSegmentBase(sb *SegmentBase, baseURL string, periodDuration float64) (*model.Playlist, error) {
	playlist := &model.Playlist{}

	if sb.Initialization != nil {
		initURL := baseURL
		if sb.Initialization.SourceURL != "" {
			initURL = hls.ResolveURL(baseURL, sb.Initialization.SourceURL)
		}
		initSeg := &model.Segment{
			Index: -1,
			URL:   initURL,
		}
		if sb.Initialization.Range != "" {
			start, stop := parseRange(sb.Initialization.Range)
			initSeg.StartRange = start
			initSeg.StopRange = stop
		}
		playlist.MediaInit = initSeg
	}

	// Single segment: the whole file
	playlist.Segments = []model.Segment{
		{Index: 0, URL: baseURL, Duration: periodDuration},
	}
	playlist.TotalDuration = periodDuration

	return playlist, nil
}

// replaceVars replaces template variables like $Number$, $RepresentationID$, etc.
func replaceVars(template string, vars map[string]string) string {
	result := template
	for k, v := range vars {
		result = strings.ReplaceAll(result, k, v)
	}
	// Handle formatted number like $Number%05d$
	re := regexp.MustCompile(`\$Number%(\d+)d\$`)
	result = re.ReplaceAllStringFunc(result, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		width, _ := strconv.Atoi(submatch[1])
		numStr := vars["$Number$"]
		num, _ := strconv.ParseInt(numStr, 10, 64)
		return fmt.Sprintf("%0*d", width, num)
	})
	return result
}

func copyVars(vars map[string]string) map[string]string {
	m := make(map[string]string, len(vars))
	for k, v := range vars {
		m[k] = v
	}
	return m
}

// parseRange parses "start-end" byte range.
func parseRange(r string) (int64, int64) {
	parts := strings.SplitN(r, "-", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	start, _ := strconv.ParseInt(parts[0], 10, 64)
	end, _ := strconv.ParseInt(parts[1], 10, 64)
	return start, end
}

// parseISO8601Duration parses ISO 8601 duration like "PT1H30M45.5S" to seconds.
func parseISO8601Duration(s string) float64 {
	if s == "" {
		return 0
	}
	s = strings.TrimPrefix(s, "P")

	var total float64

	// Split on 'T' to separate date and time parts
	parts := strings.SplitN(s, "T", 2)

	// Date part (days, etc.) - simplified
	datePart := parts[0]
	if idx := strings.Index(datePart, "D"); idx >= 0 {
		days, _ := strconv.ParseFloat(datePart[:idx], 64)
		total += days * 86400
	}

	if len(parts) < 2 {
		return total
	}

	timePart := parts[1]

	// Hours
	if idx := strings.Index(timePart, "H"); idx >= 0 {
		hours, _ := strconv.ParseFloat(timePart[:idx], 64)
		total += hours * 3600
		timePart = timePart[idx+1:]
	}

	// Minutes
	if idx := strings.Index(timePart, "M"); idx >= 0 {
		minutes, _ := strconv.ParseFloat(timePart[:idx], 64)
		total += minutes * 60
		timePart = timePart[idx+1:]
	}

	// Seconds
	if idx := strings.Index(timePart, "S"); idx >= 0 {
		seconds, _ := strconv.ParseFloat(timePart[:idx], 64)
		total += seconds
	}

	return total
}

// detectMediaType determines media type from DASH contentType/mimeType.
func detectMediaType(contentType, mimeType, codecs string) model.MediaType {
	ct := contentType
	if ct == "" {
		ct = mimeType
	}
	ct = strings.ToLower(ct)

	switch {
	case strings.HasPrefix(ct, "audio"):
		return model.MediaAudio
	case strings.HasPrefix(ct, "text"), strings.Contains(codecs, "stpp"), strings.Contains(codecs, "wvtt"):
		return model.MediaSubtitle
	default:
		return model.MediaVideo
	}
}

// ParseDuration parses Go-style or HH:mm:ss duration string.
func ParseDuration(s string) (time.Duration, error) {
	// Try Go duration first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// Try HH:mm:ss
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
	}
	return 0, fmt.Errorf("invalid duration: %s", s)
}
