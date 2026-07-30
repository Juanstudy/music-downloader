package querysearcher

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// Compile-time interface compliance check.
var _ ports.QuerySearcher = (*QuerySearcher)(nil)

func TestEmptyQuery_ReturnsError(t *testing.T) {
	s := &QuerySearcher{binary: "echo"}
	_, err := s.SearchByQuery(context.Background(), "", 10)
	if err == nil {
		t.Fatal("expected error for empty query")
	}

	var domainErr domain.Error
	if !errors.As(err, &domainErr) {
		t.Fatalf("expected domain.Error, got %T", err)
	}
	if domainErr.Code != domain.ErrorInvalidURL {
		t.Errorf("expected ErrorInvalidURL, got %v", domainErr.Code)
	}
	if domainErr.Message != "query must not be empty" {
		t.Errorf("expected message 'query must not be empty', got %q", domainErr.Message)
	}
}

func TestWhitespaceQuery_ReturnsError(t *testing.T) {
	s := &QuerySearcher{binary: "echo"}
	_, err := s.SearchByQuery(context.Background(), "   ", 10)
	if err == nil {
		t.Fatal("expected error for whitespace-only query")
	}

	var domainErr domain.Error
	if !errors.As(err, &domainErr) {
		t.Fatalf("expected domain.Error, got %T", err)
	}
	if domainErr.Code != domain.ErrorInvalidURL {
		t.Errorf("expected ErrorInvalidURL, got %v", domainErr.Code)
	}
}

func TestLimitDefault_DoesNotError(t *testing.T) {
	s := &QuerySearcher{binary: "echo"}
	// limit=0 should default to 10 without error
	_, err := s.SearchByQuery(context.Background(), "test", 0)
	if err != nil {
		t.Fatalf("unexpected error with limit=0: %v", err)
	}

	// limit negative should also default
	_, err = s.SearchByQuery(context.Background(), "test", -5)
	if err != nil {
		t.Fatalf("unexpected error with limit=-5: %v", err)
	}
}

func TestSearchResult_SourceIsYoutubeMusic(t *testing.T) {
	s := &QuerySearcher{binary: "echo"}
	result, err := s.SearchByQuery(context.Background(), "test", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "youtube-music" {
		t.Errorf("expected source 'youtube-music', got %q", result.Source)
	}
}

// ---------------------------------------------------------------------------
// Integration tests (require yt-dlp on $PATH)
// ---------------------------------------------------------------------------

func TestSearch_ValidQuery_ReturnsResults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp)")
	}

	s := NewQuerySearcher()
	result, err := s.SearchByQuery(context.Background(), "never gonna give you up", 3)
	if err != nil {
		if strings.Contains(err.Error(), "Unsupported url scheme") || strings.Contains(err.Error(), "HTTP Error") {
			t.Skipf("yt-dlp search not supported: %v", err)
		}
		t.Fatalf("SearchByQuery failed: %v", err)
	}
	if result.Source != "youtube-music" {
		t.Errorf("expected source 'youtube-music', got %q", result.Source)
	}
	t.Logf("found %d tracks", len(result.Tracks))
}

func TestSearch_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp)")
	}

	s := NewQuerySearcher()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	_, err := s.SearchByQuery(ctx, "test", 5)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestSearch_SpecialCharacters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp)")
	}

	s := NewQuerySearcher()
	result, err := s.SearchByQuery(context.Background(), "rock & roll!", 5)
	if err != nil {
		if strings.Contains(err.Error(), "Unsupported url scheme") || strings.Contains(err.Error(), "HTTP Error") {
			t.Skipf("yt-dlp search not supported: %v", err)
		}
		t.Fatalf("SearchByQuery failed: %v", err)
	}
	if result.Source != "youtube-music" {
		t.Errorf("expected source 'youtube-music', got %q", result.Source)
	}
	t.Logf("found %d tracks", len(result.Tracks))
}

func TestSearch_VeryLongQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp)")
	}

	s := NewQuerySearcher()
	longQuery := strings.Repeat("a", 2000)
	result, err := s.SearchByQuery(context.Background(), longQuery, 5)
	// Should not crash; may return empty or error depending on yt-dlp
	if err != nil {
		t.Logf("long query returned error (acceptable): %v", err)
	}
	_ = result
}
