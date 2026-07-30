package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
)

// tokenHandler is a helper that writes a JSON token response.
func writeTokenResponse(w http.ResponseWriter, status int, token string, expiresIn int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if token != "" && status == http.StatusOK {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   expiresIn,
		})
	}
}

func TestGetToken_FirstCall(t *testing.T) {
	var callCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/token" {
			t.Errorf("expected /api/token, got %s", r.URL.Path)
		}
		// Verify Basic auth header
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("wrong Content-Type: %s", r.Header.Get("Content-Type"))
		}
		writeTokenResponse(w, http.StatusOK, "BQfirst", 3600)
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	token, err := s.getToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "BQfirst" {
		t.Errorf("expected BQfirst, got %s", token)
	}
	if callCount != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount)
	}
}

func TestGetToken_Cached(t *testing.T) {
	var callCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		writeTokenResponse(w, http.StatusOK, "BQcached", 3600)
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
		token: &oauth2Token{
			AccessToken: "BQcached",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
			ExpiresAt:   time.Now().Add(5 * time.Minute),
		},
	}

	// First call should use cache.
	token, err := s.getToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "BQcached" {
		t.Errorf("expected BQcached, got %s", token)
	}
	if callCount != 0 {
		t.Errorf("expected 0 HTTP calls (cached), got %d", callCount)
	}
}

func TestGetToken_Expired(t *testing.T) {
	var callCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		writeTokenResponse(w, http.StatusOK, "BQfresh", 3600)
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
		token: &oauth2Token{
			AccessToken: "BQstale",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
			ExpiresAt:   time.Now().Add(-1 * time.Minute), // expired
		},
	}

	token, err := s.getToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "BQfresh" {
		t.Errorf("expected BQfresh, got %s", token)
	}
	if callCount != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount)
	}
}

func TestGetToken_InvalidCredentials(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "bad",
		clientSecret:    "bad",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	_, err := s.getToken(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid credentials")
	}
	if !contains(err.Error(), "HTTP 401") {
		t.Errorf("expected error mentioning HTTP 401, got: %v", err)
	}
}

func TestGetToken_RateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	_, err := s.getToken(context.Background())
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

func TestGetToken_ConcurrentSafe(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		writeTokenResponse(w, http.StatusOK, "BQsafe", 3600)
	}))
	defer ts.Close()

	s := &SpotifySearcher{
		clientID:        "cid",
		clientSecret:    "csec",
		httpClient:      ts.Client(),
		accountsBaseURL: ts.URL,
		apiBaseURL:      ts.URL,
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := s.getToken(context.Background())
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if token != "BQsafe" {
				t.Errorf("expected BQsafe, got %s", token)
			}
		}()
	}
	wg.Wait()

	// At least one call must have been made (the first goroutine to reach the
	// HTTP endpoint). In practice callCount is 1 because after the first fetch
	// the token is cached for all subsequent goroutines.
	mu.Lock()
	count := callCount
	mu.Unlock()
	if count == 0 {
		t.Error("expected at least 1 HTTP call")
	}
}

// contains reports whether substr is within s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
