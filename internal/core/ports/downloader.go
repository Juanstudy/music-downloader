package ports

import (
	"context"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
)

// DownloadResult holds the result of a single track download.
type DownloadResult struct {
	Media      domain.Media
	OutputPath string
}

// Downloader downloads a single track to the specified output directory.
type Downloader interface {
	Download(ctx context.Context, media domain.Media, outputDir string) (DownloadResult, error)
}
