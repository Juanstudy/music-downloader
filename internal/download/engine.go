// Package download abstracts the download engine (yt-dlp).
package download

import (
	"github.com/Juanstudy/music-downloader/internal/model"
)

// Engine defines the interface for download backends.
// Keeps the TUI decoupled from yt-dlp implementation details.
type Engine interface {
	// CheckInstalled verifies the engine binary is available on $PATH.
	CheckInstalled() error

	// Resolve fetches metadata for a URL. For a single track it returns
	// one Media, for a playlist it returns all items.
	Resolve(url string) ([]*model.Media, error)

	// Download starts downloading one track. Returns when complete.
	// The progress channel receives status updates (for future use).
	Download(track *model.Media, outputDir string, progress chan<- string) error
}
