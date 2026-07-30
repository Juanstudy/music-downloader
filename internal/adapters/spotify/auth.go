package spotify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
)

// oauth2Token holds a cached Spotify Client Credentials token.
type oauth2Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	ExpiresAt   time.Time
}

// getToken returns a valid access token, using a cached one if it is still
// valid (with a 60‑second safety margin before expiry).
func (s *SpotifySearcher) getToken(ctx context.Context) (string, error) {
	s.tokenMu.Lock()
	if s.token != nil && time.Now().Before(s.token.ExpiresAt) {
		t := s.token.AccessToken
		s.tokenMu.Unlock()
		return t, nil
	}
	s.tokenMu.Unlock()

	tok, err := s.fetchToken(ctx)
	if err != nil {
		return "", err
	}

	s.tokenMu.Lock()
	s.token = tok
	s.tokenMu.Unlock()

	return tok.AccessToken, nil
}

// refreshToken always fetches a fresh token from Spotify, ignoring the cache.
// It is used when a 401 is received from the tracks API.
func (s *SpotifySearcher) refreshToken(ctx context.Context) (string, error) {
	tok, err := s.fetchToken(ctx)
	if err != nil {
		return "", err
	}

	s.tokenMu.Lock()
	s.token = tok
	s.tokenMu.Unlock()

	return tok.AccessToken, nil
}

// fetchToken performs the actual Client Credentials grant request.
func (s *SpotifySearcher) fetchToken(ctx context.Context) (*oauth2Token, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.accountsBaseURL+"/api/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: building token request: %w",
			domain.Error{Code: domain.ErrorNetwork, Message: "spotify auth"},
			err,
		)
	}

	encoded := base64.StdEncoding.EncodeToString(
		[]byte(s.clientID + ":" + s.clientSecret),
	)
	req.Header.Set("Authorization", "Basic "+encoded)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: token request failed: %w",
			domain.Error{Code: domain.ErrorNetwork, Message: "spotify auth"},
			err,
		)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: reading token response: %w",
			domain.Error{Code: domain.ErrorNetwork, Message: "spotify auth"},
			err,
		)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// parse below
	case http.StatusTooManyRequests:
		return nil, domain.Error{
			Code:    domain.ErrorNetwork,
			Message: "spotify auth: rate limited",
		}
	case http.StatusBadRequest, http.StatusUnauthorized:
		return nil, fmt.Errorf("spotify auth failed (HTTP %d): %s", resp.StatusCode, string(raw))
	case http.StatusForbidden:
		return nil, domain.Error{
			Code:    domain.ErrorTrackUnavailable,
			Message: "Spotify: Premium subscription required (HTTP 403). See README for setup instructions.",
		}
	default:
		return nil, domain.Error{
			Code:    domain.ErrorNetwork,
			Message: fmt.Sprintf("spotify auth: unexpected HTTP %d", resp.StatusCode),
		}
	}

	var tok oauth2Token
	if err := json.Unmarshal(raw, &tok); err != nil {
		return nil, fmt.Errorf(
			"%w: decoding token response: %w",
			domain.Error{Code: domain.ErrorNetwork, Message: "spotify auth"},
			err,
		)
	}

	// Cache with a 60‑second buffer so we refresh before the real expiry.
	tok.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second - 60*time.Second)
	return &tok, nil
}
