package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// Orchestrator composes a Searcher and Downloader into a high-level
// music resolution and download workflow.
type Orchestrator struct {
	searcher   ports.Searcher
	downloader ports.Downloader
}

// NewOrchestrator creates a new Orchestrator with the given dependencies.
func NewOrchestrator(s ports.Searcher, d ports.Downloader) *Orchestrator {
	return &Orchestrator{
		searcher:   s,
		downloader: d,
	}
}

// qualitySetter is the optional capability the downloader may expose. Kept
// local so core does not import the adapter package and the port stays frozen.
type qualitySetter interface {
	SetAudioBitrate(string)
}

// SetAudioQuality forwards the audio quality to the downloader so subsequent
// downloads use it. No-op when the injected downloader has no setter.
func (o *Orchestrator) SetAudioQuality(q string) {
	if s, ok := o.downloader.(qualitySetter); ok {
		s.SetAudioBitrate(q)
	}
}

// ResolveTrack validates the URL, searches for tracks, and marks them as resolved.
// If the search returns partial results with an error (e.g. yt-dlp failed mid-playlist),
// the tracks are still returned along with the error so the user can use partial results.
func (o *Orchestrator) ResolveTrack(ctx context.Context, url string) ([]domain.Media, error) {
	if strings.TrimSpace(url) == "" {
		return nil, domain.Error{
			Code:    domain.ErrorInvalidURL,
			Message: "URL cannot be empty",
		}
	}

	result, err := o.searcher.Search(ctx, url)
	if err != nil && len(result.Tracks) == 0 {
		return nil, err
	}

	for i := range result.Tracks {
		result.Tracks[i].Status = domain.StatusResolved
	}

	return result.Tracks, err // may return partial tracks + warning
}

// DownloadTrack downloads a single track and returns the updated Media.
func (o *Orchestrator) DownloadTrack(ctx context.Context, media domain.Media, outputDir string) (domain.Media, error) {
	// Check context cancellation before delegating.
	if ctx.Err() != nil {
		media.Status = domain.StatusFailed
		media.Error = ctx.Err().Error()
		return media, fmt.Errorf("download cancelled: %w", ctx.Err())
	}

	result, err := o.downloader.Download(ctx, media, outputDir)
	if err != nil {
		media.Status = domain.StatusFailed
		media.Error = err.Error()
		return media, fmt.Errorf("download failed: %w", err)
	}

	media.Status = domain.StatusDone
	media.OutputPath = result.OutputPath
	return media, nil
}
