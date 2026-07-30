# Design: Spotify Adapter

## Status

- **Artifact:** Design
- **Change:** `spotify-adapters`
- **Based on:** Proposal decisions (source selection in TUI, YouTube auto-resolve, no SoundCloud)
- **Date:** 2026-07-29

---

## 1. Package Structure

```
internal/adapters/spotify/
├── spotify.go          # SpotifySearcher struct, constructor, Search()
├── auth.go             # OAuth2 Client Credentials flow, token cache/refresh
├── config.go           # Config file loading (JSON), ConfigPath
├── resolve.go          # YouTube resolution via yt-dlp ytsearch
├── router.go           # SourceRouter — delegates Searcher calls by URL/source
├── spotify_test.go     # Unit tests (httptest.Server for Spotify API)
├── auth_test.go        # Token management tests
├── config_test.go      # Config loading tests
└── resolve_test.go     # YouTube resolution tests (mocked exec)
```

### Dependency Diagram

```
ports.Searcher (interface)
      ▲
      │
SourceRouter ──┬──▶ searcher.Searcher (YouTube)
               │
               └──▶ SpotifySearcher ──▶ (parses URL, calls Spotify API)
                        │
                        └──▶ resolve.go ──▶ searcher.ParseLine()
                                │
                                └──▶ os/exec (yt-dlp ytsearch)
```

Key dependency rules:

- `SpotifySearcher` implements `ports.Searcher` (same as existing yt-dlp searcher)
- `SourceRouter` also implements `ports.Searcher` — delegates to the right adapter
- `resolve.go` reuses `searcher.ParseLine()` — no duplication of JSON parsing
- `config.go` has zero dependencies on Spotify API — pure file I/O
- TUI **never imports** adapter packages directly — everything goes through `ports.Searcher`

### File Responsibilities

| File | Responsibility | Imports |
| ------ | --------------- | --------- |
| `spotify.go` | Search flow orchestration, URL parsing, error handling | `context`, `net/http`, `core/ports`, `core/domain` |
| `auth.go` | Token acquisition & refresh via Client Credentials flow | `context`, `net/http`, `encoding/json`, `sync` |
| `config.go` | JSON config loading from `~/.config/music-dl/config.json` | `encoding/json`, `os`, `path/filepath` |
| `resolve.go` | Runs yt-dlp ytsearch, parses results via `ParseLine` | `context`, `os/exec`, `searcher` (for ParseLine), `core/domain` |
| `router.go` | SourceRouter — routes `Search()` by URL pattern or mode | `context`, `core/ports` |

---

## 2. SpotifySearcher

### Struct

```go
type SpotifySearcher struct {
    clientID     string
    clientSecret string
    httpClient   *http.Client      // reused across requests (includes token transport)
    tokenMu      sync.Mutex
    token        *oauth2Token      // cached token, refreshed automatically
}

type oauth2Token struct {
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
    ExpiresIn   int    `json:"expires_in"`
    ExpiresAt   time.Time         // computed: time.Now() + ExpiresIn - 60s buffer
}
```

### Constructor

```go
// NewSpotifySearcher creates a Spotify adapter that resolves Spotify
// URLs by calling the Spotify Web API and resolving each track to YouTube.
func NewSpotifySearcher(clientID, clientSecret string) *SpotifySearcher {
    return &SpotifySearcher{
        clientID:     clientID,
        clientSecret: clientSecret,
        httpClient:   &http.Client{Timeout: 10 * time.Second},
    }
}
```

The constructor DOES NOT validate credentials at construction time. Validation happens lazily on first `Search()` call. This lets the app start even if Spotify is not configured (the SourceRouter will simply not route to it).

### Fields

| Field | Type | Purpose |
| ------- | ------ | --------- |
| `clientID` | `string` | Spotify API Client ID |
| `clientSecret` | `string` | Spotify API Client Secret |
| `httpClient` | `*http.Client` | Shared HTTP client with timeout |
| `tokenMu` | `sync.Mutex` | Guards token access for concurrent calls |
| `token` | `*oauth2Token` | Cached OAuth2 token (nil = not yet fetched) |

### Search() — Full Flow

```
Search(ctx, url)
  │
  ├─ 1. Parse URL → determine type: track | album | playlist
  │    - "open.spotify.com/track/..."  → track
  │    - "open.spotify.com/album/..."  → album
  │    - "open.spotify.com/playlist/..." → playlist
  │    - other → error (domain.ErrorInvalidURL)
  │
  ├─ 2. Get token (auth.go)
  │    - If token is nil or expired → POST /api/token (Client Credentials)
  │    - Cache: store token + computed ExpiresAt (now + ExpiresIn - 60s grace)
  │    - Mutex-guarded for concurrent safety
  │
  ├─ 3. Call Spotify API
  │    - GET /v1/tracks/{id}              (track)
  │    - GET /v1/albums/{id}/tracks       (album)
  │    - GET /v1/playlists/{id}/tracks    (playlist)
  │    - Headers: Authorization: Bearer {token}
  │
  ├─ 4. Parse Spotify response → []domain.Media
  │    - Map: artist(s).name → Artist, name → Title, duration_ms → Duration
  │    - Source = "spotify", URL = Spotify track URL
  │    - Status = StatusPending
  │
  ├─ 5. For EACH track → YouTube resolution (resolve.go)
  │    - yt-dlp ytsearch1 "{artist} {title}"
  │    - Parse output via searcher.ParseLine()
  │    - On success: replace Media.URL with YouTube URL, keep title/artist from Spotify
  │    - On failure: set Media.Error = "no YouTube match", Status = StatusFailed
  │    - **No skips** — every track gets a result, even if error
  │
  └─ 6. Return ports.SearchResult
       - Tracks: the resolved tracks (some may have StatusFailed)
       - Source: "spotify"
       - If ALL tracks failed → return error
       - If partial failures → return tracks + error (same pattern as yt-dlp searcher)
```

### URL Parsing

URLs are parsed via standard `net/url` + path splitting:

```
"https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
  → entity = "track", id = "4iV5W9uYEdYUVa79Axb7Rh"

"https://open.spotify.com/album/7hIVC7KO8uFTtO7aFRCJIO"
  → entity = "album", id = "7hIVC7KO8uFTtO7aFRCJIO"

"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M"
  → entity = "playlist", id = "37i9dQZF1DXcBWIGoYBM5M"
```

Spotify IDs are alphanumeric (base62), typically 22 chars. Validate that the extracted ID is non-empty and matches `[a-zA-Z0-9]+`.

### No-Match Handling

When **all tracks** in a Spotify query fail YouTube resolution:

```go
return ports.SearchResult{
    Tracks: tracks, // every track has StatusFailed + Error set
    Source: "spotify",
}, fmt.Errorf("no tracks could be resolved: %d/%d failed", failed, total)
```

The TUI will show the error AND the failed tracks (decision: "mostrar error, no skipear").

---

## 3. Auth — OAuth2 Token Management

### Location: `internal/adapters/spotify/auth.go`

### Token Acquisition

Spotify uses OAuth2 **Client Credentials** flow (no user login needed for read-only):

```http
POST https://accounts.spotify.com/api/token
Content-Type: application/x-www-form-urlencoded
Authorization: Basic {base64(client_id:client_secret)}

grant_type=client_credentials
```

Response:

```json
{
    "access_token": "BQ...",
    "token_type": "Bearer",
    "expires_in": 3600
}
```

### Token Cache & Refresh Strategy

```go
func (s *SpotifySearcher) getToken(ctx context.Context) (string, error)
```

- Check if `s.token` is nil → fetch new token
- Check if `s.token.ExpiresAt` is before `time.Now()` → fetch new token (60s grace buffer)
- Otherwise → return cached `s.token.AccessToken`
- All guarded by `s.tokenMu` — but a simple mutex is fine since yt-dlp resolution per track is the bottleneck

No refresh token needed — Client Credentials grants a new token on every request. We simply re-fetch when expired.

### Error Handling

- Network error → `domain.Error{Code: ErrorNetwork}`
- Invalid credentials (401) → `domain.Error{Code: ErrorGeneric, Message: "Spotify credentials rejected"}`
- Rate limiting (429) → parse Retry-After header, return `domain.Error{Code: ErrorNetwork}`

---

## 4. Config System

### Location: `internal/adapters/spotify/config.go`

### Format and Location

**File:** `~/.config/music-dl/config.json`

```json
{
    "spotify": {
        "client_id": "your_client_id_here",
        "client_secret": "your_client_secret_here"
    }
}
```

### API

```go
// ConfigPath returns the default config file path.
// ~/.config/music-dl/config.json on Linux/macOS
func ConfigPath() string { ... }

// Config holds optional provider credentials.
type Config struct {
    Spotify struct {
        ClientID     string `json:"client_id"`
        ClientSecret string `json:"client_secret"`
    } `json:"spotify"`
}

// LoadConfig reads and parses the JSON config file.
// Returns nil, nil if file doesn't exist (graceful degradation).
// Returns error for malformed JSON.
func LoadConfig(path string) (*Config, error)
```

### Integration with Startup

```go
// In cmd/music-dl/main.go

func main() {
    // ... preflight checks ...

    searcherImpl := searcher.NewSearcher()  // always created (YouTube fallback)

    configPath := spotify.ConfigPath()
    cfg, err := spotify.LoadConfig(configPath)
    if err != nil {
        log.Printf("warning: invalid Spotify config at %s: %v", configPath, err)
    }

    var spotifySearcher *spotify.SpotifySearcher
    if cfg != nil && cfg.Spotify.ClientID != "" && cfg.Spotify.ClientSecret != "" {
        spotifySearcher = spotify.NewSpotifySearcher(cfg.Spotify.ClientID, cfg.Spotify.ClientSecret)
    }

    // Router decides which searcher to use per URL
    router := spotify.NewSourceRouter(searcherImpl, spotifySearcher)
    downloaderImpl := downloader.NewDownloader()
    orch := service.NewOrchestrator(router, downloaderImpl)

    m := tui.NewModel(orch, router, outputDir)
    // ...
}
```

Key principle: **Spotify config is optional**. If no config file or empty credentials, the app still works — it just uses YouTube only.

---

## 5. Source Selection in the TUI

### Strategy: Auto-Detect with Manual Toggle

The simplest UX that covers the decisions:

1. **Auto-detect mode** (default): URL is analyzed as the user types
   - `spotify.com` → Spotify source
   - `youtube.com` / `music.youtube.com` → YouTube source
   - Neither → YouTube (default fallback)

2. **Manual toggle**: User presses `Tab` on the input screen to cycle sources
   - Shows: `[YouTube]` → `[Spotify]` → `[Auto]` → `[YouTube]` ...
   - The toggle is only meaningful when URL doesn't match the desired source
   - When Spotify is configured, Spotify option is available; otherwise hidden

### Key decision: Tab toggles, Enter triggers resolve

### TUI Messages

**No new message types needed.** Reuse `resolveFinishedMsg` — it already carries `[]domain.Media` and `error`. The `Media.Source` field is set by the searcher.

### Model Changes (`internal/tui/model.go`)

```go
type SourceMode int

const (
    SourceAuto     SourceMode = iota // detect from URL
    SourceYouTube                    // force YouTube
    SourceSpotify                    // force Spotify (only if configured)
)

type Model struct {
    // ... existing fields ...

    // Source selection
    sourceMode      SourceMode
    spotifyAvailable bool             // true if Spotify adapter is configured
    searcher         ports.Searcher   // keeps working as before (router)
}
```

No need for separate searcher references. The `router` implements `ports.Searcher` and handles routing internally. The TUI just tells the router which mode via... hmm, the router doesn't know about mode.

Actually, let me reconsider. The TUI needs to communicate source preference to the searcher. The `ports.Searcher` interface is `Search(ctx, url)`. We could:

**Option A: Extend the interface** — add `Source()` or pass the mode via context
**Option B: Router wraps the mode** — create a new router per mode change (awkward)
**Option C: TUI picks the searcher** — store both searchers, call the right one

I'll go with **Option C** — it's the most explicit and doesn't change `ports.Searcher`:

```go
type Model struct {
    // ... existing fields ...
    youtubeSearcher  ports.Searcher  // the original yt-dlp searcher
    spotifySearcher  ports.Searcher  // the Spotify adapter (nil if not configured)
    sourceMode       SourceMode
    sourceModeLabel  string           // for display: "Auto", "YouTube", "Spotify"
}
```

And in `startResolve`:

```go
func (m Model) startResolve(url string) (tea.Model, tea.Cmd) {
    searcher := m.selectSearcher(url)
    m.Screen = ScreenResolving
    // ...
    return m, resolveCmd(searcher, url)
}

func (m Model) selectSearcher(url string) ports.Searcher {
    switch m.sourceMode {
    case SourceSpotify:
        if m.spotifySearcher != nil {
            return m.spotifySearcher
        }
        // Fallthrough to auto if Spotify not configured
        fallthrough
    case SourceYouTube:
        return m.youtubeSearcher
    default: // SourceAuto
        if m.spotifySearcher != nil && isSpotifyURL(url) {
            return m.spotifySearcher
        }
        return m.youtubeSearcher
    }
}
```

### Update Changes (`internal/tui/update.go`)

In `handleInputKeys`, add `Tab` handler:

```go
case tea.KeyTab:
    m.cycleSourceMode()
    return m, nil
```

Where `cycleSourceMode`:

```go
func (m *Model) cycleSourceMode() {
    switch m.sourceMode {
    case SourceAuto:
        m.sourceMode = SourceYouTube
    case SourceYouTube:
        if m.spotifySearcher != nil {
            m.sourceMode = SourceSpotify
        } else {
            m.sourceMode = SourceAuto
        }
    case SourceSpotify:
        m.sourceMode = SourceAuto
    }
}
```

### View Changes (`internal/tui/view.go`)

In `renderInputView`:

```go
func (m Model) renderInputView() string {
    var b strings.Builder
    b.WriteString(m.renderHeader("♪ music-dl"))
    b.WriteString("\n\n")
    b.WriteString("Paste a YouTube or Spotify URL:\n\n")
    b.WriteString(inputStyle.Render(m.Input.View()))
    b.WriteString("\n")

    // Source mode indicator
    sourceLabel := m.sourceModeLabel()
    if sourceLabel != "" {
        b.WriteString("\n")
        b.WriteString(mutedStyle.Render("  Source: "))
        b.WriteString(emphStyle.Render(sourceLabel))
        b.WriteString(mutedStyle.Render("  (Tab to switch)"))
        b.WriteString("\n")
    }

    if m.resolveErr != "" {
        b.WriteString("\n")
        b.WriteString(errorStyle.Render("✗ " + m.resolveErr))
        b.WriteString("\n")
    }

    b.WriteString("\n")
    b.WriteString(m.renderFooter())
    return b.String()
}

func (m Model) sourceModeLabel() string {
    switch m.sourceMode {
    case SourceAuto:
        return "Auto"
    case SourceYouTube:
        return "YouTube"
    case SourceSpotify:
        return "Spotify"
    default:
        return ""
    }
}
```

### NewModel signature change

```go
func NewModel(orch *service.Orchestrator, youtubeSearcher, spotifySearcher ports.Searcher, outputDir string) Model {
    // ...
    return Model{
        // ...
        youtubeSearcher: youtubeSearcher,
        spotifySearcher: spotifySearcher,
        sourceMode:      SourceAuto,
    }
}
```

---

## 6. SourceRouter (Optional Simplicity Path)

If we don't want to change the TUI model to hold two searchers, we can use a **SourceRouter** that wraps both and uses context values or a separate method:

```go
// internal/adapters/spotify/router.go

package spotify

import (
    "context"
    "strings"

    "github.com/Juanstudy/music-downloader/internal/core/ports"
)

type sourceKey struct{}

// WithSource stores the preferred source in context.
func WithSource(ctx context.Context, source string) context.Context {
    return context.WithValue(ctx, sourceKey{}, source)
}

// SourceFromContext reads the preferred source from context.
func SourceFromContext(ctx context.Context) string {
    if s, ok := ctx.Value(sourceKey{}).(string); ok {
        return s
    }
    return "auto"
}

// SourceRouter delegates Search() to the right adapter based on URL or context source.
type SourceRouter struct {
    youtube ports.Searcher
    spotify ports.Searcher
}

func NewSourceRouter(youtube, spotify ports.Searcher) *SourceRouter {
    return &SourceRouter{
        youtube: youtube,
        spotify: spotify,
    }
}

func (r *SourceRouter) Search(ctx context.Context, url string) (ports.SearchResult, error) {
    switch SourceFromContext(ctx) {
    case "spotify":
        if r.spotify != nil {
            return r.spotify.Search(ctx, url)
        }
    case "youtube":
        return r.youtube.Search(ctx, url)
    }

    // Auto-detect
    if r.spotify != nil && isSpotifyURL(url) {
        return r.spotify.Search(ctx, url)
    }
    return r.youtube.Search(ctx, url)
}

func isSpotifyURL(url string) bool {
    return strings.Contains(url, "open.spotify.com") || strings.Contains(url, "spotify.com/track")
}
```

**Final recommendation:** Use the **SourceRouter** approach. It:

- Keeps `ports.Searcher` unchanged
- Avoids changing the TUI to hold multiple searchers
- Keeps routing logic in the adapter layer
- The TUI only needs to add `context.WithValue(ctx, "source", "spotify")` when calling `Search`
- But... the current TUI doesn't pass a meaningful context — it uses `context.Background()` in `resolveCmd`

Given the current TUI uses `context.Background()`, the **Option C** (TUI holds both searchers) is simpler to implement. The TUI already owns the searcher reference. Adding a second reference is minimal diff.

**Final recommendation: Option C** — TUI holds both searchers and selects at call time. The router approach is architecturally cleaner but adds indirection for no concrete benefit here.

---

## 7. YouTube Resolution (`resolve.go`)

### How it reuses `searcher.ParseLine()`

```go
package spotify

import (
    "bufio"
    "context"
    "fmt"
    "os/exec"
    "strings"

    "github.com/Juanstudy/music-downloader/internal/adapters/searcher"
    "github.com/Juanstudy/music-downloader/internal/core/domain"
)

// resolveTrack searches YouTube for a Spotify track via yt-dlp ytsearch
// and returns the track with YouTube URL populated.
func resolveTrack(ctx context.Context, track domain.Media) (domain.Media, error) {
    query := fmt.Sprintf("%s %s", track.Artist, track.Title)
    query = strings.TrimSpace(query)
    if query == "" {
        track.Status = domain.StatusFailed
        track.Error = "no artist or title to search"
        return track, fmt.Errorf("cannot resolve track without artist or title")
    }

    args := []string{
        "--default-search", "ytsearch",
        "--flat-playlist",
        "--dump-json",
        "--ignore-errors",
        "--no-warnings",
        fmt.Sprintf("ytsearch1:%s", query),
    }

    cmd := exec.CommandContext(ctx, "yt-dlp", args...)
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        track.Status = domain.StatusFailed
        track.Error = fmt.Sprintf("yt-dlp pipe: %v", err)
        return track, err
    }

    stderr := new(strings.Builder)
    cmd.Stderr = stderr

    if err := cmd.Start(); err != nil {
        track.Status = domain.StatusFailed
        track.Error = fmt.Sprintf("yt-dlp start: %v", err)
        return track, err
    }

    scanner := bufio.NewScanner(stdout)
    scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

    var resolved domain.Media
    found := false

    for scanner.Scan() {
        parsed, err := searcher.ParseLine(scanner.Text())
        if err != nil {
            continue
        }
        if !found {
            resolved = parsed
            found = true
        }
    }

    if scanErr := scanner.Err(); scanErr != nil {
        track.Status = domain.StatusFailed
        track.Error = fmt.Sprintf("read yt-dlp output: %v", scanErr)
        return track, scanErr
    }

    if err := cmd.Wait(); err != nil {
        if !found {
            track.Status = domain.StatusFailed
            track.Error = fmt.Sprintf("no YouTube match found for %q", track.Title)
            return track, fmt.Errorf("no YouTube result: %w\n%s", err, stderr.String())
        }
        // Partial: we have a result but yt-dlp emitted stderr
    }

    if !found {
        track.Status = domain.StatusFailed
        track.Error = fmt.Sprintf("no YouTube match found for %q", track.Title)
        return track, fmt.Errorf("no YouTube result for %s - %s", track.Artist, track.Title)
    }

    // Merge: keep Spotify metadata (title, artist), replace URL with YouTube result
    track.URL = resolved.URL
    track.Source = "youtube"       // the actual playable source
    track.Duration = resolved.Duration
    track.Status = domain.StatusPending
    track.Error = ""

    return track, nil
}
```

### Error Handling Summary

| Scenario | Result |
| ---------- | -------- |
| Spotify API down | All tracks get `StatusFailed` + error, `Search()` returns error |
| Specific track has no YouTube match | That track gets `StatusFailed` + `Error`, other tracks proceed |
| yt-dlp not found | `resolveTrack` fails with exec error, track gets `StatusFailed` |
| Context cancelled | `exec.CommandContext` kills yt-dlp, error propagates |

---

## 8. Wiring in `main.go`

### Current

```go
searcherImpl := searcher.NewSearcher()
downloaderImpl := downloader.NewDownloader()
orch := service.NewOrchestrator(searcherImpl, downloaderImpl)
m := tui.NewModel(orch, searcherImpl, outputDir)
```

### New

```go
searcherImpl := searcher.NewSearcher()

// Optional: Spotify adapter
var spotifySearcher ports.Searcher
configPath := spotify.ConfigPath()
cfg, err := spotify.LoadConfig(configPath)
if err != nil {
    log.Printf("warning: invalid spotify config at %s: %v", configPath, err)
}
if cfg != nil && cfg.Spotify.ClientID != "" && cfg.Spotify.ClientSecret != "" {
    spotifySearcher = spotify.NewSpotifySearcher(cfg.Spotify.ClientID, cfg.Spotify.ClientSecret)
}

downloaderImpl := downloader.NewDownloader()
orch := service.NewOrchestrator(searcherImpl, downloaderImpl)
m := tui.NewModel(orch, searcherImpl, spotifySearcher, outputDir)
```

Config path resolution order:

1. `MUSIC_DL_CONFIG` env var (if set)
2. `~/.config/music-dl/config.json` (default)
3. File not found → no Spotify (graceful, no error)

---

## 9. Test Strategy

### 9.1 Unit Tests: Spotify API Mocking (`spotify_test.go`)

Use `httptest.NewServer` to simulate the Spotify Web API:

```go
func TestSpotifySearcher_Track_Success(t *testing.T) {
    // Start mock Spotify server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/api/token":
            // Return OAuth2 token
            json.NewEncoder(w).Encode(map[string]interface{}{
                "access_token": "test-token",
                "token_type":   "Bearer",
                "expires_in":   3600,
            })
        case "/v1/tracks/4iV5W9uYEdYUVa79Axb7Rh":
            // Return track data
            json.NewEncoder(w).Encode(map[string]interface{}{
                "name": "Never Gonna Give You Up",
                "artists": []map[string]interface{}{
                    {"name": "Rick Astley"},
                },
                "duration_ms": 212000,
                "external_urls": map[string]string{
                    "spotify": "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
                },
            })
        }
    }))
    defer server.Close()

    // Override API endpoint for testing
    s := NewSpotifySearcher("test-id", "test-secret")
    s.apiBaseURL = server.URL  // injected for testing

    result, err := s.Search(context.Background(),
        "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh")
    // Assertions...
}
```

Test cases:

| Test | What it covers |
| ------ | --------------- |
| `Track_Success` | Single track, token fetch + track API call |
| `Album_Success` | Album with multiple tracks |
| `Playlist_Success` | Playlist with paginated results |
| `Track_NoMatch` | Track found on Spotify but no YouTube match |
| `Token_Expired` | Token expiry triggers auto-refresh on next call |
| `Token_Unauthorized` | Invalid credentials return 401 |
| `RateLimited` | 429 response handled gracefully |
| `InvalidURL` | Non-Spotify URL or malformed path |
| `ContextCancelled` | Context cancellation propagates |

### 9.2 Token Tests (`auth_test.go`)

| Test | What it covers |
| ------ | --------------- |
| `GetToken_FirstCall` | Nil token → fetches new token |
| `GetToken_Cached` | Valid token → returns cached (no HTTP call) |
| `GetToken_Expired` | Expired token → fetches new token |
| `GetToken_Concurrent` | Multiple goroutines don't race on token |

### 9.3 Config Tests (`config_test.go`)

| Test | What it covers |
| ------ | --------------- |
| `LoadConfig_Valid` | Correctly parses valid JSON config |
| `LoadConfig_FileNotFound` | Returns nil, nil (graceful degradation) |
| `LoadConfig_Malformed` | Returns error for invalid JSON |
| `ConfigPath_Default` | Returns correct default path (platform-aware) |

### 9.4 YouTube Resolution Tests (`resolve_test.go`)

**Strategy:** Extract `resolveTrack` as a package-level function that accepts a `commandRunner` interface for testability.

```go
// In resolve.go:
type commandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd

var runCommand commandRunner = exec.CommandContext

// In tests:
func TestResolveTrack_Success(t *testing.T) {
    // Override runCommand to return known JSON output
    // without calling real yt-dlp
    runCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
        // Return a cmd that outputs static JSON
    }
    defer func() { runCommand = exec.CommandContext }()
}
```

| Test | What it covers |
| ------ | --------------- |
| `ResolveTrack_Success` | yt-dlp returns valid JSON → parsed correctly |
| `ResolveTrack_NoOutput` | yt-dlp returns nothing → error |
| `ResolveTrack_MalformedOutput` | yt-dlp returns invalid JSON → error |
| `ResolveTrack_ytdlpMissing` | exec.LookPath fails → error |

### 9.5 Integration Tests (opt-in)

Skipped with `testing.Short()`. Require:

- Real Spotify API credentials in env: `SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET`
- `yt-dlp` on `$PATH`

---

## 10. Risks

| Risk | Mitigation |
| ------ | ----------- |
| **Spotify API rate limiting** | Token caching reduces calls. Only 1 API call per search, then yt-dlp for resolution. |
| **Config file security** | `~/.config/music-dl/config.json` uses standard Unix permissions. Document that users should `chmod 600`. |
| **yt-dlp ytsearch quality** | First result may not match the exact track. Current approach takes the top result. Documented limitation. |
| **No-match UX** | Per decision: error shown, track not skipped. TUI surfaces partial failures. |
| **Concurrent Spotify searches** | `tokenMu` guards token refresh. Network calls are per-request. |

---

## 11. Migration Path

This design **does not break** the existing `ports.Searcher` interface or the `Orchestrator`. The Spotify adapter is a new `ports.Searcher` implementation. The TUI changes are additive (new field, new key handler, new view text).

Backward compatibility:

- No config file → app works as before (YouTube only)
- Existing TUI keyboard shortcuts unchanged
- All existing tests continue passing

---

## 12. Open Questions for Implementation

1. **`apiBaseURL` injection**: During `Search()`, the SpotifySearcher calls `accounts.spotify.com` and `api.spotify.com`. For testing, these need to be overridable. Design proposal: `struct { apiBaseURL, accountsBaseURL string }` with production defaults.

2. **yt-dlp binary path**: The existing `searcher.Searcher` hardcodes `"yt-dlp"`. For consistency, `resolveTrack` should also just use `"yt-dlp"`. If we need a configurable path later, it goes in a separate change.

3. **Config file directory**: `~/.config/music-dl/` follows XDG. Should we use `$XDG_CONFIG_HOME` if set? Design proposal: respect `$XDG_CONFIG_HOME` for Linux, `~/Library/Application Support/music-dl/` for macOS. First iteration: just `~/.config/music-dl/`.
