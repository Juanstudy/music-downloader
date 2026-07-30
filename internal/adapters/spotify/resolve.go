package spotify

import (
	"context"
	"fmt"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// resolveTrack takes a Spotify track with metadata and resolves it to a YouTube
// URL using the provided Searcher (yt-dlp). It preserves the Spotify metadata
// (Title, Artist, Source) and adds the YouTube URL and duration.
//
// If no YouTube results are found, the track is returned with StatusFailed and
// a descriptive error message. If the searcher itself fails, the error is
// propagated.
func resolveTrack(ctx context.Context, track domain.Media, ytSearcher ports.Searcher) (domain.Media, error) {
	query := track.Artist + " - " + track.Title

	result, err := ytSearcher.Search(ctx, "ytsearch:"+query)
	if err != nil {
		return domain.Media{}, err
	}

	if len(result.Tracks) == 0 {
		track.Status = domain.StatusFailed
		track.Error = fmt.Sprintf("no YouTube results found for: %s", query)
		return track, nil
	}

	ytTrack := result.Tracks[0]
	resolved := track
	resolved.URL = ytTrack.URL
	resolved.Duration = ytTrack.Duration
	resolved.Source = "spotify"

	return resolved, nil
}
