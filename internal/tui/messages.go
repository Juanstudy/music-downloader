package tui

import "github.com/Juanstudy/music-downloader/internal/core/domain"

// resolveFinishedMsg is sent when the URL resolution goroutine completes.
type resolveFinishedMsg struct {
	tracks []domain.Media
	err    error
}

// trackDownloadedMsg is sent after each individual track download completes.
type trackDownloadedMsg struct {
	index int
	media domain.Media
	err   error
}
