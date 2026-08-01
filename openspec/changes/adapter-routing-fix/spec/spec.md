# Specification: Adapter Routing Fix (Auto URL Detection + Spotify URI Support + Minimal CI)

**Change:** `adapter-routing-fix`
**Date:** 2026-08-01

## Purpose

Fix issue #19: with source mode `auto` and search mode `URL`, pasting a YouTube Music URL routes to the Spotify adapter whenever Spotify credentials are configured, failing with `✗ not a Spotify URL`. The fix makes Auto mode URL-host-driven: a recognized Spotify URL (or `spotify:` URI) routes to the Spotify searcher; everything else falls back to the general-purpose yt-dlp searcher. The change also unlocks the Spotify adapter's existing `spotify:` URI support end-to-end by relaxing the TUI URL-mode input gate, and adds a minimal CI workflow (`go vet` + `go build` + `go test -short ./...` on push and pull_request) so a routing regression like #19 becomes a machine-checked failure instead of a shipped bug.

Scope is deliberately tight: no entity-level Spotify support (track-only stays), no behavior change for users without Spotify credentials, no change to explicit source modes, no new URI schemes, and no linter/coverage in CI.

## Business Rules

- Auto mode is URL-host-driven: the Spotify searcher is used ONLY when credentials are configured AND the pasted value is a Spotify URL or `spotify:` URI; every other input resolves through yt-dlp (conservative fallback, since yt is the general resolver).
- Explicit modes keep their semantics byte-for-byte: `SourceSpotify` → Spotify searcher when configured, else yt-dlp; `SourceYouTube` → yt-dlp always.
- Spotify credentials remain required for Spotify routing; without them Auto degrades to yt-dlp exactly as today.
- The URL-mode input gate admits exactly two shapes: a string containing `://`, or a string starting with `spotify:`. Nothing else.
- Routing is a host-level question (`spotify.IsSpotifyURL`); entity-level validation stays in `parseSpotifyURL` (track-only, untouched).
- CI is minimal: `go vet ./...`, `go build ./...`, `go test -short ./...` on push and pull_request — nothing more.
- The local test runner is `go test -short ./...` (mirrors `openspec/config.yaml`); network-gated integration tests skip under `-short` so CI and local runs stay hermetic.

---

## DOMAIN: internal-tui — ADDED Requirements

Canonical at `openspec/specs/internal-tui/spec.md`.

### ADDED Requirement: ARF-001 — URL-aware searcher selection

The system MUST select the searcher for URL-mode resolution through a URL-aware helper with signature `func (m Model) selectedSearcher(url string) ports.Searcher`, and the call site MUST pass the pasted URL: `resolveCmd(m.selectedSearcher(url), url)`. In `SourceAuto` mode the Spotify searcher MUST be used only when `m.spotifySearcher != nil` AND `spotify.IsSpotifyURL(url)` is true; otherwise resolution MUST use `m.searcher` (yt-dlp). Explicit modes keep today's semantics: `SourceSpotify` MUST return `m.spotifySearcher` when non-nil and `m.searcher` otherwise; `SourceYouTube` MUST always return `m.searcher`; any other/unknown mode value MUST return `m.searcher`.

#### Scenario: Auto + YouTube Music URL + Spotify configured → yt searcher (issue #19)

- GIVEN a `Model` in `SourceAuto` with a non-nil `m.searcher` and a non-nil `m.spotifySearcher` (credentials configured)
- WHEN `selectedSearcher("https://music.youtube.com/watch?v=...")` is called
- THEN it MUST return `m.searcher`
- AND it MUST NOT return `m.spotifySearcher`

#### Scenario: Auto + open.spotify.com track URL + Spotify configured → Spotify searcher

- GIVEN a `Model` in `SourceAuto` with a non-nil `m.searcher` and a non-nil `m.spotifySearcher`
- WHEN `selectedSearcher("https://open.spotify.com/track/{id}")` is called
- THEN it MUST return `m.spotifySearcher`

#### Scenario: Auto + spotify: URI + Spotify configured → Spotify searcher

- GIVEN a `Model` in `SourceAuto` with a non-nil `m.searcher` and a non-nil `m.spotifySearcher`
- WHEN `selectedSearcher("spotify:track:{id}")` is called
- THEN it MUST return `m.spotifySearcher`

#### Scenario: Auto + Spotify URL without credentials → yt searcher

- GIVEN a `Model` in `SourceAuto` with a non-nil `m.searcher` and `m.spotifySearcher == nil` (no credentials)
- WHEN `selectedSearcher("https://open.spotify.com/track/{id}")` is called
- THEN it MUST return `m.searcher`

#### Scenario: Auto + non-Spotify URL without credentials → yt searcher

- GIVEN a `Model` in `SourceAuto` with a non-nil `m.searcher` and `m.spotifySearcher == nil`
- WHEN `selectedSearcher("https://music.youtube.com/watch?v=...")` is called
- THEN it MUST return `m.searcher`

#### Scenario: SourceYouTube ignores the URL → yt searcher

- GIVEN a `Model` in `SourceYouTube` with a non-nil `m.searcher` and a non-nil `m.spotifySearcher`
- WHEN `selectedSearcher(anyURL)` is called, including a Spotify URL or a `spotify:` URI
- THEN it MUST return `m.searcher`

#### Scenario: SourceSpotify uses Spotify when configured, yt otherwise

- GIVEN a `Model` in `SourceSpotify` with a non-nil `m.searcher` and a non-nil `m.spotifySearcher`
- WHEN `selectedSearcher(anyURL)` is called
- THEN it MUST return `m.spotifySearcher`
- GIVEN a `Model` in `SourceSpotify` with `m.spotifySearcher == nil`
- WHEN `selectedSearcher(anyURL)` is called
- THEN it MUST return `m.searcher`

#### Scenario: resolution command receives the URL-aware selection

- GIVEN a `Model` on `ScreenInput` with a pasted URL/URI in `InputText`
- WHEN the user presses Enter and resolution starts
- THEN the spawned resolution command MUST be built from `selectedSearcher(url)` with the exact pasted value
- AND the searcher returned by `selectedSearcher` MUST be the one used for resolution

---

## DOMAIN: internal-tui — MODIFIED Requirements

Canonical at `openspec/specs/internal-tui/spec.md`.

### MODIFIED Requirement: ARF-002 — Input screen behavior

(Previously: the URL-mode gate accepted only input containing `://`; `spotify:` URIs were rejected with "That doesn't look like a URL" even though the Spotify adapter could parse them.)

The system MUST accept input in URL mode when it contains `://` OR starts with the `spotify:` prefix. Input with neither MUST be rejected with the existing inline error and MUST NOT start resolution. Empty/whitespace-only input MUST still be rejected first with the existing empty-URL error (unchanged). URI schemes other than `spotify:` MUST remain rejected.

#### Scenario: Enter key with valid URL triggers resolving

- GIVEN a `Model` on `ScreenInput` with `InputText` containing a URL (a string with `://`, e.g. `https://open.spotify.com/track/{id}` or `https://music.youtube.com/watch?v=...`)
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST transition to `ScreenResolving`
- AND a `tea.Cmd` MUST be returned that resolves the URL through the searcher selected by `selectedSearcher(url)` and sends the result as a `resolveResultMsg`

#### Scenario: Enter key with a spotify: URI triggers resolving

- GIVEN a `Model` on `ScreenInput` with `InputText == "spotify:track:{id}"`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST transition to `ScreenResolving`
- AND `m.InputError` MUST NOT contain "That doesn't look like a URL"
- AND a `tea.Cmd` MUST be returned that resolves the URI through the searcher selected by `selectedSearcher(url)` and sends the result as a `resolveResultMsg`

#### Scenario: Enter key with empty URL does nothing

- GIVEN a `Model` on `ScreenInput` with `InputText == ""`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST remain `ScreenInput`
- AND `m.InputError` MUST indicate the URL is empty

#### Scenario: Enter key with input that is neither a URL nor a spotify: URI shows inline error

- GIVEN a `Model` on `ScreenInput` with `InputText` that contains no `://` and does not start with `spotify:` (e.g. `itunes:track:{id}`, `not a url`)
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST remain `ScreenInput`
- AND `m.InputError` MUST contain the "That doesn't look like a URL" message

#### Scenario: Typing characters update InputText

- GIVEN a `Model` on `ScreenInput`
- WHEN `Update()` receives a `tea.KeyMsg` with a printable character
- THEN the character MUST be appended to `m.InputText`

#### Scenario: Backspace removes last character

- GIVEN a `Model` on `ScreenInput` with non-empty `InputText`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyBackspace}`
- THEN the last character MUST be removed from `m.InputText`

#### Scenario: Esc on Input screen quits

- GIVEN a `Model` on `ScreenInput`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEsc}`
- THEN a `tea.Quit` command MUST be returned

#### Scenario: Ctrl+C quits from any screen

- GIVEN a `Model` on any screen
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyCtrlC}`
- THEN a `tea.Quit` command MUST be returned

---

## DOMAIN: adapters-spotify — ADDED Requirements

Canonical at `openspec/specs/adapters-spotify/spec.md`.

### ADDED Requirement: ARF-003 — Exported IsSpotifyURL host-level helper

The system MUST export `IsSpotifyURL(url string) bool` from the `spotify` package (`internal/adapters/spotify`). It MUST return `true` for HTTP(S) URLs whose host is exactly `spotify.com` or ends with `.spotify.com` (covers `open.spotify.com`, `www.spotify.com`, regional subdomains), and for any value starting with the `spotify:` scheme prefix regardless of entity. It MUST return `false` for empty and whitespace-only input, for `music.youtube.com` and `youtube.com`, for lookalike hosts such as `evilspotify.com` and `spotify.com.evil.example`, and for any other host. The helper MUST answer only the host-level question "is this URL hosted by Spotify?" and MUST NOT validate entities (track-only validation stays in `parseSpotifyURL`). The `spotify` package MUST NOT import `internal/tui`.

#### Scenario: HTTP Spotify hosts are recognized

- GIVEN `https://open.spotify.com/track/{id}`, `https://open.spotify.com/playlist/{id}`, `https://open.spotify.com/album/{id}`, `https://open.spotify.com/artist/{id}`, and `https://www.spotify.com/...`
- WHEN `IsSpotifyURL` is called for each
- THEN each MUST return `true`

#### Scenario: spotify: URIs of any entity form are recognized

- GIVEN `spotify:track:{id}`, `spotify:playlist:{id}`, `spotify:album:{id}`, and `spotify:artist:{id}`
- WHEN `IsSpotifyURL` is called for each
- THEN each MUST return `true`

#### Scenario: non-Spotify and lookalike hosts are rejected

- GIVEN `https://music.youtube.com/watch?v=...`, `https://youtube.com/watch?v=...`, `https://evilspotify.com/track/x`, and `https://spotify.com.evil.example/track/x`
- WHEN `IsSpotifyURL` is called for each
- THEN each MUST return `false`

#### Scenario: empty and whitespace-only input is rejected

- GIVEN `""` and `"   "`
- WHEN `IsSpotifyURL` is called for each
- THEN each MUST return `false`

---

## DOMAIN: github-workflows — FULL SPEC (New Domain)

No canonical spec exists at `openspec/specs/github-workflows/spec.md`. This domain is a new artifact: the repository currently has no `.github/workflows/` directory at all.

### Purpose

Provide a minimal, hermetic CI gate (vet + build + `go test -short ./...` on every push and pull request) so regressions like issue #19 fail the build instead of shipping silently. Deliberately minimal per the locked scope decision: no linter, no coverage gate, no caching, no matrix, no path filters.

### ADDED Requirement: ARF-004 — CI workflow runs vet, build, and hermetic tests on push and pull_request

The repository MUST include `.github/workflows/ci.yml` defining a workflow that triggers on `push` and `pull_request` (no path filters) and runs one job on `ubuntu-latest` which checks out the repository with `actions/checkout@v4`, installs Go with `actions/setup-go@v5` using `go-version-file: go.mod`, and then runs, in order: `go vet ./...`, `go build ./...`, `go test -short ./...`. The `-short` flag MUST be present so network-gated yt-dlp/Spotify integration tests skip and the workflow stays hermetic.

#### Scenario: workflow file exists at the expected path

- GIVEN the repository root
- WHEN checking for `.github/workflows/ci.yml`
- THEN the file MUST exist

#### Scenario: triggers cover push and pull_request

- GIVEN the workflow's `on:` key
- WHEN inspecting it
- THEN it MUST include `push`
- AND it MUST include `pull_request`
- AND it MUST NOT restrict either trigger with path filters

#### Scenario: job runs the three commands in order

- GIVEN the workflow's job steps
- WHEN inspecting them in order
- THEN step 1 MUST be `actions/checkout@v4`
- AND step 2 MUST be `actions/setup-go@v5` with `go-version-file: go.mod`
- AND the following steps MUST run `go vet ./...`, then `go build ./...`, then `go test -short ./...`

#### Scenario: tests are hermetic under -short

- GIVEN a machine with no network access (e.g. CI sandbox)
- WHEN the workflow's test step runs `go test -short ./...`
- THEN the network-gated integration tests MUST skip
- AND the step MUST pass without live network calls

### ADDED Requirement: ARF-005 — CI stays minimal

The workflow MUST NOT include a linter step, a coverage gate, caching, a build matrix, path filters, or any step beyond checkout, Go setup, vet, build, and the short test run.

#### Scenario: no extras in the workflow

- GIVEN the workflow file content
- WHEN inspecting it for linters, coverage, caching, matrix, and path filters
- THEN it MUST NOT reference golangci-lint (or any linter), coverage commands or thresholds, cache actions, `matrix` keys, or `paths:`/`paths-ignore:` filters

---

## NON-FUNCTIONAL Requirements

### Requirement: ARF-006 — Port and configuration stability

The change MUST NOT modify the `ports.Searcher` interface (or any port), the orchestrator/service layer, or `openspec/config.yaml`. The `audio-quality` change's files and behavior MUST be untouched. The new helper MUST NOT introduce an import cycle (`internal/adapters/spotify` MUST NOT import `internal/tui`). The downloader and the yt-dlp searcher adapter receive exactly ONE scoped change each: a `--` option terminator immediately before the URL argument in `buildArgs` (downloader) and `searchArgs` (searcher) to prevent option injection (ARF-010); no other behavior in those files changes.

#### Scenario: existing contracts compile and pass unchanged

- GIVEN the change applied
- WHEN running `go test -short ./...`
- THEN all pre-existing tests MUST pass unchanged

#### Scenario: config.yaml is untouched

- GIVEN the change applied
- WHEN comparing `openspec/config.yaml` to its pre-change content
- THEN it MUST be byte-identical

#### Scenario: no import cycle

- GIVEN the `spotify` package's imports after the change
- WHEN inspecting them
- THEN `internal/adapters/spotify` MUST NOT import `internal/tui`

### Requirement: ARF-007 — Entity-level validation stays track-only

The change MUST NOT modify `parseSpotifyURL` or `validateTrack`; `IsSpotifyURL` MUST NOT perform entity validation. Playlist, album, and artist URLs/URIs MUST continue to be rejected by the Spotify resolver with the existing "only track URLs are supported in this version" message.

#### Scenario: non-track Spotify URLs still rejected by the resolver

- GIVEN `https://open.spotify.com/playlist/{id}` or `spotify:playlist:{id}` routed to the Spotify searcher (Auto + configured, or explicit Spotify mode)
- WHEN the resolver processes it
- THEN it MUST return the existing error message "only track URLs are supported in this version"
- AND `IsSpotifyURL` MUST have returned `true` for the same input (routing is host-level)

### Requirement: ARF-008 — The gate admits no URI scheme other than spotify

The URL-mode input gate MUST NOT be relaxed for any scheme other than `spotify:`. Inputs like `itunes:track:{id}` or `tidal:track:{id}` MUST keep producing the existing "That doesn't look like a URL" inline error and MUST NOT start resolution.

#### Scenario: other schemes still blocked

- GIVEN a `Model` on `ScreenInput` with `InputText == "itunes:track:{id}"`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST remain `ScreenInput`
- AND `m.InputError` MUST contain "That doesn't look like a URL"

### Requirement: ARF-009 — Hermetic full-suite green

`go test -short ./...` MUST pass (exit 0) with the change applied, on a machine without network access; network-gated integration tests MUST skip under `-short`. This is the local test runner declared in `openspec/config.yaml` and the exact command CI runs.

#### Scenario: short test suite passes locally

- GIVEN the change applied and no network access
- WHEN running `go test -short ./...`
- THEN it MUST exit 0
- AND network-gated integration tests (e.g. the yt-dlp integration tests) MUST be skipped, not run

---

## Test Specifications

### Test: TUI searcher routing table (unit)

**File:** `internal/tui/update_test.go`

**Pattern:** `Model{}` literals with distinct non-nil sentinel searchers for `searcher` and `spotifySearcher` (nil when "not configured"); assert identity of the returned searcher. Written red-first (strict TDD): case 1 fails against the current code because the `SourceAuto` branch returns `m.spotifySearcher` for every URL.

| Case | Mode | URL | Spotify configured | Expected searcher |
| ------ | ---- | --- | ------------------ | ----------------- |
| 1 (issue #19) | Auto | `https://music.youtube.com/watch?v=...` | yes | `m.searcher` |
| 2 | Auto | `https://open.spotify.com/track/{id}` | yes | `m.spotifySearcher` |
| 3 | Auto | `spotify:track:{id}` | yes | `m.spotifySearcher` |
| 4 | Auto | `https://open.spotify.com/track/{id}` | no | `m.searcher` |
| 5 | Auto | `https://music.youtube.com/watch?v=...` | no | `m.searcher` |
| 6 | SourceYouTube | any URL (incl. Spotify) | either | `m.searcher` |
| 7a | SourceSpotify | any URL | yes | `m.spotifySearcher` |
| 7b | SourceSpotify | any URL | no | `m.searcher` |

### Test: TUI URL-mode input gate (unit)

**File:** `internal/tui/update_test.go`

| Case | Input | Expected |
| ------ | ----- | --------- |
| spotify: URI accepted | `spotify:track:{id}` + Enter | `ScreenResolving`; `InputError` MUST NOT contain "That doesn't look like a URL" |
| Other scheme still blocked | `itunes:track:{id}` + Enter | stays `ScreenInput`; `InputError` contains "That doesn't look like a URL" |
| Empty input unchanged | `""` + Enter | stays `ScreenInput`; `InputError` indicates empty URL |
| Regular URL unchanged | `https://open.spotify.com/track/{id}` + Enter | `ScreenResolving` |

### Test: IsSpotifyURL table (unit)

**File:** `internal/adapters/spotify/url_test.go`

| Input | Expected |
| ------ | --------- |
| `https://open.spotify.com/track/{id}` | `true` |
| `https://open.spotify.com/playlist/{id}` | `true` |
| `https://open.spotify.com/album/{id}` | `true` |
| `https://open.spotify.com/artist/{id}` | `true` |
| `https://www.spotify.com/...` | `true` |
| `spotify:track:{id}` | `true` |
| `spotify:playlist:{id}` | `true` |
| `spotify:album:{id}` | `true` |
| `spotify:artist:{id}` | `true` |
| `https://music.youtube.com/watch?v=...` | `false` |
| `https://youtube.com/watch?v=...` | `false` |
| `https://evilspotify.com/track/x` | `false` |
| `https://spotify.com.evil.example/track/x` | `false` |
| `""` | `false` |
| `"   "` (whitespace only) | `false` |

### Test: repository gates (entrypoint-style)

| Case | Command | Expected |
| ------ | ------- | --------- |
| Vet | `go vet ./...` | exit 0 |
| Build | `go build ./...` | exit 0 |
| Hermetic suite | `go test -short ./...` | exit 0; network-gated integration tests skipped |

### Test: CI workflow (file inspection)

| Case | Check | Expected |
| ------ | ----- | --------- |
| File exists | `.github/workflows/ci.yml` | present |
| Triggers | `on:` key | `push` and `pull_request`, no path filters |
| Runner | job `runs-on` | `ubuntu-latest` |
| Setup | steps | `actions/checkout@v4`; `actions/setup-go@v5` with `go-version-file: go.mod` |
| Commands | steps in order | `go vet ./...`, `go build ./...`, `go test -short ./...` |
| Minimality | workflow content | no linter, no coverage gate, no caching, no matrix, no path filters |

---

## No-Regression Requirements

The following existing behaviors MUST remain unchanged and are covered by the requirements above:

- Users without Spotify credentials: Auto mode resolves through yt-dlp exactly as today (ARF-001 cases 4–5, ARF-009).
- Explicit source modes: `SourceSpotify` and `SourceYouTube` keep their semantics byte-for-byte (ARF-001 cases 6–7).
- Query/search mode: `startQuerySearch` and the search-mode flow are untouched; `selectedSearcher` is only reached from URL-mode resolution.
- The Tab source-mode cycle (Auto → Spotify → YouTube → Auto) is unchanged.
- `parseSpotifyURL` and `validateTrack`: untouched; track-only entity validation and its error messages are unchanged (ARF-007).
- The `ports.Searcher` interface, orchestrator, downloader, and `openspec/config.yaml`: untouched (ARF-006).
- The `audio-quality` change: its files and behavior are untouched (ARF-006).
- The empty-input error ("Please enter a URL") and all other Input-screen key behavior (typing, backspace, Esc, Ctrl+C) are unchanged (ARF-002).

### ADDED Requirement: ARF-010 — yt-dlp option terminator before the URL

Both yt-dlp invocations MUST place a `--` option terminator as the argument immediately before the URL/input string, so pasted input that merely contains `://` and starts with `-` (e.g. `--config-location=http://evil.example/x`) is treated as a positional URL by yt-dlp, never as an option. This applies to `buildArgs` in the downloader adapter and to `searchArgs` (the argument builder used by `Searcher.Search`) in the yt-dlp searcher adapter. Rationale: the URL-aware routing change (ARF-001) sends non-Spotify `://`-containing input to the yt-dlp searcher in Auto+Spotify-configured mode; without the terminator, such input is parsed by yt-dlp as an option, which can fetch remote config files and apply arbitrary options (e.g. `--exec`) — option injection.

#### Scenario: search args place "--" immediately before the URL

- GIVEN `searchArgs("--config-location=http://evil.example/x")` (or any URL-like input)
- WHEN inspecting the returned argument slice
- THEN the element at `len(args)-2` MUST be `"--"`
- AND the element at `len(args)-1` MUST be the input string

#### Scenario: download args place "--" immediately before the URL

- GIVEN `buildArgs(media, outputDir, "")` with a non-empty `media.URL`
- WHEN inspecting the returned argument slice
- THEN the element at `len(args)-2` MUST be `"--"`
- AND the element at `len(args)-1` MUST be `media.URL`

---

## Test Specifications

### Test: yt-dlp option terminator (unit)

**File:** `internal/adapters/searcher/ytdlp_test.go` — `TestSearchArgs_OptionTerminatorBeforeURL`
**Pattern:** table-driven over `searchArgs(url)` asserting `args[len-2] == "--"` and `args[len-1] == url`, including an option-looking input (`--config-location=...`) and a dash-prefixed input (`--output`).

**File:** `internal/adapters/downloader/ytdlp_test.go` — `TestBuildArgs_OptionTerminatorBeforeURL`
**Pattern:** `buildArgs(media, outputDir, "")` asserting `args[len-2] == "--"` and `args[len-1] == media.URL`.
