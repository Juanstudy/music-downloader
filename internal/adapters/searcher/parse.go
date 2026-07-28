package searcher

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
)

type ytDlpTrack struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
	Channel  string  `json:"channel"`
	Uploader string  `json:"uploader"`
	Creator  string  `json:"creator"`
	Webpage  string  `json:"webpage_url"`
}

// ParseLine parses one line of yt-dlp --dump-json output into domain.Media.
// Returns an error for invalid JSON, missing title, or missing webpage_url.
func ParseLine(line string) (domain.Media, error) {
	var raw ytDlpTrack
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return domain.Media{}, fmt.Errorf("parse yt-dlp output: %w", err)
	}
	if raw.Title == "" {
		return domain.Media{}, fmt.Errorf("missing title in yt-dlp output")
	}
	if raw.Webpage == "" {
		return domain.Media{}, fmt.Errorf("missing webpage_url in yt-dlp output")
	}

	artist := raw.Channel
	if artist == "" {
		artist = raw.Uploader
	}
	if artist == "" {
		artist = raw.Creator
	}

	return domain.Media{
		URL:      raw.Webpage,
		Title:    raw.Title,
		Artist:   artist,
		Duration: time.Duration(raw.Duration * float64(time.Second)),
		Source:   "youtube",
		Status:   domain.StatusPending,
	}, nil
}
