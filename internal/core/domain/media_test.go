package domain

import (
	"testing"
	"time"
)

// ----- Status enum -----

func TestStatusValues_Sequential(t *testing.T) {
	tests := []struct {
		name   string
		got    Status
		expect Status
	}{
		{"StatusPending", StatusPending, 0},
		{"StatusResolving", StatusResolving, 1},
		{"StatusResolved", StatusResolved, 2},
		{"StatusDownloading", StatusDownloading, 3},
		{"StatusDone", StatusDone, 4},
		{"StatusFailed", StatusFailed, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expect {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.expect)
			}
		})
	}
}

func TestStatusConstants_AreTyped(t *testing.T) {
	var s Status
	_ = s // Status is the type of all constants
}

// ----- ErrorCode enum -----

func TestErrorCodeValues_Sequential(t *testing.T) {
	tests := []struct {
		name   string
		got    ErrorCode
		expect ErrorCode
	}{
		{"ErrorGeneric", ErrorGeneric, 0},
		{"ErrorNetwork", ErrorNetwork, 1},
		{"ErrorInvalidURL", ErrorInvalidURL, 2},
		{"ErrorBinaryNotFound", ErrorBinaryNotFound, 3},
		{"ErrorTrackUnavailable", ErrorTrackUnavailable, 4},
		{"ErrorAgeRestricted", ErrorAgeRestricted, 5},
		{"ErrorDiskFull", ErrorDiskFull, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expect {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.expect)
			}
		})
	}
}

func TestErrorCodeConstants_AreTyped(t *testing.T) {
	var ec ErrorCode
	_ = ec // ErrorCode is the type of all constants
}

// ----- Media struct -----

func TestMedia_ZeroValue(t *testing.T) {
	var m Media

	if m.URL != "" {
		t.Errorf("Media.URL zero value = %q, want empty", m.URL)
	}
	if m.Title != "" {
		t.Errorf("Media.Title zero value = %q, want empty", m.Title)
	}
	if m.Artist != "" {
		t.Errorf("Media.Artist zero value = %q, want empty", m.Artist)
	}
	if m.Duration != 0 {
		t.Errorf("Media.Duration zero value = %v, want 0", m.Duration)
	}
	if m.Source != "" {
		t.Errorf("Media.Source zero value = %q, want empty", m.Source)
	}
	if m.Status != StatusPending {
		t.Errorf("Media.Status zero value = %v, want StatusPending", m.Status)
	}
	if m.Error != "" {
		t.Errorf("Media.Error zero value = %q, want empty", m.Error)
	}
	if m.OutputPath != "" {
		t.Errorf("Media.OutputPath zero value = %q, want empty", m.OutputPath)
	}
}

func TestMedia_StructLiteral(t *testing.T) {
	m := Media{
		URL:        "https://youtube.com/watch?v=test",
		Title:      "Test Song",
		Artist:     "Test Artist",
		Duration:   3*time.Minute + 30*time.Second,
		Source:     "youtube",
		Status:     StatusResolved,
		Error:      "",
		OutputPath: "/tmp/test.mp3",
	}

	if m.URL != "https://youtube.com/watch?v=test" {
		t.Errorf("Media.URL = %q, want %q", m.URL, "https://youtube.com/watch?v=test")
	}
	if m.Title != "Test Song" {
		t.Errorf("Media.Title = %q, want %q", m.Title, "Test Song")
	}
	if m.Artist != "Test Artist" {
		t.Errorf("Media.Artist = %q, want %q", m.Artist, "Test Artist")
	}
	if m.Duration != 3*time.Minute+30*time.Second {
		t.Errorf("Media.Duration = %v, want %v", m.Duration, 3*time.Minute+30*time.Second)
	}
	if m.Source != "youtube" {
		t.Errorf("Media.Source = %q, want %q", m.Source, "youtube")
	}
	if m.Status != StatusResolved {
		t.Errorf("Media.Status = %v, want StatusResolved", m.Status)
	}
	if m.Error != "" {
		t.Errorf("Media.Error = %q, want empty", m.Error)
	}
	if m.OutputPath != "/tmp/test.mp3" {
		t.Errorf("Media.OutputPath = %q, want %q", m.OutputPath, "/tmp/test.mp3")
	}
}

// ----- domain.Error -----

func TestError_ImplementsErrorInterface(t *testing.T) {
	err := Error{
		Code:    ErrorNetwork,
		Message: "network timeout",
		Track:   "https://youtube.com/watch?v=test",
	}

	var e error = err
	if e.Error() != "network timeout" {
		t.Errorf("domain.Error.Error() = %q, want %q", e.Error(), "network timeout")
	}
}

func TestError_WithTrackSet(t *testing.T) {
	err := Error{
		Code:    ErrorBinaryNotFound,
		Message: "yt-dlp not found in PATH",
		Track:   "https://youtube.com/watch?v=specific-track",
	}

	if err.Code != ErrorBinaryNotFound {
		t.Errorf("Error.Code = %v, want ErrorBinaryNotFound", err.Code)
	}
	if err.Track != "https://youtube.com/watch?v=specific-track" {
		t.Errorf("Error.Track = %q, want %q", err.Track, "https://youtube.com/watch?v=specific-track")
	}
	if err.Error() != "yt-dlp not found in PATH" {
		t.Errorf("Error.Error() = %q, want %q", err.Error(), "yt-dlp not found in PATH")
	}
}
