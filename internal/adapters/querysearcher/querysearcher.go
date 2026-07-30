package querysearcher

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/Juanstudy/music-downloader/internal/adapters/searcher"
	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// QuerySearcher searches YouTube Music via free-text queries using yt-dlp's
// youtube:music:search_url extractor (triggered by a YouTube Music search URL).
// It follows the same patterns as the existing adapters/searcher.Searcher.
type QuerySearcher struct {
	binary string
}

// NewQuerySearcher creates a QuerySearcher that uses the system yt-dlp binary.
func NewQuerySearcher() *QuerySearcher {
	return &QuerySearcher{binary: "yt-dlp"}
}

// SearchByQuery runs a YouTube Music search URL through yt-dlp and returns
// parsed tracks. The source is always "youtube-music".
//
// Uses https://music.youtube.com/search?q=<query> with --playlist-end to
// control result count, which triggers yt-dlp's youtube:music:search_url
// extractor. This is more portable than the ytmusicsearch: prefix (which
// requires a specific yt-dlp build).
//
// An empty or whitespace-only query returns a domain.Error with
// ErrorInvalidURL code. A limit <= 0 defaults to 10.
func (s *QuerySearcher) SearchByQuery(ctx context.Context, query string, limit int) (ports.SearchResult, error) {
	if trimmed := strings.TrimSpace(query); trimmed == "" {
		return ports.SearchResult{}, domain.Error{
			Code:    domain.ErrorInvalidURL,
			Message: "query must not be empty",
		}
	}

	if limit <= 0 {
		limit = 10
	}

	searchURL := fmt.Sprintf("https://music.youtube.com/search?q=%s", url.QueryEscape(query))
	args := []string{
		"--flat-playlist",
		"--dump-json",
		"--ignore-errors",
		"--no-warnings",
		"--playlist-end", fmt.Sprintf("%d", limit),
		searchURL,
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
		track, parseErr := searcher.ParseLine(line)
		if parseErr != nil {
			continue // skip unparseable lines
		}
		tracks = append(tracks, track)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return ports.SearchResult{}, fmt.Errorf("read yt-dlp output: %w", scanErr)
	}

	if err := cmd.Wait(); err != nil {
		errOutput := stderr.String()
		if len(tracks) == 0 {
			return ports.SearchResult{}, fmt.Errorf("yt-dlp failed: %w\n%s", err, errOutput)
		}
		return ports.SearchResult{
			Tracks: tracks,
			Source: "youtube-music",
		}, fmt.Errorf("yt-dlp finished with errors: %w\n%s", err, errOutput)
	}

	return ports.SearchResult{Tracks: tracks, Source: "youtube-music"}, nil
}
