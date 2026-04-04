package parser

import (
	"context"

	"github.com/caorushizi/mediago-core/internal/model"
)

// Parser extracts stream and segment information from a URL.
type Parser interface {
	Parse(ctx context.Context, url string, headers map[string]string) (*model.ParseResult, error)
}
