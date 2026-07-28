package domain

import "time"

// Status represents the lifecycle state of a media item or download operation.
type Status int

const (
	StatusPending Status = iota
	StatusResolving
	StatusResolved
	StatusDownloading
	StatusDone
	StatusFailed
)

// Media represents a downloadable track.
type Media struct {
	URL        string
	Title      string
	Artist     string
	Duration   time.Duration
	Source     string
	Status     Status
	Error      string
	OutputPath string
}

// ErrorCode represents a category of domain error.
type ErrorCode int

const (
	ErrorGeneric ErrorCode = iota
	ErrorNetwork
	ErrorInvalidURL
	ErrorBinaryNotFound
	ErrorTrackUnavailable
	ErrorAgeRestricted
	ErrorDiskFull
)

// Error is a structured domain error that implements the error interface.
type Error struct {
	Code    ErrorCode
	Message string
	Track   string
}

func (e Error) Error() string { return e.Message }
