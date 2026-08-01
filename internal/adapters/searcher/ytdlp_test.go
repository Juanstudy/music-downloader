package searcher

import (
	"context"
	"strings"
	"testing"
)

func TestSearcher_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp)")
	}

	s := NewSearcher()

	// Test with a known URL that returns a single video
	result, err := s.Search(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Tracks) == 0 {
		t.Fatal("expected at least one track")
	}
	if result.Tracks[0].Title == "" {
		t.Error("expected non-empty title")
	}
	if result.Source != "youtube" {
		t.Errorf("expected source 'youtube', got %q", result.Source)
	}
}

func TestSearcher_NonExistentURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp)")
	}

	s := NewSearcher()
	_, err := s.Search(context.Background(), "https://www.youtube.com/watch?v=thisdoesnotexist12345")
	if err == nil {
		t.Error("expected error for non-existent URL")
	}
}

func TestSearcher_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp)")
	}

	s := NewSearcher()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	_, err := s.Search(ctx, "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestSearcher_YouTubeMusicSource(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp)")
	}

	s := NewSearcher()
	result, err := s.Search(context.Background(),
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if !strings.Contains(result.Source, "youtube") {
		t.Errorf("expected youtube source, got %q", result.Source)
	}
}

func TestSearchArgs_OptionTerminatorBeforeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "regular URL", url: "https://youtube.com/watch?v=dQw4w9WgXcQ"},
		{name: "option-looking input", url: "--config-location=http://evil.example/x"},
		{name: "dash-prefixed input", url: "--output"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := searchArgs(tt.url)
			if len(args) < 2 || args[len(args)-2] != "--" || args[len(args)-1] != tt.url {
				t.Fatalf("expected \"--\" immediately before the URL %q, got tail: %v", tt.url, args[len(args)-2:])
			}
		})
	}
}
