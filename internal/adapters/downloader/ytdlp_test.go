package downloader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
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

// ----- AQ-005 / AQ-006: buildArgs, options, setter (no real yt-dlp) -----

// var _ ensures the Downloader implementation satisfies the frozen port
// (AQ-018 compile-time evidence).
var _ ports.Downloader = (*Downloader)(nil)

func argsIndex(args []string, want string) int {
	for i, a := range args {
		if a == want {
			return i
		}
	}
	return -1
}

func TestBuildArgs_NoBitrate(t *testing.T) {
	outputDir := t.TempDir()
	media := domain.Media{URL: "https://youtube.com/watch?v=abc", Title: "Some Song", Artist: "Some Artist"}

	args := buildArgs(media, outputDir, "")

	if idx := argsIndex(args, "--audio-quality"); idx != -1 {
		t.Errorf("args contain --audio-quality at %d with empty bitrate: %v", idx, args)
	}

	if idx := argsIndex(args, "-x"); idx == -1 {
		t.Errorf("args missing -x: %v", args)
	}
	if idx := argsIndex(args, "--audio-format"); idx == -1 || args[idx+1] != "mp3" {
		t.Errorf("args missing '--audio-format mp3': %v", args)
	}
	if argsIndex(args, "--embed-metadata") == -1 {
		t.Errorf("args missing --embed-metadata: %v", args)
	}
	if idx := argsIndex(args, "-o"); idx == -1 {
		t.Errorf("args missing -o: %v", args)
	} else if tmpl := args[idx+1]; !strings.HasPrefix(tmpl, outputDir) {
		t.Errorf("-o template %q not prefixed with outputDir %q", tmpl, outputDir)
	} else if !strings.Contains(tmpl, "%(artist)s - %(title)s.%(ext)s") {
		t.Errorf("-o template %q missing '%%(artist)s - %%(title)s.%%(ext)s'", tmpl)
	}
}

func TestBuildArgs_BitratePosition(t *testing.T) {
	outputDir := t.TempDir()
	media := domain.Media{URL: "https://youtube.com/watch?v=abc", Title: "Some Song", Artist: "Some Artist"}

	args := buildArgs(media, outputDir, "192k")

	idx := argsIndex(args, "--audio-format")
	if idx == -1 {
		t.Fatalf("args missing --audio-format: %v", args)
	}
	if args[idx+1] != "mp3" {
		t.Fatalf("args[%d] = %q, want \"mp3\" after --audio-format", idx+1, args[idx+1])
	}
	if args[idx+2] != "--audio-quality" || args[idx+3] != "192k" {
		t.Errorf("--audio-quality 192k not immediately after '--audio-format mp3': %v", args)
	}
}

func TestBuildArgs_ExistingFlagsIntact(t *testing.T) {
	outputDir := t.TempDir()
	media := domain.Media{URL: "https://youtube.com/watch?v=abc", Title: "Some Song", Artist: "Some Artist"}

	cases := []struct {
		name    string
		bitrate string
	}{
		{"no bitrate", ""},
		{"with bitrate", "320k"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			args := buildArgs(media, outputDir, tt.bitrate)

			for _, want := range []string{"-x", "--audio-format", "mp3", "--embed-metadata", "--embed-thumbnail", "--add-metadata", "-o", "--no-warnings", media.URL} {
				if argsIndex(args, want) == -1 {
					t.Errorf("args missing %q: %v", want, args)
				}
			}
		})
	}
}

func TestWithAudioBitrate_EachLevel(t *testing.T) {
	outputDir := t.TempDir()
	media := domain.Media{URL: "https://youtube.com/watch?v=abc", Title: "Some Song", Artist: "Some Artist"}

	cases := []struct {
		name string
		q    string
	}{
		{"128k", "128k"},
		{"192k", "192k"},
		{"320k", "320k"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDownloader(WithAudioBitrate(tt.q))
			args := buildArgs(media, outputDir, d.audioBitrate)

			idx := argsIndex(args, "--audio-quality")
			if idx == -1 {
				t.Fatalf("args missing --audio-quality for %q: %v", tt.q, args)
			}
			if args[idx+1] != tt.q {
				t.Errorf("--audio-quality value = %q, want %q", args[idx+1], tt.q)
			}
		})
	}
}

func TestNewDownloader_NoOption(t *testing.T) {
	outputDir := t.TempDir()
	media := domain.Media{URL: "https://youtube.com/watch?v=abc", Title: "Some Song", Artist: "Some Artist"}

	d := NewDownloader()
	if d.audioBitrate != "" {
		t.Fatalf("NewDownloader() audioBitrate = %q, want empty (pre-change behavior)", d.audioBitrate)
	}

	args := buildArgs(media, outputDir, d.audioBitrate)
	if argsIndex(args, "--audio-quality") != -1 {
		t.Errorf("NewDownloader() without options emits --audio-quality: %v", args)
	}
}

func TestSetAudioBitrate_MidSession(t *testing.T) {
	outputDir := t.TempDir()
	media := domain.Media{URL: "https://youtube.com/watch?v=abc", Title: "Some Song", Artist: "Some Artist"}

	d := NewDownloader(WithAudioBitrate("128k"))
	before := buildArgs(media, outputDir, d.audioBitrate)

	d.SetAudioBitrate("320k")
	after := buildArgs(media, outputDir, d.audioBitrate)

	if idx := argsIndex(before, "--audio-quality"); idx == -1 || before[idx+1] != "128k" {
		t.Errorf("before setter args should use 128k: %v", before)
	}
	if idx := argsIndex(after, "--audio-quality"); idx == -1 || after[idx+1] != "320k" {
		t.Errorf("after setter args should use 320k: %v", after)
	}
}

// TestAudioBitrateConcurrentAccess exercises SetAudioBitrate (write) against
// Download (read) to prove the bitrate field is safe under concurrent access.
// The downloader binary is pointed at a path that does not exist so Download
// fails fast after reading the bitrate — no network, no yt-dlp required.
func TestAudioBitrateConcurrentAccess(t *testing.T) {
	d := NewDownloader()
	d.binary = filepath.Join(t.TempDir(), "yt-dlp-missing")

	media := domain.Media{URL: "https://youtube.com/watch?v=abc", Title: "Some Song", Artist: "Some Artist"}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			d.SetAudioBitrate("128k")
			_ = i
		}(i)
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = d.Download(context.Background(), media, t.TempDir())
		}()
	}
	wg.Wait()
}

func TestBuildArgs_OptionTerminatorBeforeURL(t *testing.T) {
	outputDir := t.TempDir()
	media := domain.Media{URL: "https://youtube.com/watch?v=abc", Title: "T", Artist: "A"}

	args := buildArgs(media, outputDir, "")

	// The "--" option terminator must directly precede the URL so arbitrary
	// pasted input starting with "-" is treated as a URL, never as an option.
	if len(args) < 2 || args[len(args)-2] != "--" || args[len(args)-1] != media.URL {
		t.Fatalf("expected \"--\" immediately before the URL, got tail: %v", args[len(args)-2:])
	}
}
