package spotify

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var spotifyIDPattern = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// parseSpotifyURL extracts the entity type and ID from a Spotify URL.
// It supports:
//   - https://open.spotify.com/track/{id}
//   - spotify:track:{id}
//
// Playlist, album, and artist URLs return an error indicating that only track
// URLs are supported in this version.
func parseSpotifyURL(rawURL string) (entity string, id string, err error) {
	// Normalize: trim whitespace
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", errors.New("empty URL")
	}

	// Try Spotify URI format first: spotify:track:{id}
	if strings.HasPrefix(rawURL, "spotify:") {
		parts := strings.SplitN(rawURL, ":", 3)
		if len(parts) == 3 && parts[0] == "spotify" {
			entity = parts[1]
			id = parts[2]
			return validateTrack(entity, id)
		}
		return "", "", errors.New("invalid Spotify URI format")
	}

	// Try HTTP URL format
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", errors.New("invalid URL format")
	}

	// Must be a Spotify domain
	if !strings.HasSuffix(parsed.Host, "open.spotify.com") && !strings.HasSuffix(parsed.Host, "spotify.com") {
		return "", "", errors.New("not a Spotify URL")
	}

	// Path should be like /track/{id}, /album/{id}, /playlist/{id}, /artist/{id}
	path := strings.Trim(parsed.Path, "/")
	segments := strings.SplitN(path, "/", 2)
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return "", "", errors.New("invalid Spotify URL format")
	}

	entity = segments[0]
	id = segments[1]
	return validateTrack(entity, id)
}

// validateTrack checks that the entity is a track and the ID is non-empty
// alphanumeric. Returns an error for unsupported entities (album, playlist,
// artist) or invalid IDs.
func validateTrack(entity, id string) (string, string, error) {
	if entity != "track" {
		return "", "", errors.New("only track URLs are supported in this version")
	}
	if id == "" || !spotifyIDPattern.MatchString(id) {
		return "", "", errors.New("invalid track ID")
	}
	return entity, id, nil
}
