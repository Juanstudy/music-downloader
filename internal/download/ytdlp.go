package download

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Juanstudy/music-downloader/internal/model"
)

// YtDlpEngine invokes yt-dlp as a subprocess.
type YtDlpEngine struct {
	BinaryPath string // defaults to "yt-dlp" looked up on $PATH
}

// ytDlpTrack is the minimal JSON output from --flat-playlist --dump-json.
type ytDlpTrack struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
	Channel  string  `json:"channel"`
	Webpage  string  `json:"webpage_url"`
}

// CheckInstalled verifies yt-dlp is on $PATH.
func (e *YtDlpEngine) CheckInstalled() error {
	binary := e.BinaryPath
	if binary == "" {
		binary = "yt-dlp"
	}
	_, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("yt-dlp not found on $PATH: %w", err)
	}
	// Also check ffmpeg (required by yt-dlp for audio conversion).
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found on $PATH (required by yt-dlp): %w", err)
	}
	return nil
}

// Resolve fetches metadata for a URL using --flat-playlist --dump-json.
func (e *YtDlpEngine) Resolve(url string) ([]*model.Media, error) {
	binary := e.BinaryPath
	if binary == "" {
		binary = "yt-dlp"
	}

	cmd := exec.Command(binary,
		"--flat-playlist",
		"--dump-json",
		"--no-warnings",
		url,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start yt-dlp: %w", err)
	}

	// Read stderr for error messages (yt-dlp outputs progress there).
	errOutput := new(strings.Builder)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			errOutput.WriteString(scanner.Text())
			errOutput.WriteString("\n")
		}
	}()

	var tracks []*model.Media
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		var raw ytDlpTrack
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue // skip lines that aren't valid JSON
		}
		track := &model.Media{
			URL:      raw.Webpage,
			Title:    raw.Title,
			Artist:   raw.Channel,
			Duration: formatDuration(raw.Duration),
			Source:   "youtube",
			Status:   model.StatusPending,
		}
		tracks = append(tracks, track)
	}

	if err := cmd.Wait(); err != nil {
		// If we got no tracks, it's a real error.
		if len(tracks) == 0 {
			return nil, fmt.Errorf("yt-dlp failed: %w\n%s", err, errOutput.String())
		}
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("no tracks found at URL")
	}

	return tracks, nil
}

// Download downloads a single track as MP3 with embedded metadata.
func (e *YtDlpEngine) Download(track *model.Media, outputDir string, progress chan<- string) error {
	binary := e.BinaryPath
	if binary == "" {
		binary = "yt-dlp"
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Pattern: {artist} - {title}.mp3 per ADR-005.
	outputTemplate := filepath.Join(outputDir, "%(artist)s - %(title)s.%(ext)s")

	args := []string{
		"-x", // extract audio
		"--audio-format", "mp3",
		"--embed-metadata",
		"--embed-thumbnail",
		"--add-metadata",
		"-o", outputTemplate,
		"--no-warnings",
		track.URL,
	}

	if progress != nil {
		defer close(progress)
	}

	cmd := exec.Command(binary, args...)

	// Capture stderr for progress info (yt-dlp outputs progress to stderr).
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start yt-dlp: %w", err)
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if progress != nil {
			progress <- line
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	return nil
}

// formatDuration converts seconds (float64) to "m:ss" format.
func formatDuration(seconds float64) string {
	mins := int(seconds) / 60
	secs := int(seconds) % 60
	return fmt.Sprintf("%d:%02d", mins, secs)
}
