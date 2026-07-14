// Package model defines the core domain types for the music-downloader.
package model

// Status represents the current state of a track in the download queue.
type Status int

const (
	StatusPending Status = iota
	StatusDownloading
	StatusCompleted
	StatusFailed
)

// Media represents a downloadable music track.
type Media struct {
	URL      string
	Title    string
	Artist   string
	Duration string // e.g. "5:55"
	Source   string // e.g. "youtube", "ytmusic"
	Status   Status
	ErrorMsg string // populated only when StatusFailed
}
