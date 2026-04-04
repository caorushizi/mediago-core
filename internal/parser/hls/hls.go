package hls

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/caorushizi/mediago-core/internal/model"
)

// Parser implements the parser.Parser interface for HLS streams.
type Parser struct {
	Client *http.Client
}

// Parse fetches and parses an HLS URL, returning streams with their segment lists.
func (p *Parser) Parse(ctx context.Context, url string, headers map[string]string) (*model.ParseResult, error) {
	content, err := p.fetch(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("fetch m3u8: %w", err)
	}

	result := &model.ParseResult{}

	if IsMasterPlaylist(content) {
		streams := ParseMasterPlaylist(content, url)

		// Fetch each stream's media playlist
		for i := range streams {
			s := &streams[i]
			if s.URL == "" {
				continue
			}
			mediaContent, err := p.fetch(ctx, s.URL, headers)
			if err != nil {
				return nil, fmt.Errorf("fetch media playlist for %s: %w", s.URL, err)
			}
			playlist, err := ParseMediaPlaylist(mediaContent, s.URL)
			if err != nil {
				return nil, fmt.Errorf("parse media playlist: %w", err)
			}
			s.Playlist = playlist

			if playlist.IsLive {
				result.IsLive = true
			}
		}

		result.Streams = streams
	} else {
		// Single media playlist
		playlist, err := ParseMediaPlaylist(content, url)
		if err != nil {
			return nil, fmt.Errorf("parse media playlist: %w", err)
		}

		stream := model.StreamSpec{
			MediaType: model.MediaVideo,
			URL:       url,
			Playlist:  playlist,
		}
		result.Streams = []model.StreamSpec{stream}
		result.IsLive = playlist.IsLive
	}

	// Determine merge type based on content
	result.MergeType = detectMergeType(result.Streams)

	return result, nil
}

// detectMergeType checks if streams use fMP4 (binary merge) or TS (ffmpeg merge).
func detectMergeType(streams []model.StreamSpec) model.MergeType {
	for _, s := range streams {
		if s.Playlist != nil && s.Playlist.MediaInit != nil {
			return model.MergeBinary // fMP4 with init segment
		}
	}
	return model.MergeFFmpeg // TS segments
}

// fetch downloads content from a URL with custom headers.
func (p *Parser) fetch(ctx context.Context, url string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
