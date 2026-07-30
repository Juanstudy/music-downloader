package ports

import (
	"testing"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
)

func TestSearchResult_ZeroValue(t *testing.T) {
	var sr SearchResult

	if sr.Tracks != nil {
		t.Errorf("SearchResult.Tracks zero value = %v, want nil", sr.Tracks)
	}
	if sr.Source != "" {
		t.Errorf("SearchResult.Source zero value = %q, want empty", sr.Source)
	}
}

func TestSearchResult_StructLiteral(t *testing.T) {
	sr := SearchResult{
		Tracks: []domain.Media{
			{URL: "https://youtube.com/watch?v=1", Title: "Track 1"},
			{URL: "https://youtube.com/watch?v=2", Title: "Track 2"},
		},
		Source: "youtube",
	}

	if len(sr.Tracks) != 2 {
		t.Errorf("len(SearchResult.Tracks) = %d, want 2", len(sr.Tracks))
	}
	if sr.Tracks[0].Title != "Track 1" {
		t.Errorf("SearchResult.Tracks[0].Title = %q, want %q", sr.Tracks[0].Title, "Track 1")
	}
	if sr.Source != "youtube" {
		t.Errorf("SearchResult.Source = %q, want %q", sr.Source, "youtube")
	}
}
