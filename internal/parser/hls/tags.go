package hls

import (
	"net/url"
	"strings"
)

// HLS tag constants.
const (
	TagExtM3U             = "#EXTM3U"
	TagExtInf             = "#EXTINF"
	TagStreamInf          = "#EXT-X-STREAM-INF"
	TagMedia              = "#EXT-X-MEDIA"
	TagKey                = "#EXT-X-KEY"
	TagMap                = "#EXT-X-MAP"
	TagByteRange          = "#EXT-X-BYTERANGE"
	TagTargetDuration     = "#EXT-X-TARGETDURATION"
	TagMediaSequence      = "#EXT-X-MEDIA-SEQUENCE"
	TagDiscontinuity      = "#EXT-X-DISCONTINUITY"
	TagEndList            = "#EXT-X-ENDLIST"
	TagPlaylistType       = "#EXT-X-PLAYLIST-TYPE"
	TagProgramDateTime    = "#EXT-X-PROGRAM-DATE-TIME"
)

// GetAttribute extracts the value of a key from an HLS tag line.
// For example, given `#EXT-X-KEY:METHOD=AES-128,URI="key.bin"` and key "METHOD",
// returns "AES-128".
func GetAttribute(line, key string) string {
	// Find the attribute section after the colon
	idx := strings.Index(line, ":")
	if idx < 0 {
		return ""
	}
	attrs := line[idx+1:]

	if key == "" {
		return strings.TrimSpace(attrs)
	}

	search := key + "="
	pos := strings.Index(attrs, search)
	if pos < 0 {
		return ""
	}
	val := attrs[pos+len(search):]

	// Quoted value
	if len(val) > 0 && val[0] == '"' {
		end := strings.Index(val[1:], "\"")
		if end < 0 {
			return val[1:]
		}
		return val[1 : end+1]
	}

	// Unquoted value: read until comma
	end := strings.Index(val, ",")
	if end < 0 {
		return strings.TrimSpace(val)
	}
	return strings.TrimSpace(val[:end])
}

// ResolveURL resolves a possibly relative URL against a base URL.
func ResolveURL(baseURL, ref string) string {
	if ref == "" {
		return baseURL
	}
	// Already absolute
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(refURL).String()
}
