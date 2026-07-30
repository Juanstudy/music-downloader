package searcher

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// Searcher invokes yt-dlp to resolve a URL into playable tracks.
type Searcher struct {
	binary string
}

// NewSearcher creates a Searcher that uses the system yt-dlp binary.
func NewSearcher() *Searcher {
	return &Searcher{binary: "yt-dlp"}
}

// Search resolves url by running yt-dlp --flat-playlist --dump-json.
// Each JSON line is parsed via ParseLine into a domain.Media.
// Non-parseable lines are silently skipped.
func (s *Searcher) Search(ctx context.Context, url string) (ports.SearchResult, error) {
	args := []string{
		"--flat-playlist",
		"--dump-json",
		"--ignore-errors",
		"--no-warnings",
		url,
	}

	cmd := exec.CommandContext(ctx, s.binary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ports.SearchResult{}, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr := new(strings.Builder)
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return ports.SearchResult{}, fmt.Errorf("start yt-dlp: %w", err)
	}

	var tracks []domain.Media
	scanner := bufio.NewScanner(stdout)
	// Increase buffer for long yt-dlp JSON lines (default 64KB is too small)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		track, err := ParseLine(line)
		if err != nil {
			continue // skip unparseable lines
		}
		tracks = append(tracks, track)
	}

	// Check for scanner errors (e.g. line exceeds 64KB buffer)
	if scanErr := scanner.Err(); scanErr != nil {
		return ports.SearchResult{}, fmt.Errorf("read yt-dlp output: %w", scanErr)
	}

	if err := cmd.Wait(); err != nil {
		errOutput := stderr.String()
		if len(tracks) == 0 {
			return ports.SearchResult{}, fmt.Errorf("yt-dlp failed: %w\n%s", err, errOutput)
		}
		// Partial results: yt-dlp failed mid-playlist but we have some tracks
		return ports.SearchResult{
			Tracks: tracks,
			Source: sourceFromURL(url),
		}, fmt.Errorf("yt-dlp finished with errors: %w\n%s", err, errOutput)
	}

	if len(tracks) == 0 {
		return ports.SearchResult{}, fmt.Errorf("no tracks found at URL")
	}

	return ports.SearchResult{Tracks: tracks, Source: sourceFromURL(url)}, nil
}

func sourceFromURL(url string) string {
	if strings.Contains(url, "music.youtube.com") {
		return "youtube-music"
	}
	return "youtube"
}
