// Package spotify implements a Searcher that resolves Spotify track URLs by
// fetching metadata from the Spotify Web API and resolving each track to a
// YouTube URL via yt-dlp ytsearch.
package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// Package-level endpoint defaults; overridable for testing.
var (
	accountsBaseURL = "https://accounts.spotify.com"
	apiBaseURL      = "https://api.spotify.com"
)

// errSpotifyUnauthorized is returned by fetchTrack when the Spotify API
// responds with 401. The Search method catches it and retries with a fresh
// token.
var errSpotifyUnauthorized = errors.New("spotify: unauthorized")

// SpotifySearcher implements ports.Searcher for Spotify tracks.
type SpotifySearcher struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	ytSearcher   ports.Searcher

	tokenMu sync.Mutex
	token   *oauth2Token

	accountsBaseURL string
	apiBaseURL      string
}

// NewSpotifySearcher validates credentials and returns a ready-to-use
// SpotifySearcher.
func NewSpotifySearcher(clientID, clientSecret string, ytSearcher ports.Searcher) (*SpotifySearcher, error) {
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("spotify: clientID and clientSecret are required")
	}

	return &SpotifySearcher{
		clientID:        clientID,
		clientSecret:    clientSecret,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		ytSearcher:      ytSearcher,
		accountsBaseURL: accountsBaseURL,
		apiBaseURL:      apiBaseURL,
	}, nil
}

// Search resolves a Spotify track URL into a SearchResult containing one
// Media entry with metadata fetched from the Spotify Web API.
func (s *SpotifySearcher) Search(ctx context.Context, url string) (ports.SearchResult, error) {
	_, id, err := parseSpotifyURL(url)
	if err != nil {
		return ports.SearchResult{}, domain.Error{
			Code:    domain.ErrorInvalidURL,
			Message: err.Error(),
		}
	}

	token, err := s.getToken(ctx)
	if err != nil {
		return ports.SearchResult{}, err
	}

	track, err := s.fetchTrack(ctx, token, id)
	if err != nil {
		if errors.Is(err, errSpotifyUnauthorized) {
			// Token may have expired — refresh once and retry.
			token, err = s.refreshToken(ctx)
			if err != nil {
				return ports.SearchResult{}, err
			}

			track, err = s.fetchTrack(ctx, token, id)
			if err != nil {
				return ports.SearchResult{}, err
			}
		} else {
			return ports.SearchResult{}, err
		}
	}

	// Resolve track metadata to a playable YouTube URL.
	resolved, err := resolveTrack(ctx, track, s.ytSearcher)
	if err != nil {
		return ports.SearchResult{}, err
	}

	return ports.SearchResult{
		Tracks: []domain.Media{resolved},
		Source: "spotify",
	}, nil
}

// spotifyTrackResponse is the JSON shape of a single track from the Spotify
// Web API.
type spotifyTrackResponse struct {
	Name    string `json:"name"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	DurationMS int `json:"duration_ms"`
}

// fetchTrack calls the Spotify /v1/tracks/{id} endpoint and returns a
// domain.Media with the metadata.
func (s *SpotifySearcher) fetchTrack(ctx context.Context, token, trackID string) (domain.Media, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		s.apiBaseURL+"/v1/tracks/"+trackID,
		http.NoBody,
	)
	if err != nil {
		return domain.Media{}, fmt.Errorf(
			"%w: building track request: %w",
			domain.Error{Code: domain.ErrorNetwork, Message: "spotify tracks"},
			err,
		)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return domain.Media{}, fmt.Errorf(
			"%w: track request failed: %w",
			domain.Error{Code: domain.ErrorNetwork, Message: "spotify tracks"},
			err,
		)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// decode below
	case http.StatusUnauthorized:
		return domain.Media{}, errSpotifyUnauthorized
	case http.StatusTooManyRequests:
		return domain.Media{}, domain.Error{
			Code:    domain.ErrorNetwork,
			Message: "spotify: rate limited",
		}
	default:
		body, _ := io.ReadAll(resp.Body)
		return domain.Media{}, domain.Error{
			Code:    domain.ErrorNetwork,
			Message: fmt.Sprintf("spotify: unexpected HTTP %d: %s", resp.StatusCode, string(body)),
		}
	}

	var tr spotifyTrackResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return domain.Media{}, fmt.Errorf(
			"%w: decoding track response: %w",
			domain.Error{Code: domain.ErrorNetwork, Message: "spotify tracks"},
			err,
		)
	}

	// Join multiple artist names with ", ".
	artists := make([]string, len(tr.Artists))
	for i, a := range tr.Artists {
		artists[i] = a.Name
	}

	return domain.Media{
		Title:    tr.Name,
		Artist:   strings.Join(artists, ", "),
		Duration: time.Duration(tr.DurationMS) * time.Millisecond,
		Source:   "spotify",
		Status:   domain.StatusPending,
	}, nil
}
