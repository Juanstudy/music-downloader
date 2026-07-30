# Adapters — Spotify Searcher Specification

## Purpose

Implement a `Searcher` port that resolves Spotify track URLs by fetching metadata from the Spotify Web API (Client Credentials flow), then resolves the track to a playable YouTube URL via `yt-dlp ytsearch`.

## Requirements

### Requirement: SpotifySearcher implements ports.Searcher

The system MUST provide a `SpotifySearcher` that satisfies the `ports.Searcher` interface.

```go
type SpotifySearcher struct {
    clientID     string
    clientSecret string
    httpClient   *http.Client // plain HTTP, no external OAuth library
}

func NewSpotifySearcher(clientID, clientSecret string) *SpotifySearcher
func (s *SpotifySearcher) Search(ctx context.Context, url string) (ports.SearchResult, error)
```

#### Scenario: NewSpotifySearcher creates a valid instance

- GIVEN valid `clientID` and `clientSecret` strings
- WHEN `NewSpotifySearcher(clientID, clientSecret)` is called
- THEN it MUST return a non-nil `*SpotifySearcher`
- AND the returned instance MUST have the credentials stored internally
- AND `httpClient` MUST be a non-nil `*http.Client`

#### Scenario: Search returns tracks with source "spotify"

- GIVEN a `SpotifySearcher` instance with valid credentials
- WHEN `Search(ctx, "https://open.spotify.com/track/123ABC")` is called
- THEN every returned `Media` MUST have `Source` set to `"spotify"`
- AND every returned `Media` MUST have `Status` set to `domain.StatusPending`

#### Scenario: Search returns exactly one Media for a track URL

- GIVEN a `SpotifySearcher` instance
- WHEN the URL is a single Spotify track
- THEN `SearchResult.Tracks` MUST have length exactly 1
- AND `Media.URL` MUST be the resolved YouTube URL (via ytsearch)
- AND `Media.Title` MUST be the Spotify track name
- AND `Media.Artist` MUST be the primary artist name from Spotify

---

### Requirement: SpotifySearcher authenticates via Client Credentials flow

The system MUST obtain an OAuth access token from the Spotify Accounts service using the Client Credentials flow before making API requests.

#### Scenario: Token is obtained with valid credentials

- GIVEN a `SpotifySearcher` with valid `clientID` and `clientSecret`
- WHEN `Search` is called
- THEN the system MUST POST to `https://accounts.spotify.com/api/token`
- AND the POST body MUST be `grant_type=client_credentials`
- AND the Authorization header MUST be `Basic <base64(clientID:clientSecret)>`
- AND the response MUST be parsed for `access_token` and `token_type`
- AND the token MUST be reused across subsequent Search calls until expiry

#### Scenario: Token is refreshed on 401 response

- GIVEN a `SpotifySearcher` with a cached but expired token
- WHEN `Search` is called and the API responds with HTTP 401
- THEN the system MUST discard the expired token
- AND MUST obtain a fresh token via Client Credentials flow
- AND MUST retry the failed API request once with the new token
- AND if the retry also fails with 401, MUST return a `domain.Error` with code `ErrorGeneric` and message indicating auth failure

#### Scenario: Token request fails with invalid credentials

- GIVEN a `SpotifySearcher` with invalid `clientID` or `clientSecret`
- WHEN `Search` is called
- THEN the token request MUST fail
- AND `Search` MUST return a `domain.Error` with a message describing the authentication failure

---

### Requirement: SpotifySearcher fetches track metadata via Spotify Web API

The system MUST call the Spotify Web API track endpoint to retrieve track metadata.

#### Scenario: Track metadata is fetched successfully

- GIVEN a valid access token and a Spotify track ID extracted from the URL
- WHEN resolving a track URL (e.g., `https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh`)
- THEN the system MUST send a GET request to `https://api.spotify.com/v1/tracks/{id}`
- AND the Authorization header MUST be `Bearer {access_token}`
- AND the response MUST be parsed for: `name`, `artists[0].name`, `album.name`, `duration_ms`, `external_ids.isrc`

#### Scenario: Track with multiple artists concatenates them

- GIVEN a Spotify track with multiple artists
- WHEN the track metadata is fetched
- THEN `Media.Artist` MUST concatenate all artists separated by `", "`
- AND the first artist from `artists[0].name` MUST come first

---

### Requirement: SpotifySearcher resolves to YouTube via yt-dlp ytsearch

The system MUST use `yt-dlp --flat-playlist --dump-json ytsearch:{query}` to find the best matching YouTube video for a Spotify track.

#### Scenario: ytsearch query includes track name and artist

- GIVEN a Spotify track with name "Blinding Lights" and artist "The Weeknd"
- WHEN resolving to YouTube
- THEN the system MUST invoke `yt-dlp --flat-playlist --dump-json --ignore-errors ytsearch:"Blinding Lights The Weeknd"`
- AND parse the first result from the JSON output into a `domain.Media`
- AND set `Media.URL` to the YouTube video URL
- AND set `Media.Source` to `"spotify"` (not overwritten by YouTube source)

#### Scenario: ytsearch returns no results

- GIVEN a Spotify track
- WHEN `yt-dlp ytsearch` returns zero results
- THEN `Search` MUST return a `domain.Error` with code `ErrorTrackUnavailable`
- AND the error message MUST include the track name and artist
- AND the track MUST NOT be skipped silently
- AND the error MUST be propagated to the UI for display

#### Scenario: ytsearch ISRC-based fallback

- GIVEN a Spotify track with an ISRC code in its metadata
- THEN the system SHOULD attempt `ytsearch:"isrc:{ISRC}"` first
- AND fall back to `ytsearch:"{track} {artist}"` if ISRC search yields no results

---

### Requirement: SpotifySearcher extracts and validates Spotify track URLs

The system MUST parse Spotify URLs to extract the resource type and ID, and validate that the resource is a supported type.

#### Scenario: Valid Spotify track URL is parsed

- GIVEN a URL in the form `https://open.spotify.com/track/{id}` or `spotify:track:{id}`
- WHEN `Search` is called
- THEN the system MUST extract the track ID
- AND proceed with Spotify API and ytsearch

#### Scenario: Invalid Spotify URL format

- GIVEN a URL that does not match a known Spotify pattern (e.g., `https://open.spotify.com/`)
- WHEN `Search` is called
- THEN `Search` MUST return a `domain.Error` with code `ErrorInvalidURL`
- AND the error message MUST indicate the URL format is invalid

#### Scenario: Unsupported Spotify resource type (playlist, album, artist)

- GIVEN a Spotify URL for a playlist, album, or artist (e.g., `https://open.spotify.com/playlist/{id}`)
- WHEN `Search` is called
- THEN `Search` MUST return a `domain.Error` with code `ErrorInvalidURL`
- AND the error message MUST indicate that only track URLs are supported (this is the first iteration; playlist/album resolution is future work)

#### Scenario: Non-Spotify URL passed to SpotifySearcher

- GIVEN a YouTube URL (e.g., `https://youtube.com/watch?v=xxx`)
- WHEN `Search` is called on the `SpotifySearcher`
- THEN `Search` MUST return a `domain.Error` with code `ErrorInvalidURL`
- AND the error message MUST indicate the URL is not a Spotify URL

---

### Requirement: SpotifySearcher handles rate limiting from Spotify API

The system MUST detect and report Spotify API rate limiting (HTTP 429).

#### Scenario: Rate limited by Spotify API

- GIVEN a `SpotifySearcher`
- WHEN the Spotify API returns HTTP 429
- THEN `Search` MUST return a `domain.Error` with code `ErrorNetwork`
- AND the error message SHOULD indicate rate limiting and suggest retrying later
- AND the system SHOULD NOT retry automatically

---

### Requirement: SpotifySearcher handles network errors

The system MUST handle transient network failures when calling the Spotify API.

#### Scenario: Network timeout or DNS failure

- GIVEN a `SpotifySearcher`
- WHEN an HTTP request to the Spotify API fails due to a network error (timeout, DNS resolution, connection refused)
- THEN `Search` MUST return a `domain.Error` with code `ErrorNetwork`
- AND the error message MUST include details about the network failure

---

### Requirement: SpotifySearcher configuration is validated at creation

The system MUST validate that credentials are provided before making any API calls.

#### Scenario: Missing client ID

- GIVEN `NewSpotifySearcher("", "valid-secret")`
- WHEN called
- THEN it MUST return a `domain.Error` with code `ErrorGeneric`
- AND the error MUST indicate that `spotify_client_id` is missing

#### Scenario: Missing client secret

- GIVEN `NewSpotifySearcher("valid-id", "")`
- WHEN called
- THEN it MUST return a `domain.Error` with code `ErrorGeneric`
- AND the error MUST indicate that `spotify_client_secret` is missing

---

### Requirement: yt-dlp search uses flat JSON parsing (shared with existing Searcher)

The system MUST reuse `searcher.ParseLine` from the existing yt-dlp searcher to parse `ytsearch` JSON output.

#### Scenario: ParseLine handles ytsearch output identically

- GIVEN a `yt-dlp ytsearch:N` JSON output line
- WHEN parsed via `ParseLine(line)`
- THEN the result MUST be the same `domain.Media` structure as a regular yt-dlp search
- AND `Media.Source` MUST be overwritten to `"spotify"` after parsing (the SpotifySearcher sets source, not ParseLine)

---

### UI Requirement: Source selection in input screen

The TUI MUST allow the user to choose which source (Spotify or YouTube) to resolve URLs from.

#### Scenario: Input screen shows source toggle

- GIVEN a `Model` on `ScreenInput`
- THEN the view MUST display a source selector or toggle
- AND the default source MUST be YouTube
- AND the user MUST be able to switch between sources with a key binding (e.g., `Tab` or `s`)
- AND the model MUST store the selected source

#### Scenario: Spotify source routes to SpotifySearcher

- GIVEN a `Model` with `selectedSource = "spotify"`
- WHEN the user enters a URL and presses Enter
- THEN the system MUST invoke `SpotifySearcher.Search()` instead of the YouTube searcher
- AND the `resolveCmd` MUST know which searcher to use based on the selected source

#### Scenario: UI indicates Spotify resolution in progress

- GIVEN a `Model` with `selectedSource = "spotify"`
- WHEN resolving a URL
- THEN the resolving screen MUST display a message indicating it's resolving via Spotify

---

### UI Requirement: Spotify errors are displayed in the UI

The TUI MUST display Spotify-specific error messages to the user.

#### Scenario: No YouTube match shows specific error

- GIVEN a Spotify track that resolved metadata but found no YouTube match
- WHEN `Search` returns `ErrorTrackUnavailable`
- THEN the UI MUST show a message like `"Track found on Spotify but no match on YouTube: {track name} - {artist}"`
- AND the user MUST be returned to the input screen

#### Scenario: Configuration error shows actionable message

- GIVEN a `SpotifySearcher` created with missing credentials
- WHEN `Search` is called
- THEN the UI MUST display an error like `"Spotify not configured: missing spotify_client_id in ~/.config/music-dl/config.toml"`
- AND the user MUST be returned to the input screen

---

### Configuration Requirement: Spotify credentials in config file

The system MUST read Spotify API credentials from a TOML configuration file.

#### Scenario: Config file is read from default location

- GIVEN `~/.config/music-dl/config.toml` exists with content:

  ```toml
  [spotify]
  client_id = "abc123"
  client_secret = "secret456"
  ```

- WHEN the application starts with Spotify source selected
- THEN the system MUST read the config file
- AND MUST extract `spotify.client_id` and `spotify.client_secret`
- AND MUST pass them to `NewSpotifySearcher`

#### Scenario: Config file is missing

- GIVEN `~/.config/music-dl/config.toml` does not exist
- WHEN a Spotify search is attempted
- THEN the system MUST return a configuration error
- AND the error MUST include instructions for creating the config file

#### Scenario: Config file exists but missing Spotify section

- GIVEN `~/.config/music-dl/config.toml` exists but has no `[spotify]` section
- WHEN a Spotify search is attempted
- THEN the system MUST return a `domain.Error`
- AND the error MUST indicate that the Spotify configuration section is missing

#### Scenario: Config file has empty Spotify credentials

- GIVEN `~/.config/music-dl/config.toml` with `client_id = ""` or `client_secret = ""`
- WHEN a Spotify search is attempted
- THEN the system MUST return a `domain.Error`
- AND the error MUST indicate the missing field(s)

---

### Non-Functional Requirement: No external HTTP/OAuth dependencies

The system MUST use Go's standard `net/http` package for all HTTP requests. No third-party OAuth or HTTP client libraries MAY be added.

#### Scenario: HTTP requests use net/http only

- GIVEN the `SpotifySearcher` implementation
- WHEN inspected for imports
- THEN `import "net/http"` MUST be the only HTTP client import
- AND `golang.org/x/oauth2` or similar MUST NOT be imported
- AND the Client Credentials flow MUST be implemented manually

---

### Non-Functional Requirement: Unit tests with HTTP mock

The `SpotifySearcher` MUST be testable with a mock HTTP server (e.g., `httptest.Server`).

#### Scenario: Track resolution with mocked Spotify API and mocked yt-dlp

- GIVEN an `httptest.Server` returning valid Spotify track metadata
- AND a mock yt-dlp binary (or exec override for testing)
- WHEN `Search` is called with a valid Spotify track URL
- THEN the test MUST verify the correct API calls were made
- AND the resulting `Media` MUST have the expected metadata

---

### Non-Functional Requirement: Integration tests are opt-in

Tests that require real Spotify API credentials MUST be behind `testing.Short()` guard.

#### Scenario: Integration test structure

- GIVEN an integration test file in the package
- WHEN `testing.Short()` is true
- THEN the test MUST skip with a message indicating it requires Spotify credentials
- WHEN `testing.Short()` is false and credentials are available via environment variables
- THEN the test MUST run against the real Spotify API

---

## Error Scenarios

| Scenario | Trigger | Expected Behavior |
| --- | --- | --- |
| Invalid Spotify URL | URL doesn't match `open.spotify.com/track/{id}` or `spotify:track:{id}` | `ErrorInvalidURL`, message describes valid formats |
| Unsupported resource | URL is for playlist/album/artist | `ErrorInvalidURL`, message says only tracks supported in this iteration |
| Non-Spotify URL to SpotifySearcher | YouTube URL passed to Spotify searcher | `ErrorInvalidURL`, message says URL is not a Spotify URL |
| Invalid credentials | Bad client_id or client_secret | `ErrorGeneric` with authentication failure message |
| Missing config file | `~/.config/music-dl/config.toml` doesn't exist | `ErrorGeneric` with instructions to create it |
| Missing config fields | Config exists but fields are missing/empty | `ErrorGeneric` naming the missing field |
| Token expired | Spotify API returns 401 | Automatic retry with fresh token; if retry fails, `ErrorGeneric` |
| Rate limited | Spotify API returns 429 | `ErrorNetwork`, suggest retry later |
| Network error | Timeout, DNS failure, connection refused | `ErrorNetwork` with details |
| YouTube no match (ISRC) | `ytsearch:isrc:{ISRC}` returns zero results | Fall back to name+artist search |
| YouTube no match (name) | `ytsearch:"track artist"` returns zero results | `ErrorTrackUnavailable` with track name and artist; no silent skip |
| yt-dlp not found | yt-dlp binary missing from PATH | `ErrorBinaryNotFound` (same as existing searcher) |
| yt-dlp non-zero exit | yt-dlp crashes or returns error | Error with stderr diagnostics |

---

## Test Specifications

### Test: Spotify URL parsing (unit)

**File:** `internal/adapters/spotify/url_test.go`

| Case | Input | Expected |
| --- | --- | --- |
| Valid track URL | `https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh` | ID extracted, no error |
| Valid URI format | `spotify:track:4iV5W9uYEdYUVa79Axb7Rh` | ID extracted, no error |
| Invalid URL | `https://open.spotify.com/` | Error |
| Playlist URL | `https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M` | Error (unsupported in this iteration) |
| Album URL | `https://open.spotify.com/album/1kfVbJpH1WPqOjLwfoCmXr` | Error (unsupported in this iteration) |
| Non-Spotify URL | `https://youtube.com/watch?v=xxx` | Error |

### Test: Spotify API authentication (unit)

**File:** `internal/adapters/spotify/auth_test.go`

| Case | Server Response | Expected |
| --- | --- | --- |
| Successful token grant | HTTP 200 with `{"access_token":"tok","token_type":"Bearer","expires_in":3600}` | Token cached, no error |
| Invalid credentials | HTTP 400 or 401 | Error returned |
| Network failure | Server unreachable | `ErrorNetwork` |

### Test: Spotify track resolution (unit with HTTP mock)

**File:** `internal/adapters/spotify/spotify_test.go`

| Case | Mock Setup | Expected |
| --- | --- | --- |
| Track found | Mock Spotify API + mock yt-dlp returning a video | 1 Media with correct metadata, Source="spotify" |
| Track multiple artists | Mock returns 2 artists | Artist="Artist1, Artist2" |
| No YouTube match | Mock Spotify API, mock yt-dlp returns empty | `ErrorTrackUnavailable` |
| Spotify API 429 | Mock returns 429 | `ErrorNetwork` |
| Spotify API 401 then retry succeeds | Mock returns 401 first, 200 on retry | Success after token refresh |
| Missing client_id | Empty client_id passed to constructor | Constructor returns error |

### Test: ytsearch integration (integration, opt-in)

**File:** `internal/adapters/spotify/ytsearch_test.go`

**Skip condition:** `testing.Short()` — requires real yt-dlp on `$PATH`.

| Case | Query | Expected |
| --- | --- | --- |
| ISRC search | Track with known ISRC | Returns at least 1 result |
| Name+artist search | `ytsearch:"Blinding Lights The Weeknd"` | Returns at least 1 result |
| No results | Bizarre non-existent query | 0 results, no error from yt-dlp |

### Test: End-to-end Spotify resolution (integration, opt-in)

**File:** `internal/adapters/spotify/e2e_test.go`

**Skip condition:** `testing.Short()` — requires real Spotify credentials AND real yt-dlp.

| Case | URL | Expected |
|---|---|---|
| Real track resolution | Valid Spotify track URL with real credentials | Returns 1 Media with URL, Title, Artist, Source="spotify" |
