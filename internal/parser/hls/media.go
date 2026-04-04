package hls

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/caorushizi/mediago-core/internal/model"
)

// ParseMediaPlaylist parses an HLS media playlist and returns a Playlist with segments.
func ParseMediaPlaylist(content string, baseURL string) (*model.Playlist, error) {
	playlist := &model.Playlist{}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	var (
		segIndex       int
		currentEncrypt *model.EncryptInfo
		currentSeg     *model.Segment
		expectSegment  bool
		prevRange      int64 // tracks end of previous byte-range for consecutive ranges
		isEndList      bool
	)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, TagTargetDuration):
			val := GetAttribute(line, "")
			playlist.TargetDuration, _ = strconv.ParseFloat(val, 64)

		case strings.HasPrefix(line, TagMediaSequence):
			val := GetAttribute(line, "")
			segIndex, _ = strconv.Atoi(val)

		case strings.HasPrefix(line, TagPlaylistType):
			val := GetAttribute(line, "")
			if strings.ToUpper(val) == "VOD" {
				isEndList = true
			}

		case strings.HasPrefix(line, TagKey):
			enc, err := parseEncryptInfo(line, baseURL)
			if err != nil {
				return nil, fmt.Errorf("parse key: %w", err)
			}
			currentEncrypt = enc

		case strings.HasPrefix(line, TagMap):
			uri := GetAttribute(line, "URI")
			if uri == "" {
				continue
			}
			initSeg := &model.Segment{
				Index: -1,
				URL:   ResolveURL(baseURL, uri),
			}
			// Parse byte-range for init segment
			if br := GetAttribute(line, "BYTERANGE"); br != "" {
				start, stop := parseByteRange(br, 0)
				initSeg.StartRange = start
				initSeg.StopRange = stop
			}
			if currentEncrypt != nil {
				initSeg.EncryptInfo = copyEncryptInfo(currentEncrypt)
			}
			playlist.MediaInit = initSeg

		case strings.HasPrefix(line, TagByteRange):
			val := GetAttribute(line, "")
			if currentSeg != nil {
				start, stop := parseByteRange(val, prevRange)
				currentSeg.StartRange = start
				currentSeg.StopRange = stop
				prevRange = stop + 1
			}

		case strings.HasPrefix(line, TagExtInf):
			val := GetAttribute(line, "")
			// Duration is before the comma
			durStr := val
			if idx := strings.Index(val, ","); idx >= 0 {
				durStr = val[:idx]
			}
			dur, _ := strconv.ParseFloat(durStr, 64)
			playlist.TotalDuration += dur

			currentSeg = &model.Segment{
				Index:    segIndex,
				Duration: dur,
			}
			if currentEncrypt != nil {
				iv := currentEncrypt.IV
				if iv == nil {
					iv = segIndexToIV(segIndex)
				}
				currentSeg.EncryptInfo = &model.EncryptInfo{
					Method: currentEncrypt.Method,
					KeyURL: currentEncrypt.KeyURL,
					Key:    currentEncrypt.Key,
					IV:     iv,
				}
			}
			segIndex++
			expectSegment = true

		case strings.HasPrefix(line, TagEndList):
			isEndList = true

		case strings.HasPrefix(line, TagDiscontinuity):
			// Continue to next segment, no special handling needed in simplified version

		case !strings.HasPrefix(line, "#") && expectSegment:
			if currentSeg != nil {
				currentSeg.URL = ResolveURL(baseURL, line)
				playlist.Segments = append(playlist.Segments, *currentSeg)
				currentSeg = nil
			}
			expectSegment = false

		case !strings.HasPrefix(line, "#") && !expectSegment:
			// Standalone URL without #EXTINF (rare but handle gracefully)
		}
	}

	playlist.IsLive = !isEndList

	return playlist, nil
}

// parseEncryptInfo extracts encryption details from a #EXT-X-KEY line.
func parseEncryptInfo(line, baseURL string) (*model.EncryptInfo, error) {
	method := GetAttribute(line, "METHOD")
	if method == "" || strings.ToUpper(method) == "NONE" {
		return nil, nil
	}

	info := &model.EncryptInfo{}

	switch strings.ToUpper(method) {
	case "AES-128":
		info.Method = model.EncryptAES128
	default:
		return nil, fmt.Errorf("unsupported encryption method: %s", method)
	}

	uri := GetAttribute(line, "URI")
	if uri != "" {
		info.KeyURL = ResolveURL(baseURL, uri)
	}

	ivStr := GetAttribute(line, "IV")
	if ivStr != "" {
		ivStr = strings.TrimPrefix(ivStr, "0x")
		ivStr = strings.TrimPrefix(ivStr, "0X")
		ivBytes, err := hex.DecodeString(ivStr)
		if err != nil {
			return nil, fmt.Errorf("decode IV: %w", err)
		}
		info.IV = ivBytes
	}

	return info, nil
}

// parseByteRange parses a BYTERANGE value like "1024@0" or "1024".
// Returns (startRange, stopRange).
func parseByteRange(val string, prevEnd int64) (int64, int64) {
	parts := strings.SplitN(val, "@", 2)
	length, _ := strconv.ParseInt(parts[0], 10, 64)

	var offset int64
	if len(parts) == 2 {
		offset, _ = strconv.ParseInt(parts[1], 10, 64)
	} else {
		offset = prevEnd
	}
	return offset, offset + length - 1
}

// segIndexToIV converts a segment index to a 16-byte IV.
// This is the default IV when none is specified in the playlist.
func segIndexToIV(index int) []byte {
	iv := make([]byte, 16)
	hexStr := fmt.Sprintf("%032x", index)
	decoded, _ := hex.DecodeString(hexStr)
	copy(iv, decoded)
	return iv
}

// copyEncryptInfo creates a copy of EncryptInfo.
func copyEncryptInfo(src *model.EncryptInfo) *model.EncryptInfo {
	if src == nil {
		return nil
	}
	dst := &model.EncryptInfo{
		Method: src.Method,
		KeyURL: src.KeyURL,
	}
	if src.Key != nil {
		dst.Key = make([]byte, len(src.Key))
		copy(dst.Key, src.Key)
	}
	if src.IV != nil {
		dst.IV = make([]byte, len(src.IV))
		copy(dst.IV, src.IV)
	}
	return dst
}
