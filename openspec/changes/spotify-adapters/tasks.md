# Tasks: Spotify Adapter

## Review Workload Forecast

| Field | Value |
| ------- | ------- |
| Estimated changed lines | ~1100–1200 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Config + package setup) → PR 2 (Auth + Spotify API Client) → PR 3 (YouTube resolution + Full Search flow) → PR 4 (TUI source selection + Wiring) |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

```text
Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High
```

## Dependencies between batches

```
Batch 1 (Setup + Config) ──→ Batch 2 (Auth) ──→ Batch 4 (Spotify API Client) ──→ Batch 5 (YouTube resolution) ──→ Batch 6 (Full Search)
                                                                                                                      │
Batch 3 (URL parsing) ───────────────────────────────────────────────────────────────────────────────────────────────┘
                                                                                                                      │
                                                                             Batch 7 (TUI source selection) ───────────┤
                                                                                                                                          │
                                                                             Batch 8 (Wiring in main.go) ←──────────────────────────────┘

Batch 9 (Tests): parallel to all above, no hard dependency.
```

---

## Batch 1: Setup + Config (PR 1)

### 1.1 Create package structure and add TOML dependency

- **Files to modify:** `go.mod`
- **Files to create:** `internal/adapters/spotify/` (directory)
- **Description:** Add `BurntSushi/toml` as a project dependency. Create the `internal/adapters/spotify/` package directory with an empty initial `spotify.go` placeholder.
- **Dependencies:** None
- **Verification:** `go mod tidy` completes without error. `go build ./...` passes. Directory `internal/adapters/spotify/` exists.
- **Owner:** implementation
- [x] Add `BurntSushi/toml` to `go.mod`, run `go mod tidy`, create `internal/adapters/spotify/` directory. <!-- sdd-owner: implementation -->

### 1.2 Implement TOML config loading (`config.go`)

- **Files to create:** `internal/adapters/spotify/config.go`
- **Description:** Implement `ConfigPath()`, `Config` struct, and `LoadConfig(path)`.
  - `ConfigPath()` returns `~/.config/music-dl/config.toml` (respect `$XDG_CONFIG_HOME` if set, fallback to `~/.config`).
  - `Config` struct with `Spotify` sub-struct containing `ClientID` and `ClientSecret` (TOML tags: `client_id`, `client_secret`).
  - `LoadConfig(path)` reads and parses TOML. Returns `(*Config, nil)` on success, `(nil, nil)` if file not found (graceful degradation), error for malformed TOML.
- **Dependencies:** 1.1
- **Verification:** Unit test with valid TOML, missing file, malformed TOML.
- **Owner:** implementation
- [x] Implement `ConfigPath()`, `Config` struct, and `LoadConfig()` in `internal/adapters/spotify/config.go`. <!-- sdd-owner: implementation -->

### 1.3 Config unit tests (`config_test.go`)

- **Files to create:** `internal/adapters/spotify/config_test.go`
- **Description:** Test cases:
  - `LoadConfig_Valid`: writes a temp TOML with `[spotify]\nclient_id = "x"\nclient_secret = "y"`, loads it, asserts fields match.
  - `LoadConfig_FileNotFound`: calls with non-existent path, expects `(nil, nil)`.
  - `LoadConfig_Malformed`: writes invalid TOML, expects error.
  - `ConfigPath_Default`: asserts path contains `.config/music-dl/config.toml`.
  - `ConfigPath_XDG`: sets `$XDG_CONFIG_HOME`, asserts path uses it.
- **Dependencies:** 1.2
- **Verification:** All tests pass with `go test ./internal/adapters/spotify/`.
- **Owner:** implementation
- [x] Create `config_test.go` with all config loading test cases. <!-- sdd-owner: implementation -->

---

## Batch 2: URL Parsing (PR 1)

### 2.1 Implement Spotify URL parsing (`url.go`)

- **Files to create:** `internal/adapters/spotify/url.go`
- **Description:** Implement `parseSpotifyURL(url string) (entity string, id string, err error)` as an unexported function.
  - Supports `https://open.spotify.com/track/{id}` and `spotify:track:{id}`.
  - Returns `entity = "track"`, `id = <extracted ID>` for valid track URLs.
  - Returns error with `ErrorInvalidURL` for: invalid format, missing ID, non-Spotify URL.
  - Returns error with `ErrorInvalidURL` for playlist/album/artist entities (message: "only track URLs are supported in this version").
  - Validates ID matches `[a-zA-Z0-9]+` (base62 pattern), minimum length 1.
- **Dependencies:** None (pure function, no package deps beyond domain errors)
- **Verification:** Unit test covers all scenarios from spec (valid track, valid URI, playlist → error, album → error, non-Spotify → error, empty ID → error).
- **Owner:** implementation
- [x] Implement `parseSpotifyURL()` in `internal/adapters/spotify/url.go`. <!-- sdd-owner: implementation -->

### 2.2 URL parsing unit tests (`url_test.go`)

- **Files to create:** `internal/adapters/spotify/url_test.go`
- **Description:** Table-driven tests covering:
  - `https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh` → entity="track", id="4iV5W9uYEdYUVa79Axb7Rh"
  - `spotify:track:4iV5W9uYEdYUVa79Axb7Rh` → entity="track", id="4iV5W9uYEdYUVa79Axb7Rh"
  - `https://open.spotify.com/` → error
  - `https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M` → error (unsupported)
  - `https://open.spotify.com/album/1kfVbJpH1WPqOjLwfoCmXr` → error (unsupported)
  - `https://open.spotify.com/artist/1kfVbJpH1WPqOjLwfoCmXr` → error (unsupported)
  - `https://youtube.com/watch?v=xxx` → error
  - Empty string → error, `spotify:track:` (no id) → error
- **Dependencies:** 2.1
- **Verification:** All tests pass.
- **Owner:** implementation
- [x] Create `url_test.go` with table-driven URL parsing tests. <!-- sdd-owner: implementation -->

---

## Batch 3: Auth — Client Credentials Flow (PR 2)

### 3.1 Implement token management (`auth.go`)

- **Files to create:** `internal/adapters/spotify/auth.go`
- **Description:**
  - Define internal `oauth2Token` struct with `AccessToken`, `TokenType`, `ExpiresIn`, `ExpiresAt` fields.
  - Define unexported `tokenEndpoint = "https://accounts.spotify.com/api/token"` and `apiEndpoint = "https://api.spotify.com"` as package vars (overridable for testing via `SetTokenEndpoint`/`SetAPIEndpoint` or struct fields).
  - Implement `getToken(ctx) (string, error)` on `*SpotifySearcher`: checks cache (`s.token`), returns cached if not expired (60s grace buffer before actual expiry), otherwise POSTs to token endpoint with `grant_type=client_credentials` and `Authorization: Basic base64(clientID:clientSecret)`. Mutex-guarded (`s.tokenMu`). On success caches token with computed `ExpiresAt = now + ExpiresIn - 60s`.
  - Implement `refreshToken(ctx) (string, error)`: same as getToken but skips cache check (used on 401 retry).
  - Error handling: network errors → `domain.Error{Code: ErrorNetwork}`, 400/401 → `domain.Error{Code: ErrorGeneric, Message: "Spotify credentials rejected"}`, 429 → `domain.Error{Code: ErrorNetwork, Message: "rate limited"}`.
- **Dependencies:** 1.1 (package exists)
- **Verification:** Unit test with `httptest.Server` mocking the token endpoint.
- **Owner:** implementation
- [ ] Implement token caching, Client Credentials POST, and 401 retry logic in `internal/adapters/spotify/auth.go`. <!-- sdd-owner: implementation -->

### 3.2 Auth unit tests (`auth_test.go`)

- **Files to create:** `internal/adapters/spotify/auth_test.go`
- **Description:** Tests using `httptest.Server`:
  - `GetToken_FirstCall`: nil cache → POSTs to token endpoint, caches result.
  - `GetToken_Cached`: valid cached token → returns cached, no HTTP call.
  - `GetToken_Expired`: expired cached token → re-fetches.
  - `GetToken_InvalidCredentials`: server returns 400 → error.
  - `GetToken_RateLimited`: server returns 429 → `ErrorNetwork`.
  - `GetToken_ConcurrentSafe`: multiple goroutines call `getToken` concurrently → no race (use `-race` flag).
- **Dependencies:** 3.1
- **Verification:** All tests pass with `go test -race ./internal/adapters/spotify/`.
- **Owner:** implementation
- [ ] Create `auth_test.go` with token management tests using `httptest.Server`. <!-- sdd-owner: implementation -->

---

## Batch 4: Spotify API Client (PR 2)

### 4.1 Implement SpotifySearcher skeleton (`spotify.go`)

- **Files to create/modify:** `internal/adapters/spotify/spotify.go`
- **Description:**
  - Define `SpotifySearcher` struct with fields: `clientID`, `clientSecret`, `httpClient`, `tokenMu`, `token`, `accountsBaseURL` (default `https://accounts.spotify.com`), `apiBaseURL` (default `https://api.spotify.com`).
  - Constructor `NewSpotifySearcher(clientID, clientSecret string) (*SpotifySearcher, error)`: validates both fields are non-empty, returns `domain.Error` if missing. Creates `http.Client{Timeout: 10 * time.Second}`.
  - Implement `Search(ctx, url string) (ports.SearchResult, error)`:
    1. Call `parseSpotifyURL(url)` — reject non-track entities.
    2. Get token via `getToken(ctx)`.
    3. `GET {apiBaseURL}/v1/tracks/{id}` with `Authorization: Bearer {token}`.
    4. Parse response: `name` → `Media.Title`, `artists[0].name` → `Media.Artist` (if multiple artists, join with `", "`), `duration_ms` → `Media.Duration`.
    5. Set `Media.Source = "spotify"`, `Media.Status = domain.StatusPending`.
    6. On 401: discard token, call `refreshToken`, retry request once. If 401 again → `domain.Error{Code: ErrorGeneric}`.
    7. On 429: return `domain.Error{Code: ErrorNetwork}`.
    8. On network error: return `domain.Error{Code: ErrorNetwork}`.
    9. Set `Media.URL` to the input Spotify URL (will be replaced by YouTube URL in resolve step).
  - For this batch, `Search()` returns the parsed track metadata WITHOUT YouTube resolution (no ytsearch yet). Return `Media` with Spotify URL as `URL`.
  - Note: Add a TODO comment that YouTube resolution will be added in the next batch.
- **Dependencies:** 2.1 (URL parsing), 3.1 (auth), 1.1 (package)
- **Verification:** Unit test with `httptest.Server` mocking both token and tracks endpoints.
- **Owner:** implementation
- [ ] Implement `SpotifySearcher` struct, constructor with validation, and `Search()` with Spotify API track fetch (no YouTube resolution yet) in `internal/adapters/spotify/spotify.go`. <!-- sdd-owner: implementation -->

### 4.2 Spotify API client unit tests (`spotify_test.go` — part 1)

- **Files to create:** `internal/adapters/spotify/spotify_test.go`
- **Description:** Tests using `httptest.Server` (mock both token + tracks endpoints):
  - `Track_Success`: valid token + track response → 1 Media with correct metadata, Source="spotify".
  - `Track_MultipleArtists`: response with 2 artists → Artist="Artist1, Artist2".
  - `Token_Unauthorized_Retry`: first request returns 401, refresh succeeds → final success.
  - `Token_Unauthorized_Fail`: first request returns 401, refresh also 401 → error.
  - `RateLimited`: 429 → `ErrorNetwork`.
  - `InvalidURL`: non-Spotify URL → `ErrorInvalidURL`.
  - `MissingCredentials`: empty constructor args → error.
  - `ContextCancelled`: cancelled context → error propagates.
- **Dependencies:** 4.1
- **Verification:** All tests pass.
- **Owner:** implementation
- [ ] Create `spotify_test.go` with API client tests using `httptest.Server`. <!-- sdd-owner: implementation -->

---

## Batch 5: YouTube Resolution via yt-dlp ytsearch (PR 3)

### 5.1 Implement YouTube resolution (`resolve.go`)

- **Files to create:** `internal/adapters/spotify/resolve.go`
- **Description:**
  - Implement unexported `resolveTrack(ctx, track domain.Media) (domain.Media, error)`.
  - Strategy: try ISRC first (if available from Spotify metadata), fallback to `"{artist} {title}"`.
  - Construct yt-dlp command: `yt-dlp --flat-playlist --dump-json --ignore-errors --no-warnings ytsearch1:{query}`.
    - First attempt: `ytsearch1:isrc:{ISRC}` if ISRC is present.
    - If no result: `ytsearch1:{artist} {title}` (trimmed, space-separated).
  - Reuse `searcher.ParseLine()` for JSON parsing (import `internal/adapters/searcher`).
  - Parse first valid result from `ParseLine`. Take the very first successfully parsed line.
  - **IMPORTANT**: After parsing, set `Media.Source = "spotify"` (overwrite whatever ParseLine set).
  - Merge: keep Spotify metadata (Title, Artist), replace URL with YouTube URL, keep YouTube Duration.
  - On success: return merged `Media` with `Status = StatusPending`, no error.
  - On no results: return original track with `Status = StatusFailed`, `Error = "no YouTube match: {track} - {artist}"`.
  - On yt-dlp exec error (binary not found): return error.
  - Use `exec.CommandContext` so context cancellation kills yt-dlp.
  - Use the same scanner pattern as `searcher.Searcher.Search()` with 10MB buffer.
  - Add a `setYtDlpBinary(name string)` for testing (allow overriding the binary path).
  - Define `ytsearch1` as a const with format string.
- **Dependencies:** 1.1 (package), references `searcher.ParseLine` (no new dependency)
- **Verification:** Unit test with mocked yt-dlp exec output.
- **Owner:** implementation
- [x] Implement `resolveTrack()` in `internal/adapters/spotify/resolve.go` with ytsearch via `ports.Searcher`. <!-- sdd-owner: implementation -->

### 5.2 YouTube resolution unit tests (`resolve_test.go`)

- **Files to create:** `internal/adapters/spotify/resolve_test.go`
- **Description:** Tests that mock the yt-dlp command output:
  - `ResolveTrack_Success`: mock returns valid JSON → parsed correctly, Source stays "spotify", URL is YouTube.
  - `ResolveTrack_NoOutput`: mock returns nothing → `StatusFailed`, error.
  - `ResolveTrack_MalformedOutput`: mock returns invalid JSON → `StatusFailed` (ParseLine errors are skipped).
  - `ResolveTrack_ISRCSuccess`: mock ISRC query returns result → success.
  - `ResolveTrack_ISRCFallback`: mock ISRC returns empty, name search returns result → fallback works.
  - `ResolveTrack_ContextCancelled`: context cancelled mid-exec → error propagates.
  - Implementation note: to mock exec, use a helper that replaces `exec.CommandContext` with a test function that returns known output (e.g., via environment variable or function variable override pattern from design).
- **Dependencies:** 5.1
- **Verification:** All tests pass.
- **Owner:** implementation
- [x] Create `resolve_test.go` with mocked yt-dlp output tests. <!-- sdd-owner: implementation -->

---

## Batch 6: Full Search Integration (PR 3)

### 6.1 Integrate YouTube resolution into Search()

- **Files to modify:** `internal/adapters/spotify/spotify.go`
- **Description:**
  - After step 4 (parse Spotify track metadata), call `resolveTrack(ctx, media)` for the track.
  - If `resolveTrack` succeeds (YouTube URL found): keep the resolved URL, title/artist from Spotify, source="spotify".
  - If `resolveTrack` fails: keep the Media with `StatusFailed` and the error message.
  - **No silent skips**: every track is returned, even if YouTube resolution fails.
  - Return `ports.SearchResult{Tracks: []domain.Media{track}, Source: "spotify"}`.
  - If the only track failed resolution, return the track slice AND an error summarizing the failure.
  - Remove the TODO from batch 4.1.
- **Dependencies:** 4.1 (spotify.go skeleton), 5.1 (resolve.go)
- **Verification:** Integration test with mocked Spotify API + mocked yt-dlp output.
- **Owner:** implementation
- [x] Integrate `resolveTrack()` call into `SpotifySearcher.Search()`, handle success and failure for single-track flow. <!-- sdd-owner: implementation -->

### 6.2 Full Search flow tests (extend spotify_test.go — part 2)

- **Files to modify:** `internal/adapters/spotify/spotify_test.go`
- **Description:** Add tests for the complete Search flow:
  - `Search_FullFlow_Success`: mock Spotify track API + mock yt-dlp result → returns 1 Media with YouTube URL and Source="spotify".
  - `Search_NoYouTubeMatch`: mock Spotify track API + mock yt-dlp returns empty → returns track with StatusFailed + error message.
  - `Search_SpotifyAPIDown`: mock Spotify returns 500 → error, no partial tracks.
  - `Search_InvalidURL`: non-Spotify URL → ErrorInvalidURL.
  - `Search_PlaylistURL`: playlist URL → ErrorInvalidURL with "only tracks supported" message.
- **Dependencies:** 6.1
- **Verification:** All tests pass.
- **Owner:** implementation
- [x] Extend `spotify_test.go` with full Search flow integration tests (mocked API + mocked yt-dlp). <!-- sdd-owner: implementation -->

---

## Batch 7: TUI Source Selection (PR 4)

### 7.1 Add source mode fields to Model (`model.go`)

- **Files to modify:** `internal/tui/model.go`
- **Description:**
  - Add `SourceMode` type with constants: `SourceAuto`, `SourceYouTube`, `SourceSpotify`.
  - Add fields to `Model`:
    - `sourceMode SourceMode`
    - `spotifySearcher ports.Searcher` (nil if not configured)
    - `spotifyAvailable bool` (derived from `spotifySearcher != nil`)
  - Modify `NewModel` signature: `func NewModel(orch *service.Orchestrator, youtubeSearcher, spotifySearcher ports.Searcher, outputDir string) Model`.
    - Store both searchers. Default `sourceMode = SourceAuto`.
  - Keep existing `searcher` field as `youtubeSearcher` (rename internally, keep the `ports.Searcher` interface for backward compat with searcher field usage in the struct).
  - Actually simpler: keep the existing `searcher` field for YouTube, add `spotifySearcher` field for Spotify. The `searcher` field is already used throughout the code. Don't rename to avoid many changes.
- **Dependencies:** None (standalone TUI change)
- **Verification:** Compiles, tests pass.
- **Owner:** implementation
- [x] Add `SourceMode` type, `sourceMode`, `spotifySearcher` fields to Model, update `NewModel` signature in `internal/tui/model.go`. <!-- sdd-owner: implementation -->

### 7.2 Add source selection logic (`update.go`)

- **Files to modify:** `internal/tui/update.go`
- **Description:**
  - Add `Tab` key handler in `handleInputKeys`: cycles source mode `Auto → YouTube → Spotify → Auto`.
  - If Spotify not available (`m.spotifySearcher == nil`), skip Spotify in the cycle: `Auto → YouTube → Auto`.
  - Implement `selectSearcher(url string) ports.Searcher`:
    - `SourceYouTube` → return `m.searcher` (YouTube).
    - `SourceSpotify` → if `m.spotifySearcher != nil` → return it; else fallthrough to Auto.
    - `SourceAuto` → if `m.spotifySearcher != nil` and URL contains `open.spotify.com` or `spotify:` → return Spotify; else return YouTube.
  - Modify `startResolve` to use `m.selectSearcher(url)` instead of `m.searcher` directly.
  - Pass the selected searcher to `resolveCmd`.
  - Store the selected source label in Model for display in resolving screen.
  - When resolving via Spotify, show "Resolving via Spotify..." in the resolving screen.
- **Dependencies:** 7.1
- **Verification:** Tab cycles modes, resolving uses correct searcher.
- **Owner:** implementation
- [x] Implement `selectedSearcher()`, Tab cycling, and modify `startResolve` to use the right searcher in `internal/tui/update.go`. <!-- sdd-owner: implementation -->

### 7.3 Update input and resolving views (`view.go`)

- **Files to modify:** `internal/tui/view.go`
- **Description:**
  - In `renderInputView`: add source mode indicator line after the input:
    - Show `"Source: Auto (Tab to switch)"` with appropriate styling.
    - When Spotify mode is active and Spotify is configured, show `"Source: Spotify"`.
    - When YouTube mode is active, show `"Source: YouTube"`.
    - Use `mutedStyle` for label, `emphStyle` for value.
  - In `renderResolvingView`: when source is Spotify, show "Resolving via Spotify..." instead of generic "Resolving URL...".
  - In `renderFooter`: add `Tab` key hint for input screen: `keyStyle.Render("Tab")+" "+keyDescStyle.Render("source")`.
- **Dependencies:** 7.1, 7.2
- **Verification:** Visual inspection shows source indicator and label.
- **Owner:** implementation
- [x] Update `renderInputView` with source mode indicator and `renderResolvingView` with Spotify-specific label in `internal/tui/view.go`. <!-- sdd-owner: implementation -->

### 7.4 Ensure Spotify errors display in UI

- **Files to modify:** `internal/tui/update.go`, `internal/tui/view.go` (if needed)
- **Description:**
  - When `Search` returns partial tracks + error (YouTube no-match), the existing `handleResolveDone` already handles partial results (tracks + error): it shows warning in playlist screen.
  - When `Search` returns 0 tracks + error (auth failure, config missing), the existing error flow in `handleResolveDone` shows the error in input screen.
  - Verify the error messages from SpotifySearcher are user-friendly:
    - "Spotify not configured: missing client_id in ~/.config/music-dl/config.toml"
    - "Track found on Spotify but no match on YouTube: {name} - {artist}"
  - If error messages need wrapping, do it in `handleResolveDone`.
- **Dependencies:** 6.1, 7.1
- **Verification:** Simulated errors display correctly in TUI.
- **Owner:** implementation
- [x] Verify and adjust Spotify error message display in `handleResolveDone`. <!-- sdd-owner: implementation -->

---

## Batch 8: Wiring in main.go (PR 4)

### 8.1 Wire Spotify adapter in main.go

- **Files to modify:** `cmd/music-dl/main.go`
- **Description:**
  - Add import for `internal/adapters/spotify`.
  - After creating `searcherImpl := searcher.NewSearcher()`:
    - Read config: `cfgPath := spotify.ConfigPath()`, `cfg, err := spotify.LoadConfig(cfgPath)`.
    - If `err != nil`: log warning, continue without Spotify.
    - If `cfg != nil && cfg.Spotify.ClientID != "" && cfg.Spotify.ClientSecret != ""`:
      - `spotifySearcher := spotify.NewSpotifySearcher(cfg.Spotify.ClientID, cfg.Spotify.ClientSecret)`.
      - If error (empty credentials), log warning.
    - Else: `spotifySearcher = nil`.
  - Modify `tui.NewModel` call to pass both searchers: `tui.NewModel(orch, searcherImpl, spotifySearcher, outputDir)`.
  - Modify the searcher passed to Orchestrator: currently `service.NewOrchestrator(searcherImpl, ...)`. Since the TUI selects the searcher, the Orchestrator still gets the YouTube searcher (it uses the searcher for URL validation only in `ResolveTrack`, which is not called from the new flow — actually, looking at update.go, `resolveCmd` calls `s.Search()` directly, bypassing the Orchestrator for search. So the Orchestrator's searcher is only used... let me check).

    Looking at update.go:

    ```go
    func resolveCmd(s ports.Searcher, url string) tea.Cmd {
        return func() tea.Msg {
            result, err := s.Search(context.Background(), url)
            ...
        }
    }
    ```

    And `startResolve` passes `m.searcher` (now will pass the selected searcher). So the `resolveCmd` directly calls the searcher. The Orchestrator's `ResolveTrack` is NOT called from the TUI resolve flow. But the model stores the orchestrator for downloading.

    So the wiring should be:
    - `orch := service.NewOrchestrator(searcherImpl, downloaderImpl)` — YouTube searcher is the default for the orchestrator.
    - TUI gets both `searcherImpl` (YouTube) and `spotifySearcher` (optional).
    - The TUI's `resolveCmd` picks the right searcher via `selectSearcher()`.

    This means the Orchestrator's searcher field isn't really used for resolve in the TUI flow (only `resolveCmd` uses it directly). But keep it for correctness.

  - Also pass `spotifySearcher` to `tui.NewModel`.
- **Dependencies:** 1.2 (config), 4.1 (constructor), 7.1 (NewModel signature)
- **Verification:** App starts with and without config file. With config, Spotify appears as source option. Without config, app works as before (YouTube only).
- **Owner:** implementation
- [x] Wire config loading and optional SpotifySearcher creation in `cmd/music-dl/main.go`. <!-- sdd-owner: implementation -->

---

## Batch 9: Tests (parallel)

### 9.1 Add integration tests for full Spotify resolution (`e2e_test.go`)

- **Files to create:** `internal/adapters/spotify/e2e_test.go`
- **Description:**
  - E2E test with real Spotify credentials from env vars `SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET`:
    - Guard with `testing.Short()` — skip when short.
    - Test resolves a real Spotify track URL → expects 1 Media with YouTube URL, Source="spotify", Title and Artist populated.
  - Integration test for yt-dlp ytsearch:
    - `TestYtSearch_ISRC`: query with known ISRC → at least 1 result.
    - `TestYtSearch_NameArtist`: query "Blinding Lights The Weeknd" → at least 1 result.
    - Guard with `testing.Short()`.
- **Dependencies:** 6.1 (full Search), 5.1 (resolveTrack)
- **Verification:** Tests pass with real credentials, skip with `-short`.
- **Owner:** implementation
- [ ] Create `e2e_test.go` with integration tests guarded by `testing.Short()`. <!-- sdd-owner: implementation -->

### 9.2 Run full test suite and verify nothing is broken

- **Description:** Run all tests to ensure existing tests still pass and new tests pass.
  - `go test ./... -count=1` (no short, but skip integration which checks Short internally)
  - `go test ./... -short -count=1` (explicit short mode, skips integration)
  - `go vet ./...`
- **Dependencies:** All previous batches complete
- **Verification:** All tests pass, no vet warnings.
- **Owner:** implementation
- [ ] Run full test suite and verify zero regressions. <!-- sdd-owner: implementation -->

---

## Design-Spec Reconciliation Notes

The following discrepancies between the design doc and the spec/decisions are resolved in these tasks:

| Item | Design says | Spec/Decisions say → Tasks follow |
| ------ | ------------- | ----------------------------------- |
| Config format | JSON (`config.json`) | TOML (`config.toml`) with BurntSushi/toml |
| Source field after ytsearch | Source = "youtube" | Source = "spotify" (never overwritten) |
| Album/playlist support | Design shows paginated endpoints | Spec says error (only tracks in v1) |
| Constructor validation | No validation (lazy) | Constructor validates both fields non-empty, returns error |
| Config path | `~/.config/music-dl/config.json` | `~/.config/music-dl/config.toml` |
