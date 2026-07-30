# Design: YouTube Music Query Search (ytmusic-search)

**Status:** Draft  
**Date:** 2026-07-30  
**Change:** `ytmusic-search`  
**Applies to:** Go 1.26 + Bubble Tea TUI music-downloader

---

## 1. Architecture Overview

### 1.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           cmd/music-dl/main.go                      │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                    internal/tui/ (Bubble Tea)                │   │
│  │                                                              │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐   │   │
│  │  │ model.go │  │ update.go│  │ view.go  │  │ messages.go│   │   │
│  │  │          │  │          │  │          │  │            │   │   │
│  │  │ SearchMode│  │ s key    │  │ mode     │  │ reuse      │   │   │
│  │  │ queryS……r│  │ non-URL  │  │ indicator│  │ resolve…Msg│   │   │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬─────┘   │   │
│  │       │              │              │               │         │   │
│  │       └──────────────┴──────────────┴───────────────┘         │   │
│  │                          │                                    │   │
│  └──────────────────────────┼────────────────────────────────────┘   │
│                             │                                        │
│  ┌──────────────────────────┼────────────────────────────────────┐   │
│  │           internal/core/ports/                               │   │
│  │                                                              │   │
│  │  ┌──────────────────┐    ┌──────────────────────────────┐    │   │
│  │  │ searcher.go      │    │ querysearcher.go  (NEW)      │    │   │
│  │  │ Searcher interface│    │ QuerySearcher interface     │    │   │
│  │  │ Search(ctx, url)  │    │ SearchByQuery(ctx,q,limit)  │    │   │
│  │  └──────────────────┘    └──────────┬───────────────────┘    │   │
│  │                                     │                         │   │
│  └─────────────────────────────────────┼─────────────────────────┘   │
│                                        │                              │
│  ┌─────────────────────────────────────┼────────────────────────┐    │
│  │          internal/adapters/         │                        │    │
│  │                                     │                        │    │
│  │  ┌──────────────────┐   ┌───────────┴────────────┐          │    │
│  │  │ searcher/         │   │ querysearcher/  (NEW)  │          │    │
│  │  │ ytdlp.go          │   │ querysearcher.go       │          │    │
│  │  │ parse.go (SHARED) │◄──│ uses ParseLine()       │          │    │
│  │  └──────────────────┘   └────────────────────────┘          │    │
│  └──────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 Data Flow: Search Mode

```
User types "rock baladas 90s" in Search mode
         │
         ▼
  handleInputKeys()
         │
         ├─ m.searchMode == SearchModeQuery?
         │   YES → m.startQuerySearch("rock baladas 90s")
         │            │
         │            ├─ Screen = ScreenResolving
         │            ├─ Input.Blur()
         │            └─ return searchResolveCmd(m.querySearcher, query, 10)
         │                  │
         │                  ▼  (goroutine)
         │            querySearcher.SearchByQuery(ctx, "rock baladas 90s", 10)
         │                  │
         │                  ├─ exec.CommandContext(ctx, "yt-dlp",
         │                  │   "--flat-playlist", "--dump-json",
         │                  │   "--ignore-errors", "--no-warnings",
         │                  │   "ytmusicsearch:10 rock baladas 90s")
         │                  │
         │                  ├─ stdout → bufio.Scanner → ParseLine() each line
         │                  │
         │                  └─ resolveFinishedMsg{tracks, err}
         │                        │
         ▼                        ▼
  handleResolveDone(msg) ←────────┘
         │
         ├─ 0 tracks + err → ScreenInput + resolveErr
         ├─ 1 track → ScreenDownloading + auto-download
         └─ N tracks → ScreenPlaylist (same as URL mode)
```

### 1.3 Component Responsibilities

| Component | Responsibility |
| ----------- | --------------- |
| `ports.QuerySearcher` | Defines the contract for query-based search (new) |
| `querysearcher.QuerySearcher` | Implements `QuerySearcher` via `yt-dlp ytmusicsearch:` |
| `searcher.ParseLine` | Reused JSON-line parser (unchanged) |
| `Model.searchMode` | Tracks whether TUI is in URL or Search mode (new) |
| `Model.querySearcher` | Injected `QuerySearcher` dependency (new) |
| `handleInputKeys` | Routes `s` toggle, non-URL detection, search Enter (modified) |
| `handlePlaylistKeys` | Routes `s` to clear and go back to input (modified) |
| `renderInputView` | Dynamic placeholder, mode indicator (modified) |
| `renderFooter` | Shows `s` keybinding on input screen (modified) |
| `renderResolvingView` | Shows "Searching YouTube Music…" in search mode (modified) |
| `main.go` | Wires `querysearcher.NewQuerySearcher()` into TUI (modified) |

---

## 2. Interface / Type Definitions

### 2.1 New Port: `ports.QuerySearcher`

**File:** `internal/core/ports/querysearcher.go`

```go
package ports

import (
    "context"
)

// QuerySearcher searches YouTube Music by free-text query.
type QuerySearcher interface {
    // SearchByQuery searches YouTube Music for the given query and returns up to
    // limit results. If limit <= 0, a default of 10 is used.
    // Returns the same SearchResult type used by Searcher.Search with
    // Source set to "youtube-music".
    SearchByQuery(ctx context.Context, query string, limit int) (SearchResult, error)
}
```

**No new types.** Reuses `ports.SearchResult` and `domain.Media`.

### 2.2 New Adapter: `querysearcher.QuerySearcher`

**File:** `internal/adapters/querysearcher/querysearcher.go`

```go
package querysearcher

import (
    "bufio"
    "context"
    "fmt"
    "os/exec"
    "strings"

    "github.com/Juanstudy/music-downloader/internal/adapters/searcher"
    "github.com/Juanstudy/music-downloader/internal/core/domain"
    "github.com/Juanstudy/music-downloader/internal/core/ports"
)

type QuerySearcher struct {
    binary string
}

func NewQuerySearcher() *QuerySearcher {
    return &QuerySearcher{binary: "yt-dlp"}
}

func (s *QuerySearcher) SearchByQuery(ctx context.Context, query string, limit int) (ports.SearchResult, error) {
    if strings.TrimSpace(query) == "" {
        return ports.SearchResult{}, domain.Error{
            Code:    domain.ErrorInvalidURL,
            Message: "query must not be empty",
        }
    }

    if limit <= 0 {
        limit = 10
    }

    searchArg := fmt.Sprintf("ytmusicsearch:%d %s", limit, query)
    args := []string{
        "--flat-playlist",
        "--dump-json",
        "--ignore-errors",
        "--no-warnings",
        searchArg,
    }

    cmd := exec.CommandContext(ctx, s.binary, args...)
    // ... stdout pipe, scanner, ParseLine loop, error handling ...
}
```

**Exact `exec.Command` invocation:**

```
yt-dlp --flat-playlist --dump-json --ignore-errors --no-warnings "ytmusicsearch:10 rock baladas 90s"
```

**Key design decisions:**

- Reuses `searcher.ParseLine()` by importing `"github.com/Juanstudy/music-downloader/internal/adapters/searcher"` directly. ParseLine is a package-level function, so `searcher.ParseLine(line)` works without any change to the searcher package.
- Error handling mirrors `searcher.Searcher.Search()` exactly: stdout pipe, scanner with large buffer, ParseLine per line, stderr capture, partial results handling.
- Empty query returns `domain.Error{Code: ErrorInvalidURL}` — validated before any yt-dlp invocation.
- `Source` is hardcoded to `"youtube-music"` (not inferred from URL).

### 2.3 New TUI Types

**File:** `internal/tui/model.go`

```go
// SearchMode toggles the TUI between URL input and search input.
type SearchMode int

const (
    SearchModeURL SearchMode = iota // default — paste a URL
    SearchModeQuery                 // type a free-text search query
)
```

**New fields on `Model`:**

```go
type Model struct {
    // ... existing fields ...

    // Query search (added)
    querySearcher ports.QuerySearcher
    searchMode    SearchMode
}
```

### 2.4 Function Signature Changes

```go
// Before:
func NewModel(orch *service.Orchestrator, youtubeSearcher, spotifySearcher ports.Searcher, outputDir string) Model

// After:
func NewModel(orch *service.Orchestrator, youtubeSearcher, spotifySearcher ports.Searcher, querySearcher ports.QuerySearcher, outputDir string) Model
```

`querySearcher` is placed before `outputDir` and after the existing searchers — grouped with the other port dependencies.

---

## 3. File-by-File Change Plan

### 3.1 NEW: `internal/core/ports/querysearcher.go`

**What:** New port interface.  
**Reason:** Clean separation — `Searcher` resolves URLs, `QuerySearcher` searches by text. Keeps existing interface unchanged.  
**Content:** `QuerySearcher` interface with `SearchByQuery(ctx, query string, limit int) (SearchResult, error)`.  

### 3.2 NEW: `internal/core/ports/querysearcher_test.go`

**What:** Compile-time interface compliance check.  
**Content:** `var _ ports.QuerySearcher = (*stubQuerySearcher)(nil)` ensures the interface is properly defined.

### 3.3 NEW: `internal/adapters/querysearcher/querysearcher.go`

**What:** Full adapter implementation.  
**Content:**

- `QuerySearcher` struct with `binary string` field (defaults to `"yt-dlp"`, overridable for testing)
- `NewQuerySearcher()` constructor
- `SearchByQuery()` method with exact exec.Command invocation described above
- Empty query validation before yt-dlp
- Limit ≤ 0 → default to 10
- Import and call `searcher.ParseLine(line)` for each JSON output line
- Error handling identical to existing `searcher.Searcher.Search()`

### 3.4 NEW: `internal/adapters/querysearcher/querysearcher_test.go`

**What:** Unit + integration tests.  
**Content:** See §5 Testing Strategy.

### 3.5 MODIFIED: `internal/tui/model.go`

**Changes:**

1. Add `SearchMode` type + `SearchModeURL`/`SearchModeQuery` constants (before `Model` struct).
2. Add `querySearcher ports.QuerySearcher` field to `Model`.
3. Add `searchMode SearchMode` field to `Model`.
4. Change `NewModel` signature to accept `querySearcher ports.QuerySearcher` as 4th parameter.
5. Initialize `querySearcher` and `searchMode: SearchModeURL` in `NewModel`.
6. Change input placeholder when `searchMode == SearchModeQuery`:
   - URL mode: `"https://music.youtube.com/..."` (unchanged)
   - Search mode: `"search query..."` (new)
7. **No changes** to `Screen` enum, `SourceMode` enum, or any other existing types.

### 3.6 MODIFIED: `internal/tui/update.go`

**Changes (additive — no existing code paths modified):**

#### 3.6.1 `handleInputKeys` — `s` key interception

Add before the existing `msg.Type` switch:

```go
case tea.KeyMsg:                    // ← opening brace already exists
    // NEW: search mode toggle — intercept before input widget
    if msg.String() == "s" {
        return m.toggleSearchMode()
    }
    // ... existing switch(msg.Type) ...
```

#### 3.6.2 `handleInputKeys` — Enter handler for URL vs Search mode

Modify the `tea.KeyEnter` case:

```go
case tea.KeyEnter:
    val := strings.TrimSpace(m.Input.Value())
    m.inputErr = ""
    if val == "" {
        m.inputErr = "Please enter a URL"
        return m, nil
    }

    if m.searchMode == SearchModeQuery {
        return m.startQuerySearch(val)
    }

    // URL mode: check if it looks like a URL
    if !strings.Contains(val, "://") {
        m.inputErr = "That doesn't look like a URL. Press 's' to switch to Search mode."
        return m, nil
    }
    return m.startResolve(val)
```

#### 3.6.3 New method: `toggleSearchMode()`

```go
func (m Model) toggleSearchMode() (tea.Model, tea.Cmd) {
    // Toggle mode
    if m.searchMode == SearchModeURL {
        m.searchMode = SearchModeQuery
    } else {
        m.searchMode = SearchModeURL
    }
    // Clear input and errors
    m.Input.SetValue("")
    m.inputErr = ""
    m.resolveErr = ""
    // If on playlist/resolving screen, go back to input
    if m.Screen != ScreenInput {
        m.tracks = nil
        m.cursor = 0
        m.scroll = 0
        m.Screen = ScreenInput
        m.PrevScreen = ScreenInput // or prev
        m.Input.Focus()
    }
    return m, nil
}
```

#### 3.6.4 New method: `startQuerySearch()`

```go
func (m Model) startQuerySearch(query string) (tea.Model, tea.Cmd) {
    if strings.TrimSpace(query) == "" {
        m.inputErr = "Please enter a search query"
        return m, nil
    }
    m.Screen = ScreenResolving
    m.PrevScreen = ScreenInput
    m.Input.Blur()
    m.InputID++
    return m, searchResolveCmd(m.querySearcher, query, 10)
}
```

#### 3.6.5 New function: `searchResolveCmd()`

```go
func searchResolveCmd(qs ports.QuerySearcher, query string, limit int) tea.Cmd {
    return func() tea.Msg {
        result, err := qs.SearchByQuery(context.Background(), query, limit)
        if err != nil {
            return resolveFinishedMsg{tracks: result.Tracks, err: err}
        }
        return resolveFinishedMsg{tracks: result.Tracks, err: nil}
    }
}
```

#### 3.6.6 `handlePlaylistKeys` — add `s` handler

In the `msg.String()` switch:

```go
case "s":
    // Toggle search mode, go back to input
    m.Screen = ScreenInput
    m.tracks = nil
    m.cursor = 0
    m.scroll = 0
    m.filter = ""
    m.resolveErr = ""
    m.Input.SetValue("")
    if m.searchMode == SearchModeURL {
        m.searchMode = SearchModeQuery
    } else {
        m.searchMode = SearchModeURL
    }
    m.Input.Focus()
    return m, nil
```

#### 3.6.7 `handleResolvingKeys` — add `s` handler

```go
case "s":
    m.Screen = ScreenInput
    m.tracks = nil
    m.Input.SetValue("")
    m.Input.Focus()
    if m.searchMode == SearchModeURL {
        m.searchMode = SearchModeQuery
    } else {
        m.searchMode = SearchModeURL
    }
    return m, nil
```

**No changes** to `handleResolveDone()`, `startDownload()`, `handleTrackDone()`, `handleDownloadingKeys()`, or `handleDoneKeys()` — search results flow through the same message and handler.

### 3.7 MODIFIED: `internal/tui/view.go`

**Changes:**

#### 3.7.1 `renderInputView` — dynamic prompt and mode indicator

Current:

```go
b.WriteString("Paste a YouTube or YouTube Music URL:\n\n")
b.WriteString(inputStyle.Render(m.Input.View()))
```

Change to:

```go
if m.searchMode == SearchModeQuery {
    b.WriteString("Search YouTube Music:\n\n")
    // Temporarily swap placeholder for rendering
    originalPlaceholder := m.Input.Placeholder
    m.Input.Placeholder = "search query..."
    m.Input.Width = m.Input.Width // keep width stable
    b.WriteString(inputStyle.Render(m.Input.View()))
    m.Input.Placeholder = originalPlaceholder
} else {
    b.WriteString("Paste a YouTube or YouTube Music URL:\n\n")
    b.WriteString(inputStyle.Render(m.Input.View()))
}
```

Or more elegantly — set the placeholder from `Model` state directly without swapping. Since `View()` is called after every Update, we can set `m.Input.Placeholder` in `Update()` when mode changes. But that couples update and view logic. The placeholder swap in View() is pragmatic and self-contained.

**Preferable approach:** Set `m.Input.Placeholder` inside `toggleSearchMode()` and inside `NewModel()` — cleaner separation.

```go
func (m Model) toggleSearchMode() (tea.Model, tea.Cmd) {
    if m.searchMode == SearchModeURL {
        m.searchMode = SearchModeQuery
        m.Input.Placeholder = "search query..."
    } else {
        m.searchMode = SearchModeURL
        m.Input.Placeholder = "https://music.youtube.com/..."
    }
    // ...
}
```

#### 3.7.2 `renderInputView` — mode indicator line

Add after the source mode indicator:

```go
b.WriteString("\n")
b.WriteString(mutedStyle.Render("Search: ") + m.renderSearchMode())

func (m Model) renderSearchMode() string {
    if m.searchMode == SearchModeQuery {
        return emphStyle.Render("Search") + mutedStyle.Render(" (s to switch)")
    }
    return emphStyle.Render("URL")
}
```

#### 3.7.3 `renderResolvingView` — dynamic resolving message

Current:

```go
if m.sourceMode == SourceSpotify {
    b.WriteString(" Resolving via Spotify...\n\n")
} else {
    b.WriteString(" Resolving URL...\n\n")
}
```

Change to:

```go
if m.sourceMode == SourceSpotify {
    b.WriteString(" Resolving via Spotify...\n\n")
} else if m.searchMode == SearchModeQuery {
    b.WriteString(" Searching YouTube Music...\n\n")
} else {
    b.WriteString(" Resolving URL...\n\n")
}
```

#### 3.7.4 `renderInputView` — non-URL suggestion styling

The existing `m.resolveErr` is already rendered below the input. For non-URL detection, we use `m.inputErr` (which is the same field shown for empty URL validation). However, looking at `renderInputView`:

Current:

```go
if m.resolveErr != "" {
    b.WriteString("\n")
    b.WriteString(errorStyle.Render("✗ " + m.resolveErr))
    b.WriteString("\n")
}
```

The non-URL suggestion should go here too. But `m.inputErr` is only set (not rendered) currently. Let me check...

Looking at `renderInputView`, it renders `m.resolveErr`, not `m.inputErr`. The `m.inputErr` is set but not rendered in the input view. Wait, let me re-read:

```go
func (m Model) renderInputView() string {
    // ...
    if m.resolveErr != "" {
        b.WriteString("\n")
        b.WriteString(errorStyle.Render("✗ " + m.resolveErr))
        b.WriteString("\n")
    }
    // ...
}
```

But `m.resolveErr` is populated by `handleResolveDone` when resolution fails. The `m.inputErr` field isn't rendered anywhere visible — but it IS used for the empty URL check. Looking at the update tests:

```go
func TestEnterEmptyURLShowsError(t *testing.T) {
    // ...
    if updated.inputErr == "" {
        t.Error("expected non-empty inputErr for empty URL")
    }
}
```

So `inputErr` is checked in tests but not rendered in view.go. That means we need to render `inputErr` (or change the non-URL suggestion to use `resolveErr`).

**Decision:** Render `m.inputErr` in `renderInputView`, below the input and before `m.resolveErr`:

```go
if m.inputErr != "" {
    b.WriteString("\n")
    b.WriteString(errorStyle.Render("✗ " + m.inputErr))
    b.WriteString("\n")
}
```

This way both "Please enter a URL" and "That doesn't look like a URL..." are shown.

#### 3.7.5 `renderFooter` — add `s` keybinding on input screen

In the `if m.Screen == ScreenInput` block, add:

```go
keys = append(keys, keyStyle.Render("s")+" "+keyDescStyle.Render("search"))
```

#### 3.7.6 `renderInputView` — placeholder update on mode change

We also need to set the placeholder when the model is built. In `NewModel`, the initial placeholder should remain `"https://music.youtube.com/..."` because `searchMode` defaults to `SearchModeURL`.

### 3.8 MODIFIED: `internal/tui/messages.go`

**No changes.** Search results reuse `resolveFinishedMsg` — the same struct drives URL and search result handling. `handleResolveDone()` already handles both single-track (→ download) and multi-track (→ playlist) flows identically.

### 3.9 MODIFIED: `internal/tui/keys.go`

**No structural changes.** The `helpContent` slice can include a search binding note for completeness in the help overlay:

```go
{"s", "Toggle search mode"},
```

### 3.10 MODIFIED: `cmd/music-dl/main.go`

**Changes:**

1. Add import: `"github.com/Juanstudy/music-downloader/internal/adapters/querysearcher"`
2. After `searcherImpl := searcher.NewSearcher()`, add:

   ```go
   querySearcherImpl := querysearcher.NewQuerySearcher()
   ```

3. Change `NewModel` call:

   ```go
   // Before:
   m := tui.NewModel(orch, searcherImpl, spotifySearcher, outputDir)
   // After:
   m := tui.NewModel(orch, searcherImpl, spotifySearcher, querySearcherImpl, outputDir)
   ```

### 3.11 MODIFIED: `internal/tui/update_test.go`

Add new test cases (see §5 Testing Strategy).

---

## 4. Data Flow Details

### 4.1 Search Mode Toggle (`s` key)

```
User presses 's' on ScreenInput with searchMode=URL
         │
         ▼
  toggleSearchMode()
         │
         ├─ m.searchMode = SearchModeQuery
         ├─ m.Input.Placeholder = "search query..."
         ├─ m.Input.SetValue("")
         ├─ m.inputErr = ""
         ├─ m.resolveErr = ""
         └─ return m, nil
```

### 4.2 Non-URL Detection Flow

```
User types "hello world" + Enter on URL mode
         │
         ▼
  handleInputKeys → KeyEnter
         │
         ├─ val = "hello world"
         ├─ m.searchMode == SearchModeURL → yes
         ├─ strings.Contains(val, "://") → false
         ├─ m.inputErr = "That doesn't look like a URL..."
         └─ return m, nil  (no yt-dlp invocation)
```

### 4.3 Search Query Flow

```
User types "rock baladas 90s" + Enter on Search mode
         │
         ▼
  handleInputKeys → KeyEnter
         │
         ├─ val = "rock baladas 90s"
         ├─ m.searchMode == SearchModeQuery → yes
         └─ m.startQuerySearch("rock baladas 90s")
               │
               ├─ ScreenResolving
               ├─ return searchResolveCmd(querySearcher, "rock baladas 90s", 10)
               │      │
               │      ▼  (goroutine)
               │   querySearcher.SearchByQuery(ctx, "rock baladas 90s", 10)
               │      │
               │      ├─ yt-dlp --flat-playlist --dump-json --ignore-errors
               │      │          --no-warnings "ytmusicsearch:10 rock baladas 90s"
               │      │
               │      ├─ ParseLine each stdout line → []domain.Media
               │      │
               │      └─ resolveFinishedMsg{tracks, err}
               │            │
               ▼            ▼
         handleResolveDone(msg) ← same handler as URL mode
               │
               ├─ single track → auto-download
               ├─ multi track → playlist screen
               └─ error/empty → input screen
```

### 4.4 Mode Independence

`searchMode` and `sourceMode` are completely orthogonal:

| `searchMode` | `sourceMode` | Behavior |
| ------------- | ------------- | ---------- |
| `URL` | `Auto` | Paste URL → auto-detect backend (existing) |
| `URL` | `YouTube` | Paste URL → force YouTube (existing) |
| `URL` | `Spotify` | Paste URL → force Spotify (existing) |
| `Query` | any | Type query → YouTube Music search (new) |

When `searchMode == SearchModeQuery`, `sourceMode` is displayed but ignored for the search — the query always searches YouTube Music. This is by design: there is no "Spotify search" in the first slice.

---

## 5. Testing Strategy

### 5.1 Port Compile-Time Checks

**File:** `internal/core/ports/querysearcher_test.go`

```go
package ports

import (
    "context"
    "testing"
)

type stubQuerySearcher struct{}

func (s *stubQuerySearcher) SearchByQuery(ctx context.Context, query string, limit int) (SearchResult, error) {
    return SearchResult{}, nil
}

func TestQuerySearcherInterface(t *testing.T) {
    var _ QuerySearcher = (*stubQuerySearcher)(nil)
}
```

### 5.2 Adapter Unit Tests

**File:** `internal/adapters/querysearcher/querysearcher_test.go`

| Test | Input | Expected |
| ------ | ------- | ---------- |
| Empty query returns error | `SearchByQuery(ctx, "", 10)` | Error returned, no yt-dlp invocation |
| Whitespace-only returns error | `SearchByQuery(ctx, "   ", 10)` | Error returned, no yt-dlp invocation |
| Limit ≤ 0 uses default 10 | `SearchByQuery(ctx, "test", 0)` | Limit defaults to 10 (tested via argument inspection if we make binary overridable, or via integration) |
| Limit 5 passed through | `SearchByQuery(ctx, "test", 5)` | Limit set to 5 |
| Compile-time compliance | `var _ ports.QuerySearcher = (*QuerySearcher)(nil)` | Compiles |

**For argument inspection without integration:** The `binary` field is a `string`, not overridable by default. We should make it a struct field that tests can set:

```go
type QuerySearcher struct {
    binary string
}
```

For unit-testing the argument construction without invoking yt-dlp, we have two options:

1. **Export the args builder** as a method: Not idiomatic — exposes internals.
2. **Make `commandContext` a func field** (dependency injection for exec.Command): Over-engineered for this codebase (existing searcher doesn't do it).
3. **Integration-only testing** with `testing.Short()`: Matches existing pattern in the codebase.

**Decision:** Follow the existing codebase pattern — unit tests for validation logic (empty query, whitespace, limit normalization) without invoking yt-dlp, and integration tests with `testing.Short()` guard for actual yt-dlp calls. The validation tests test `SearchByQuery` but check that errors are returned BEFORE any cmd execution (since empty query validation happens first). For limit normalization, we test integration behavior.

### 5.3 Adapter Integration Tests

**File:** `internal/adapters/querysearcher/querysearcher_test.go`

| Test | Input | Expected |
| ------ | ------- | ---------- |
| Valid query returns results | `SearchByQuery(ctx, "test", 5)` | Returns ≥0 tracks, no error |
| Empty results (improbable query) | `SearchByQuery(ctx, "xyznonexistent12345", 5)` | Returns empty SearchResult (not error) |
| Special characters | `SearchByQuery(ctx, "rock & roll!", 5)` | Returns results, no shell errors |
| Context cancellation | Cancel context before yt-dlp completes | Error wrapping `context.Canceled` |
| Very long query | `SearchByQuery(ctx, longQuery, 5)` | No crash, returns results or empty |

Pattern:

```go
func TestQuerySearcher_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test (requires yt-dlp)")
    }
    s := NewQuerySearcher()
    result, err := s.SearchByQuery(context.Background(), "test", 5)
    if err != nil {
        t.Fatalf("SearchByQuery failed: %v", err)
    }
    if result.Source != "youtube-music" {
        t.Errorf("expected source 'youtube-music', got %q", result.Source)
    }
}
```

### 5.4 TUI Unit Tests

**File:** `internal/tui/update_test.go`

**New stub:**

```go
type stubQuerySearcher struct {
    result ports.SearchResult
    err    error
}

func (s *stubQuerySearcher) SearchByQuery(ctx context.Context, query string, limit int) (ports.SearchResult, error) {
    return s.result, s.err
}
```

**New test cases (additive, all existing tests unchanged):**

| Test | Case Description |
| ------ | ----------------- |
| `TestSearchMode_ToggleOnInput` | `s` on input with URL mode → `searchMode=SearchModeQuery`, input cleared |
| `TestSearchMode_ToggleOnPlaylist` | `s` on playlist → screen=input, tracks cleared, mode toggled |
| `TestSearchMode_ToggleOnResolving` | `s` on resolving → screen=input, mode toggled |
| `TestSearchMode_ToggleTwice` | `s` twice returns to URL mode |
| `TestSearchMode_EnterTriggersSearch` | Enter in search mode → `ScreenResolving`, cmd returned |
| `TestSearchMode_EmptyQuery` | Enter with empty query in search mode → `inputErr`, stay on input |
| `TestURLMode_NonURLSuggestion` | Enter with plain text in URL mode → `inputErr` with suggestion, stay on input |
| `TestURLMode_ValidURLStillResolves` | Enter with valid URL in URL mode → normal resolve flow |
| `TestURLMode_SuggestionClearsOnTyping` | Typing clears `inputErr` |
| `TestSearchMode_SearchResultsFlow` | Search results via `resolveFinishedMsg` → same playlist flow |

**Test helpers needed:**

- `modelWithQuerySearcher()` — builds a Model with `querySearcher` field set
- `newInput()` already exists

### 5.5 Wiring Compilation Test

```bash
go build ./cmd/music-dl/   # must succeed
go build ./...             # must succeed
go vet ./...               # must succeed
go test -race ./...        # must pass
```

---

## 6. Key Design Decisions

### 6.1 Reuse `resolveFinishedMsg` (No New Message Types)

Search results reuse the existing `resolveFinishedMsg` struct. The `handleResolveDone()` handler already handles single-track (→ auto-download), multi-track (→ playlist), and error (→ input with error) flows. No new message types are needed.

### 6.2 `SearchMode` vs `SourceMode` Orthogonality

The two mode systems are independent:

- `SourceMode` (Auto/YouTube/Spotify) controls which *backend* resolves a URL
- `SearchMode` (URL/Query) controls what the *input* accepts

Independence means:

- Tab toggling `sourceMode` never changes `searchMode`
- `s` toggling `searchMode` never changes `sourceMode`
- In Search mode, `sourceMode` is displayed but unused (searches always go to YouTube Music)

### 6.3 No Orchestrator Changes

The search bypasses the orchestrator — the TUI calls `querySearcher.SearchByQuery()` directly, not via `orchestrator.SearchByQuery()`. This is consistent with how the existing TUI calls `selectedSearcher().Search()` directly. The orchestrator only handles the resolve+download workflow, not raw search.

### 6.4 `looksLikeURL` Detection

The check is intentionally permissive: `strings.Contains(val, "://")`. Any string containing `://` is treated as a URL. This matches the spec requirement:

- `"https://youtube.com/watch?v=xyz"` → URL (contains `://`)
- `"hello world"` → non-URL (no `://`)
- `"spotify:track:abc"` → non-URL (no `://`) — but Spotify URIs use `spotify:track:`, not URLs. Users should paste the full URL from the browser.

### 6.5 Placeholder Management

Rather than swapping placeholder strings in `View()` (which is inelegant), the placeholder is set in `toggleSearchMode()`:

- `SearchModeURL` → `"https://music.youtube.com/..."`
- `SearchModeQuery` → `"search query..."`

And initially in `NewModel()` to match the default `SearchModeURL`.

---

## 7. Migration / Rollback

### 7.1 Migration (Forward)

No data migration needed — this is purely additive. Deploy order:

1. Add `ports.QuerySearcher` interface (new file, zero impact)
2. Add `querysearcher` adapter package (new package, zero impact)
3. Add `searchMode` and `querySearcher` to Model + `NewModel` signature (breaks callers)
4. Update `main.go` to wire the new adapter
5. Update TUI handlers (additive, existing flow untouched)
6. Add test cases
7. Run full test suite
8. Build and deploy

### 7.2 Rollback (Backward)

Fully backward-compatible rollback:

1. Revert TUI changes (model.go, update.go, view.go, keys.go)
2. Remove `querysearcher` adapter package
3. Remove `QuerySearcher` port interface
4. Revert `main.go` wiring
5. No existing functionality affected at any step

### 7.3 Compatibility Guarantees

| Artifact | Affected? |
| ---------- | ----------- |
| `ports.Searcher` | NO — unchanged |
| `ports.SearchResult` | NO — reused |
| `ports.Downloader` | NO — unchanged |
| `searcher.ParseLine()` | NO — imported, not modified |
| `searcher.Searcher.Search()` | NO — unchanged |
| `spotify.SpotifySearcher` | NO — unchanged |
| `service.Orchestrator` | NO — unchanged |
| `domain.Media`, `domain.Error` | NO — unchanged |
| `tui.Screen`, `tui.SourceMode` | NO — unchanged |
| TUI URL mode behavior | NO — unchanged (default mode) |
| Download flow | NO — unchanged |
| All existing test cases | NO — unchanged (new tests added) |

---

## 8. Risks and Mitigations

| Risk | Severity | Mitigation |
| ------ | ---------- | ------------ |
| `s` key conflicts with existing keybindings | Low | No existing binding for `s`; `q`/`esc`/`?` use different keys |
| `ytmusicsearch:` format changes in yt-dlp | Medium | Adapter reuses `ParseLine()` which handles JSON schema stably; format change would only affect the search argument prefix |
| TUI test complexity from modal state | Low | `toggleSearchMode()` is a single pure-mutation method; tested via table-driven key press tests |
| Input placeholder swap in View() is fragile | Low | Moved to `toggleSearchMode()` and `NewModel()` — View() only reads the placeholder, doesn't mutate it |
| Partial results from yt-dlp during search | Low | Same path as existing `handleResolveDone()` — partial tracks + warning displayed |
| Search query with shell metacharacters | None | `exec.CommandContext` does not invoke shell — no shell injection possible |

---

## Summary of Changed Files

| File | Status | Change Summary |
| ------ | -------- | --------------- |
| `internal/core/ports/querysearcher.go` | **NEW** | `QuerySearcher` interface with `SearchByQuery` |
| `internal/core/ports/querysearcher_test.go` | **NEW** | Compile-time interface compliance test |
| `internal/adapters/querysearcher/querysearcher.go` | **NEW** | Full adapter implementation — yt-dlp ytmusicsearch: |
| `internal/adapters/querysearcher/querysearcher_test.go` | **NEW** | Unit + integration tests |
| `internal/tui/model.go` | **MODIFIED** | Add `SearchMode`, `querySearcher`, `searchMode`; update `NewModel` |
| `internal/tui/update.go` | **MODIFIED** | Add `s` handler, non-URL detection, `toggleSearchMode`, `startQuerySearch`, `searchResolveCmd` |
| `internal/tui/view.go` | **MODIFIED** | Dynamic prompt/placeholder, mode indicator, search footer, resolving message |
| `internal/tui/keys.go` | **MODIFIED** | Add search mode binding to help overlay |
| `internal/tui/update_test.go` | **MODIFIED** | Add search-mode TUI tests (~10 new test cases) |
| `cmd/music-dl/main.go` | **MODIFIED** | Wire `querysearcher.NewQuerySearcher()` into `NewModel` |
