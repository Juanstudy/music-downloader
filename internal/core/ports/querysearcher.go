package ports

import "context"

// QuerySearcher searches YouTube Music by free-text query.
type QuerySearcher interface {
	// SearchByQuery searches YouTube Music for the given query and returns up to
	// limit results. If limit <= 0, a default of 10 is used.
	// Returns the same SearchResult type used by Searcher.Search with
	// Source set to "youtube-music".
	SearchByQuery(ctx context.Context, query string, limit int) (SearchResult, error)
}
