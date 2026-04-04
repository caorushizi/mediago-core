package dash

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/caorushizi/mediago-core/internal/model"
)

// Parser implements the parser.Parser interface for DASH streams.
type Parser struct {
	Client *http.Client
}

// Parse fetches and parses a DASH MPD URL.
func (p *Parser) Parse(ctx context.Context, url string, headers map[string]string) (*model.ParseResult, error) {
	content, err := p.fetch(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("fetch MPD: %w", err)
	}
	return ParseMPD(content, url)
}

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
