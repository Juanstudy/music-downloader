package downloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// Downloader invokes yt-dlp to download a single track as MP3.
type Downloader struct {
	binary string

	mu sync.Mutex // guards audioBitrate: downloads run in goroutines while the TUI may change quality mid-session

	audioBitrate string // "" = no --audio-bitrate flag (pre-change behavior)
}

// Option configures a Downloader at construction time.
type Option func(*Downloader)

// WithAudioBitrate sets the MP3 bitrate used for subsequent downloads.
func WithAudioBitrate(q string) Option {
	return func(d *Downloader) {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.audioBitrate = q
	}
}

// NewDownloader creates a Downloader that uses the system yt-dlp binary.
// Options are applied in order; calling with no options keeps the pre-change
// no-bitrate behavior.
func NewDownloader(opts ...Option) *Downloader {
	d := &Downloader{binary: "yt-dlp"}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// SetAudioBitrate changes the bitrate used for subsequent downloads mid-session.
// Safe to call while downloads are in flight: each download snapshots the value
// once at start, so an in-flight download keeps the bitrate it was launched with.
func (d *Downloader) SetAudioBitrate(q string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.audioBitrate = q
}

// var _ pins the Downloader implementation to the frozen Downloader port
// (AQ-018 compile-time evidence).
var _ ports.Downloader = (*Downloader)(nil)

// Download downloads media to outputDir as an MP3 file with embedded metadata.
// It returns a DownloadResult containing the resolved output file path.
func (d *Downloader) Download(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return ports.DownloadResult{}, fmt.Errorf("create output dir: %w", err)
	}

	args := buildArgs(media, outputDir, d.bitrateSnapshot())

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

// bitrateSnapshot returns the current bitrate value, safe for concurrent
// access with SetAudioBitrate.
func (d *Downloader) bitrateSnapshot() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.audioBitrate
}

// buildArgs returns the yt-dlp invocation arguments. When bitrate is non-empty,
// --audio-bitrate <bitrate> is inserted immediately after --audio-format mp3.
// When bitrate is empty the args are byte-for-byte identical to the pre-change
// invocation. Pure function (no receiver, no I/O) — unit-testable without yt-dlp.
func buildArgs(media domain.Media, outputDir, bitrate string) []string {
	outputTemplate := filepath.Join(outputDir, "%(artist)s - %(title)s.%(ext)s")
	args := []string{"-x", "--audio-format", "mp3"}
	if bitrate != "" {
		args = append(args, "--audio-bitrate", bitrate)
	}
	args = append(args,
		"--embed-metadata",
		"--embed-thumbnail",
		"--add-metadata",
		"-o", outputTemplate,
		"--no-warnings",
		media.URL,
	)
	return args
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
