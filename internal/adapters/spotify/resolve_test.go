package spotify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// mockSearcher implements ports.Searcher for testing, returning a fixed result
// or error.
type mockSearcher struct {
	result ports.SearchResult
	err    error
}

func (m *mockSearcher) Search(_ context.Context, _ string) (ports.SearchResult, error) {
	return m.result, m.err
}

func TestResolveTrack_Success(t *testing.T) {
	ytSearcher := &mockSearcher{
		result: ports.SearchResult{
			Tracks: []domain.Media{
				{
					URL:      "https://youtube.com/watch?v=test123",
					Title:    "YouTube Title",
					Artist:   "YouTube Artist",
					Duration: 200 * time.Second,
					Source:   "youtube",
				},
			},
		},
	}

	track := domain.Media{
		Title:  "Original Title",
		Artist: "Original Artist",
		Source: "spotify",
		Status: domain.StatusPending,
	}

	resolved, err := resolveTrack(context.Background(), track, ytSearcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Source must remain "spotify" — never becomes "youtube"
	if resolved.Source != "spotify" {
		t.Errorf("expected Source 'spotify', got %q", resolved.Source)
	}
	// Title and Artist preserved from Spotify
	if resolved.Title != "Original Title" {
		t.Errorf("expected Title 'Original Title', got %q", resolved.Title)
	}
	if resolved.Artist != "Original Artist" {
		t.Errorf("expected Artist 'Original Artist', got %q", resolved.Artist)
	}
	// URL comes from YouTube
	if resolved.URL != "https://youtube.com/watch?v=test123" {
		t.Errorf("expected URL from YouTube, got %q", resolved.URL)
	}
	// Duration comes from YouTube
	if resolved.Duration != 200*time.Second {
		t.Errorf("expected Duration 200s, got %v", resolved.Duration)
	}
	// Status remains pending (successful resolution)
	if resolved.Status != domain.StatusPending {
		t.Errorf("expected StatusPending, got %v", resolved.Status)
	}
}

func TestResolveTrack_NoResults(t *testing.T) {
	ytSearcher := &mockSearcher{
		result: ports.SearchResult{
			Tracks: []domain.Media{},
		},
	}

	track := domain.Media{
		Title:  "Some Track",
		Artist: "Some Artist",
		Source: "spotify",
		Status: domain.StatusPending,
	}

	resolved, err := resolveTrack(context.Background(), track, ytSearcher)
	if err != nil {
		t.Fatalf("expected nil error for no results, got: %v", err)
	}

	if resolved.Status != domain.StatusFailed {
		t.Errorf("expected StatusFailed, got %v", resolved.Status)
	}
	if resolved.Error == "" {
		t.Error("expected non-empty error message on track")
	}
	// Original metadata preserved
	if resolved.Title != "Some Track" {
		t.Errorf("expected Title preserved, got %q", resolved.Title)
	}
	if resolved.Artist != "Some Artist" {
		t.Errorf("expected Artist preserved, got %q", resolved.Artist)
	}
	if resolved.Source != "spotify" {
		t.Errorf("expected Source 'spotify', got %q", resolved.Source)
	}
}

func TestResolveTrack_SearcherError(t *testing.T) {
	expectedErr := errors.New("yt-dlp not found")
	ytSearcher := &mockSearcher{
		err: expectedErr,
	}

	track := domain.Media{
		Title:  "Track",
		Artist: "Artist",
	}

	_, err := resolveTrack(context.Background(), track, ytSearcher)
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestResolveTrack_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ytSearcher := &mockSearcher{
		err: context.Canceled,
	}

	track := domain.Media{
		Title:  "Track",
		Artist: "Artist",
	}

	_, err := resolveTrack(ctx, track, ytSearcher)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
