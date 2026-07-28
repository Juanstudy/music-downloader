package downloader

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
)

func TestDownloader_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp + ffmpeg)")
	}

	outputDir := t.TempDir()
	d := NewDownloader()

	media := domain.Media{
		URL:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Title:  "Rick Astley - Never Gonna Give You Up",
		Artist: "Rick Astley",
	}

	result, err := d.Download(context.Background(), media, outputDir)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	if result.OutputPath == "" {
		t.Error("expected non-empty output path")
	}
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Errorf("output file does not exist: %s", result.OutputPath)
	}
}

func TestDownloader_NonExistentURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp)")
	}

	outputDir := t.TempDir()
	d := NewDownloader()

	media := domain.Media{
		URL:    "https://www.youtube.com/watch?v=thisdoesnotexist",
		Title:  "Non-existent",
		Artist: "Unknown",
	}

	_, err := d.Download(context.Background(), media, outputDir)
	if err == nil {
		t.Error("expected error for non-existent URL")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"already safe", "Hello World", "Hello World"},
		{"removes slashes", "a/b/c", "a_b_c"},
		{"removes colons", "Title: Subtitle", "Title_ Subtitle"},
		{"keeps dots and dashes", "file-name.ext", "file-name.ext"},
		{"trims spaces", "  spaced  ", "spaced"},
		{"unicode to underscore", "caf\u00e9", "caf_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDownloader_OutputFileCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (requires yt-dlp + ffmpeg)")
	}

	outputDir := t.TempDir()
	d := NewDownloader()

	// Create a dummy output file as if yt-dlp had created it
	dummyPath := filepath.Join(outputDir, "Rick Astley - Never Gonna Give You Up.mp3")
	if err := os.WriteFile(dummyPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	media := domain.Media{
		URL:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Title:  "Never Gonna Give You Up",
		Artist: "Rick Astley",
	}

	// By creating the file beforehand, we simulate a successful download
	// and test that the output path resolution works
	result, err := d.Download(context.Background(), media, outputDir)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	if result.OutputPath == "" {
		t.Error("expected non-empty output path")
	}
}
