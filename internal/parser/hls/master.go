package hls

import (
	"strconv"
	"strings"

	"github.com/caorushizi/mediago-core/internal/model"
)

// ParseMasterPlaylist parses an HLS master playlist and returns a list of variant streams.
func ParseMasterPlaylist(content string, baseURL string) []model.StreamSpec {
	var streams []model.StreamSpec
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, TagStreamInf) {
			spec := model.StreamSpec{
				MediaType: model.MediaVideo,
			}

			if bw := GetAttribute(line, "BANDWIDTH"); bw != "" {
				spec.Bandwidth, _ = strconv.ParseInt(bw, 10, 64)
			}
			if avgBw := GetAttribute(line, "AVERAGE-BANDWIDTH"); avgBw != "" {
				spec.Bandwidth, _ = strconv.ParseInt(avgBw, 10, 64)
			}
			spec.Codecs = GetAttribute(line, "CODECS")
			spec.Resolution = GetAttribute(line, "RESOLUTION")
			if fr := GetAttribute(line, "FRAME-RATE"); fr != "" {
				spec.FrameRate, _ = strconv.ParseFloat(fr, 64)
			}

			// Next non-empty, non-comment line is the playlist URL
			for i++; i < len(lines); i++ {
				next := strings.TrimSpace(lines[i])
				if next != "" && !strings.HasPrefix(next, "#") {
					spec.URL = ResolveURL(baseURL, next)
					break
				}
			}
			streams = append(streams, spec)

		} else if strings.HasPrefix(line, TagMedia) {
			mediaType := GetAttribute(line, "TYPE")
			uri := GetAttribute(line, "URI")

			// Skip if no URI (default rendition embedded in video)
			if uri == "" {
				continue
			}

			spec := model.StreamSpec{
				GroupID:  GetAttribute(line, "GROUP-ID"),
				Language: GetAttribute(line, "LANGUAGE"),
				Name:     GetAttribute(line, "NAME"),
				URL:      ResolveURL(baseURL, uri),
			}

			switch strings.ToUpper(mediaType) {
			case "AUDIO":
				spec.MediaType = model.MediaAudio
				spec.Channels = GetAttribute(line, "CHANNELS")
			case "SUBTITLES":
				spec.MediaType = model.MediaSubtitle
			default:
				continue
			}
			streams = append(streams, spec)
		}
	}

	return streams
}

// IsMasterPlaylist checks if the content is a master playlist.
func IsMasterPlaylist(content string) bool {
	return strings.Contains(content, TagStreamInf)
}
