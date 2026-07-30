package downloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// Downloader invokes yt-dlp to download a single track as MP3.
type Downloader struct {
	binary string
}

// NewDownloader creates a Downloader that uses the system yt-dlp binary.
func NewDownloader() *Downloader {
	return &Downloader{binary: "yt-dlp"}
}

// Download downloads media to outputDir as an MP3 file with embedded metadata.
// It returns a DownloadResult containing the resolved output file path.
func (d *Downloader) Download(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return ports.DownloadResult{}, fmt.Errorf("create output dir: %w", err)
	}

	outputTemplate := filepath.Join(outputDir, "%(artist)s - %(title)s.%(ext)s")

	args := []string{
		"-x", "--audio-format", "mp3",
		"--embed-metadata",
		"--embed-thumbnail",
		"--add-metadata",
		"-o", outputTemplate,
		"--no-warnings",
		media.URL,
	}

	cmd := exec.CommandContext(ctx, d.binary, args...)
	cmd.Stdout = io.Discard // prevent yt-dlp output from corrupting the TUI
	stderr := new(strings.Builder)
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		return ports.DownloadResult{}, fmt.Errorf("download failed: %w\n%s", err, errOutput)
	}

	// Build output path from media metadata
	safeTitle := sanitizeFilename(media.Title)
	safeArtist := sanitizeFilename(media.Artist)
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s - %s.mp3", safeArtist, safeTitle))

	// Check if file exists with slightly different naming (yt-dlp may sanitize differently)
	// If the expected file doesn't exist, try to find any .mp3 in outputDir that contains the title
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		// yt-dlp may have used a different sanitization; find by pattern
		entries, _ := filepath.Glob(filepath.Join(outputDir, "*.mp3"))
		for _, entry := range entries {
			if strings.Contains(strings.ToLower(entry), strings.ToLower(safeTitle)) {
				outputPath = entry
				break
			}
		}
	}

	return ports.DownloadResult{
		Media:      media,
		OutputPath: outputPath,
	}, nil
}

// sanitizeFilename replaces characters that are problematic in filenames.
func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_' || r == '.' || r == ' ':
			return r
		default:
			return '_'
		}
	}, name)
	return strings.TrimSpace(name)
}
