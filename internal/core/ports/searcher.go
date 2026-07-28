package ports

import (
	"context"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
)

// SearchResult holds the tracks found for a given URL and their source.
type SearchResult struct {
	Tracks []domain.Media
	Source string
}

// Searcher resolves a URL into playable tracks.
type Searcher interface {
	Search(ctx context.Context, url string) (SearchResult, error)
}
