package ports

import (
	"context"
	"testing"
)

type stubQuerySearcher struct{}

func (s *stubQuerySearcher) SearchByQuery(ctx context.Context, query string, limit int) (SearchResult, error) {
	return SearchResult{}, nil
}

func TestQuerySearcherInterface(t *testing.T) {
	var _ QuerySearcher = (*stubQuerySearcher)(nil)
}
