package spotify

import (
	"strings"
	"testing"
)

func TestParseSpotifyURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantEntity string
		wantID     string
		wantErr    bool
		wantMsg    string // substring to check in error message
	}{
		{
			name:       "valid track URL",
			url:        "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			wantEntity: "track",
			wantID:     "4iV5W9uYEdYUVa79Axb7Rh",
			wantErr:    false,
		},
		{
			name:       "valid track URI",
			url:        "spotify:track:4iV5W9uYEdYUVa79Axb7Rh",
			wantEntity: "track",
			wantID:     "4iV5W9uYEdYUVa79Axb7Rh",
			wantErr:    false,
		},
		{
			name:    "invalid Spotify URL (no path)",
			url:     "https://open.spotify.com/",
			wantErr: true,
		},
		{
			name:    "playlist URL — unsupported",
			url:     "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M",
			wantErr: true,
			wantMsg: "only track URLs are supported",
		},
		{
			name:    "album URL — unsupported",
			url:     "https://open.spotify.com/album/1kfVbJpH1WPqOjLwfoCmXr",
			wantErr: true,
			wantMsg: "only track URLs are supported",
		},
		{
			name:    "artist URL — unsupported",
			url:     "https://open.spotify.com/artist/1kfVbJpH1WPqOjLwfoCmXr",
			wantErr: true,
			wantMsg: "only track URLs are supported",
		},
		{
			name:    "non-Spotify URL",
			url:     "https://youtube.com/watch?v=xxx",
			wantErr: true,
			wantMsg: "not a Spotify URL",
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: true,
		},
		{
			name:    "track URI with no ID",
			url:     "spotify:track:",
			wantErr: true,
		},
		{
			name:       "short alphanumeric track ID",
			url:        "https://open.spotify.com/track/abc123",
			wantEntity: "track",
			wantID:     "abc123",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity, id, err := parseSpotifyURL(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
					t.Errorf("expected error containing %q, got %q", tt.wantMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if entity != tt.wantEntity {
				t.Errorf("expected entity %q, got %q", tt.wantEntity, entity)
			}
			if id != tt.wantID {
				t.Errorf("expected id %q, got %q", tt.wantID, id)
			}
		})
	}
}
