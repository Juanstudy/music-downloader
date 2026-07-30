package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// testTrackResponse is the JSON shape for a Spotify track in tests.
type testTrackResponse struct {
	Name       string       `json:"name"`
	Artists    []testArtist `json:"artists"`
	DurationMS int          `json:"duration_ms"`
}

type testArtist struct {
	Name string `json:"name"`
}

// spyHandler records request information for assertions.
type spyHandler struct {
	mu          sync.Mutex
	tokenCalls  int
	trackCalls  int
	tokenStatus int
	trackStatus int
	tokenBody   string
	trackBody   string
	authHeader  string // last Authorization header on track endpoint
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// newTestServer creates a combined token+tracks test server. The token
// endpoint always returns a successful response unless tokenStatus is
// non-zero.
func newTestServer(t *testing.T, spy *spyHandler) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/token":
			spy.mu.Lock()
			spy.tokenCalls++
			spy.mu.Unlock()

			if spy.tokenStatus != 0 && spy.tokenStatus != http.StatusOK {
				w.WriteHeader(spy.tokenStatus)
				if spy.tokenBody != "" {
					w.Write([]byte(spy.tokenBody))
				}
				return
			}

			writeTokenResponse(w, http.StatusOK, "BQtest-token", 3600)

		case strings.HasPrefix(r.URL.Path, "/v1/tracks/"):
			spy.mu.Lock()
			spy.trackCalls++
			spy.authHeader = r.Header.Get("Authorization")
			spy.mu.Unlock()

			if spy.trackStatus != 0 && spy.trackStatus != http.StatusOK {
				w.WriteHeader(spy.trackStatus)
				if spy.trackBody != "" {
					w.Write([]byte(spy.trackBody))
				}
				return
			}

			writeJSON(w, testTrackResponse{
				Name:       "Test Track",
				Artists:    []testArtist{{Name: "Test Artist"}},
				DurationMS: 200000,
			})

		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func mustParseURL(t *testing.T, rawURL string) string {
	t.Helper()
	_, id, err := parseSpotifyURL(rawURL)
	if err != nil {
		t.Fatalf("parseSpotifyURL(%q): %v", rawURL, err)
	}
	return id
}

// ---------------------------------------------------------------------------

func TestTrack_Success(t *testing.T) {
	spy := &spyHandler{}
	ts := newTestServer(t, spy)
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:     "cid",
		clientSecret: "csec",
		httpClient:   ts.Client(),
		ytSearcher: &mockSearcher{
			result: ports.SearchResult{
				Tracks: []domain.Media{{
					URL:      "https://youtube.com/watch?v=yt123",
					Duration: 200 * time.Second,
				}},
			},
		},
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	result, err := s.Search(
		context.Background(),
		"https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Source != "spotify" {
		t.Errorf("expected source spotify, got %q", result.Source)
	}
	if len(result.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(result.Tracks))
	}

	track := result.Tracks[0]
	if track.Title != "Test Track" {
		t.Errorf("expected Title 'Test Track', got %q", track.Title)
	}
	if track.Artist != "Test Artist" {
		t.Errorf("expected Artist 'Test Artist', got %q", track.Artist)
	}
	if track.Duration != 200*time.Second { // from YouTube via resolveTrack
		t.Errorf("expected Duration 200s, got %v", track.Duration)
	}
	if track.Source != "spotify" {
		t.Errorf("expected Source spotify, got %q", track.Source)
	}
	if track.Status != domain.StatusPending {
		t.Errorf("expected StatusPending, got %v", track.Status)
	}
	// URL is now the resolved YouTube URL (not empty)
	if track.URL != "https://youtube.com/watch?v=yt123" {
		t.Errorf("expected YouTube URL, got %q", track.URL)
	}
}

func TestTrack_MultipleArtists(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/token" {
			writeTokenResponse(w, http.StatusOK, "BQmulti", 3600)
			return
		}
		writeJSON(w, testTrackResponse{
			Name:       "Duet Song",
			Artists:    []testArtist{{Name: "Alice"}, {Name: "Bob"}},
			DurationMS: 180000,
		})
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:     "cid",
		clientSecret: "csec",
		httpClient:   ts.Client(),
		ytSearcher: &mockSearcher{
			result: ports.SearchResult{
				Tracks: []domain.Media{{URL: "https://youtube.com/watch?v=multi"}},
			},
		},
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	result, err := s.Search(
		context.Background(),
		"https://open.spotify.com/track/abc123",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	track := result.Tracks[0]
	if track.Artist != "Alice, Bob" {
		t.Errorf("expected 'Alice, Bob', got %q", track.Artist)
	}
	// Source remains spotify after resolution
	if track.Source != "spotify" {
		t.Errorf("expected Source spotify, got %q", track.Source)
	}
	// URL resolved from YouTube
	if track.URL != "https://youtube.com/watch?v=multi" {
		t.Errorf("expected YouTube URL, got %q", track.URL)
	}
}

func TestToken_Unauthorized_Retry(t *testing.T) {
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			writeTokenResponse(w, http.StatusOK, "BQretry", 3600)
		default: // tracks
			attempt++
			if attempt == 1 {
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				writeJSON(w, testTrackResponse{
					Name:       "Retried",
					Artists:    []testArtist{{Name: "Artist"}},
					DurationMS: 100000,
				})
			}
		}
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:     "cid",
		clientSecret: "csec",
		httpClient:   ts.Client(),
		ytSearcher: &mockSearcher{
			result: ports.SearchResult{
				Tracks: []domain.Media{{URL: "https://youtube.com/watch?v=retry"}},
			},
		},
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	result, err := s.Search(
		context.Background(),
		"spotify:track:retrytest",
	)
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if result.Tracks[0].Title != "Retried" {
		t.Errorf("expected 'Retried', got %q", result.Tracks[0].Title)
	}
}

func TestToken_Unauthorized_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			writeTokenResponse(w, http.StatusOK, "BQfail", 3600)
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	_, err := s.Search(
		context.Background(),
		"https://open.spotify.com/track/xxx",
	)
	if err == nil {
		t.Fatal("expected error after 401 retry failure")
	}
}

func TestRateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			writeTokenResponse(w, http.StatusOK, "BQrate", 3600)
		default:
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	_, err := s.Search(
		context.Background(),
		"https://open.spotify.com/track/ratelimited",
	)
	if err == nil {
		t.Fatal("expected error for rate limit")
	}

	var de domain.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain.Error, got %T", err)
	}
	if de.Code != domain.ErrorNetwork {
		t.Errorf("expected ErrorNetwork, got %v", de.Code)
	}
}

func TestInvalidURL(t *testing.T) {
	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      http.DefaultClient,
		accountsBaseURL: "https://accounts.spotify.com",
		apiBaseURL:      "https://api.spotify.com",
	}

	_, err := s.Search(context.Background(), "https://youtube.com/watch?v=xxx")
	if err == nil {
		t.Fatal("expected error for non-Spotify URL")
	}

	var de domain.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain.Error, got %T", err)
	}
	if de.Code != domain.ErrorInvalidURL {
		t.Errorf("expected ErrorInvalidURL, got %v", de.Code)
	}
}

func TestMissingCredentials(t *testing.T) {
	_, err := NewSpotifySearcher("", "secret", nil)
	if err == nil {
		t.Fatal("expected error for missing clientID")
	}

	_, err = NewSpotifySearcher("id", "", nil)
	if err == nil {
		t.Fatal("expected error for missing clientSecret")
	}

	_, err = NewSpotifySearcher("", "", nil)
	if err == nil {
		t.Fatal("expected error for missing both")
	}
}

func TestContextCancelled(t *testing.T) {
	// Server that hangs until we cancel
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			// Client cancelled, no response needed
		}
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := s.Search(ctx, "https://open.spotify.com/track/abc123")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	// Should be a network error wrapping context.Canceled
	if !errors.Is(err, context.Canceled) {
		t.Logf("error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Full-flow integration tests (PR 3: YouTube Resolution)
// ---------------------------------------------------------------------------

func TestSearch_FullFlow_Success(t *testing.T) {
	spy := &spyHandler{}
	ts := newTestServer(t, spy)
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:     "cid",
		clientSecret: "csec",
		httpClient:   ts.Client(),
		ytSearcher: &mockSearcher{
			result: ports.SearchResult{
				Tracks: []domain.Media{{
					URL:      "https://youtube.com/watch?v=fullflow",
					Duration: 210 * time.Second,
				}},
			},
		},
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	result, err := s.Search(
		context.Background(),
		"https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Source != "spotify" {
		t.Errorf("expected Source 'spotify', got %q", result.Source)
	}
	if len(result.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(result.Tracks))
	}

	track := result.Tracks[0]
	// Spotify metadata preserved
	if track.Title != "Test Track" {
		t.Errorf("expected Title 'Test Track', got %q", track.Title)
	}
	if track.Artist != "Test Artist" {
		t.Errorf("expected Artist 'Test Artist', got %q", track.Artist)
	}
	// YouTube URL and duration
	if track.URL != "https://youtube.com/watch?v=fullflow" {
		t.Errorf("expected YouTube URL, got %q", track.URL)
	}
	if track.Duration != 210*time.Second {
		t.Errorf("expected Duration 210s, got %v", track.Duration)
	}
	// Source is still spotify
	if track.Source != "spotify" {
		t.Errorf("expected Source 'spotify', got %q", track.Source)
	}
	if track.Status != domain.StatusPending {
		t.Errorf("expected StatusPending, got %v", track.Status)
	}
}

func TestSearch_NoYouTubeMatch(t *testing.T) {
	spy := &spyHandler{}
	ts := newTestServer(t, spy)
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:     "cid",
		clientSecret: "csec",
		httpClient:   ts.Client(),
		ytSearcher: &mockSearcher{
			result: ports.SearchResult{
				Tracks: []domain.Media{},
			},
		},
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	result, err := s.Search(
		context.Background(),
		"https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Track is included even when YouTube resolution fails
	if len(result.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(result.Tracks))
	}

	track := result.Tracks[0]
	if track.Status != domain.StatusFailed {
		t.Errorf("expected StatusFailed, got %v", track.Status)
	}
	if track.Error == "" {
		t.Error("expected error message on track")
	}
	if track.Source != "spotify" {
		t.Errorf("expected Source 'spotify', got %q", track.Source)
	}
	// Original metadata still present
	if track.Title != "Test Track" {
		t.Errorf("expected Title 'Test Track', got %q", track.Title)
	}
	if track.Artist != "Test Artist" {
		t.Errorf("expected Artist 'Test Artist', got %q", track.Artist)
	}
}

func TestSearch_SpotifyAPIDown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			writeTokenResponse(w, http.StatusOK, "BQtoken", 3600)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	_, err := s.Search(
		context.Background(),
		"https://open.spotify.com/track/abc123",
	)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	var de domain.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain.Error, got %T", err)
	}
	if de.Code != domain.ErrorNetwork {
		t.Errorf("expected ErrorNetwork, got %v", de.Code)
	}
}

func TestSearch_InvalidURL(t *testing.T) {
	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      http.DefaultClient,
		accountsBaseURL: "https://accounts.spotify.com",
		apiBaseURL:      "https://api.spotify.com",
	}

	_, err := s.Search(context.Background(), "https://youtube.com/watch?v=xxx")
	if err == nil {
		t.Fatal("expected error for non-Spotify URL")
	}

	var de domain.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain.Error, got %T", err)
	}
	if de.Code != domain.ErrorInvalidURL {
		t.Errorf("expected ErrorInvalidURL, got %v", de.Code)
	}
}
