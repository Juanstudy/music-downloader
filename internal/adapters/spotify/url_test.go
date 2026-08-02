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

func TestIsSpotifyURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		// HTTP Spotify hosts — all entity paths are host-level true (ARF-003).
		{name: "open.spotify.com track URL", url: "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh", want: true},
		{name: "open.spotify.com playlist URL", url: "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M", want: true},
		{name: "open.spotify.com album URL", url: "https://open.spotify.com/album/1kfVbJpH1WPqOjLwfoCmXr", want: true},
		{name: "open.spotify.com artist URL", url: "https://open.spotify.com/artist/1kfVbJpH1WPqOjLwfoCmXr", want: true},
		{name: "www.spotify.com URL", url: "https://www.spotify.com/...", want: true},
		{name: "uppercase host", url: "https://OPEN.SPOTIFY.COM/track/4iV5W9uYEdYUVa79Axb7Rh", want: true},
		// Exact-host branch: host == "spotify.com" (design §2.1 decision table).
		{name: "exact spotify.com host", url: "https://spotify.com/track/x", want: true},
		// spotify: URIs — any entity form, prefix-based (design §2.1).
		{name: "spotify track URI", url: "spotify:track:4iV5W9uYEdYUVa79Axb7Rh", want: true},
		{name: "spotify playlist URI", url: "spotify:playlist:37i9dQZF1DXcBWIGoYBM5M", want: true},
		{name: "spotify album URI", url: "spotify:album:1kfVbJpH1WPqOjLwfoCmXr", want: true},
		{name: "spotify artist URI", url: "spotify:artist:1kfVbJpH1WPqOjLwfoCmXr", want: true},
		// Whitespace-tolerant URI admission (design §7.4 — TrimSpace inside the helper).
		{name: "spotify URI with surrounding whitespace", url: "  spotify:track:4iV5W9uYEdYUVa79Axb7Rh  ", want: true},
		// Non-Spotify and lookalike hosts — must be false (ARF-003, design §6.4).
		{name: "music.youtube.com URL", url: "https://music.youtube.com/watch?v=xxx", want: false},
		{name: "youtube.com URL", url: "https://youtube.com/watch?v=xxx", want: false},
		{name: "evilspotify.com lookalike", url: "https://evilspotify.com/track/x", want: false},
		{name: "spotify.com.evil.example lookalike", url: "https://spotify.com.evil.example/track/x", want: false},
		// Empty and whitespace-only input — must be false (ARF-003).
		{name: "empty string", url: "", want: false},
		{name: "whitespace only", url: "   ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSpotifyURL(tt.url); got != tt.want {
				t.Errorf("IsSpotifyURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
