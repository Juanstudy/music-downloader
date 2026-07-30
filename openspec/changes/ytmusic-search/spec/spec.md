# Specification: YouTube Music Query Search

**Change:** `ytmusic-search`
**Date:** 2026-07-30

## Purpose

Add a text-based search capability to music-dl that lets users discover YouTube Music tracks by typing a free-text query, complementing the existing URL-based resolution flow. The change introduces a new `QuerySearcher` port and adapter, a Search mode toggle in the TUI, and a non-URL suggestion in URL mode — all without modifying existing interfaces or behaviors.

---

## DOMAIN: core-ports — ADDED Requirements

Canonical at `openspec/specs/core-ports/spec.md`. These requirements are additions to the existing `Searcher`, `Downloader`, and `PreflightChecker` interfaces.

### ADDED Requirement: QuerySearcher interface with SearchByQuery

The system MUST provide a `QuerySearcher` interface for searching YouTube Music by free-text query.

```go
type QuerySearcher interface {
    SearchByQuery(ctx context.Context, query string, limit int) (SearchResult, error)
}
```

- Package MUST be `core/ports` in file `core/ports/querysearcher.go`.
- Imports MUST be only `"context"` and `"core/domain"` (and `"core/ports"` for `SearchResult`, which is in the same package).
- `SearchByQuery` MUST accept a `ctx context.Context`, `query string`, and `limit int`.
- `limit` controls the maximum number of results returned. A value ≤ 0 MUST be treated as "use default limit" (10).
- `SearchByQuery` MUST return `(SearchResult, error)` — the same `SearchResult` type used by `Searcher.Search`.
- The returned `SearchResult.Source` MUST be `"youtube-music"`.
- The returned `SearchResult.Tracks` MUST contain zero or more `domain.Media` items.

#### Scenario: valid query returns SearchResult with tracks

- GIVEN a type implementing `QuerySearcher`
- WHEN `SearchByQuery(ctx, "rock baladas 90s", 10)` is called
- THEN it MUST return a `SearchResult` with `Source == "youtube-music"`
- AND `Tracks` MUST contain zero or more `domain.Media` items
- AND each track MUST have a non-empty `URL` and `Title` when results exist

#### Scenario: empty query returns validation error

- GIVEN a type implementing `QuerySearcher`
- WHEN `SearchByQuery(ctx, "", 10)` is called
- THEN it MUST return a non-nil `error`
- AND the error SHOULD contain the message "query must not be empty" or similar

#### Scenario: limit ≤ 0 uses default of 10

- GIVEN a type implementing `QuerySearcher`
- WHEN `SearchByQuery(ctx, "test", 0)` is called
- THEN the implementation MUST treat `limit` as 10
- WHEN `SearchByQuery(ctx, "test", -5)` is called
- THEN the implementation MUST treat `limit` as 10

#### Scenario: QuerySearcher interface compliance compile-time check

- GIVEN the package `internal/core/ports`
- WHEN a compile-time assignment `var _ ports.QuerySearcher = (*adapterImpl)(nil)` is written
- THEN it MUST compile

#### Scenario: SearchByQuery accepts context cancellation

- GIVEN a type implementing `QuerySearcher`
- WHEN the provided `ctx` is cancelled before the search completes
- THEN `SearchByQuery` MUST return an error wrapping `context.Canceled`

---

## DOMAIN: adapters-querysearcher — FULL SPEC (New Domain)

No canonical spec exists at `openspec/specs/adapters-querysearcher/spec.md`. This domain is a new adapter package.

### Purpose

Implement the `QuerySearcher` port by invoking `yt-dlp --flat-playlist --dump-json --ignore-errors "ytmusicsearch:{limit} {query}"` and parsing each JSON output line via the existing `searcher.ParseLine()` function. The adapter reuses the parse logic from `internal/adapters/searcher/parse.go` without modification.

### Package Structure

```
internal/adapters/querysearcher/
├── querysearcher.go   # QuerySearcher implementation
└── querysearcher_test.go
```

### Requirement: QuerySearcher runs yt-dlp with ytmusicsearch: prefix

The system MUST provide a `QuerySearcher` implementation in package `internal/adapters/querysearcher/`.

```go
type QuerySearcher struct{}

func NewQuerySearcher() *QuerySearcher
func (s *QuerySearcher) SearchByQuery(ctx context.Context, query string, limit int) (ports.SearchResult, error)
```

#### Scenario: SearchByQuery calls yt-dlp with ytmusicsearch: prefix

- GIVEN a `QuerySearcher` instance
- WHEN `SearchByQuery(ctx, "test query", 10)` is called
- THEN the executable `yt-dlp` MUST be invoked
- AND the arguments MUST include `--flat-playlist`
- AND the arguments MUST include `--dump-json`
- AND the arguments MUST include `--ignore-errors`
- AND the arguments MUST include `--no-warnings`
- AND the last argument MUST be `"ytmusicsearch:10 test query"`
- AND the invocation MUST use `exec.CommandContext` (not shell escaping) to prevent shell injection

#### Scenario: results are parsed via ParseLine (each line → domain.Media)

- GIVEN a `QuerySearcher` instance
- WHEN yt-dlp returns valid JSON lines (one JSON object per line)
- THEN each line MUST be parsed via `searcher.ParseLine()` (imported from `internal/adapters/searcher`)
- AND non-parseable lines MUST be silently skipped
- AND successfully parsed `domain.Media` items MUST be collected into the returned `SearchResult`

#### Scenario: yt-dlp not found returns domain.Error

- GIVEN a `QuerySearcher` instance
- WHEN `yt-dlp` is not on `$PATH`
- THEN `SearchByQuery` MUST return a `domain.Error` with `Code == ErrorBinaryNotFound`

#### Scenario: yt-dlp non-zero exit returns error

- GIVEN a `QuerySearcher` instance
- WHEN `yt-dlp` exits with a non-zero status code
- THEN `SearchByQuery` MUST return a non-nil `error`
- AND the error SHOULD include stderr output when available for diagnostics

#### Scenario: context cancelled returns error wrapping context.Canceled

- GIVEN a `QuerySearcher` instance
- WHEN the context is cancelled before yt-dlp completes
- THEN `SearchByQuery` MUST return an error that wraps `context.Canceled`
- AND yt-dlp process MUST be terminated

#### Scenario: empty results from yt-dlp returns empty SearchResult (not error)

- GIVEN a `QuerySearcher` instance
- WHEN yt-dlp completes successfully with no output (zero lines)
- THEN `SearchByQuery` MUST return `(SearchResult{Tracks: nil, Source: "youtube-music"}, nil)`
- AND it MUST NOT return an error — empty results are a valid response

#### Scenario: query with special characters is properly escaped

- GIVEN a `QuerySearcher` instance
- WHEN `SearchByQuery(ctx, "rock & roll! \"best of\"", 10)` is called
- THEN the query MUST be passed as-is to `exec.CommandContext`
- AND shell injection MUST NOT be possible (no `sh -c`, `cmd := exec.CommandContext(ctx, "yt-dlp", args...)`)
- AND `yt-dlp` MUST receive the literal query string

#### Scenario: limit is formatted correctly as non-breaking prefix

- GIVEN a `QuerySearcher` instance
- WHEN `SearchByQuery(ctx, "jazz", 5)` is called
- THEN the yt-dlp search argument MUST be `"ytmusicsearch:5 jazz"`
- WHEN `SearchByQuery(ctx, "jazz", 0)` is called (default)
- THEN the yt-dlp search argument MUST be `"ytmusicsearch:10 jazz"`

#### Scenario: SearchResult.Source is set to "youtube-music"

- GIVEN a `QuerySearcher` instance
- WHEN `SearchByQuery` completes successfully
- THEN `SearchResult.Source` MUST be `"youtube-music"`
- AND all returned tracks SHOULD parse their individual `Source` from the JSON per `ParseLine` behavior

---

## DOMAIN: internal-tui — ADDED Requirements

Canonical at `openspec/specs/internal-tui/spec.md`. These requirements are additions to the existing 5-screen TUI.

### ADDED Requirement: Search mode distinct from URL mode

The system MUST add a `searchMode` field to the `Model` struct that is distinct from the existing `sourceMode`.

```go
// SearchMode toggles the TUI between URL input and search input.
type SearchMode int

const (
    SearchModeURL SearchMode = iota   // default — paste a URL
    SearchModeQuery                   // type a free-text search query
)
```

#### Scenario: Model gains SearchMode field

- GIVEN the `Model` struct
- WHEN a new `Model` is created via `NewModel()`
- THEN `m.searchMode` MUST be `SearchModeURL` (default)
- AND `m.searchMode` MUST be a separate field from `m.sourceMode`

#### Scenario: SearchMode is independent from SourceMode

- GIVEN a `Model` instance
- WHEN the user toggles `sourceMode` (via Tab)
- THEN `searchMode` MUST be unaffected
- WHEN the user toggles `searchMode` (via `s`)
- THEN `sourceMode` MUST be unaffected

#### Scenario: NewModel accepts QuerySearcher parameter

- GIVEN `NewModel` is called
- WHEN inspecting its signature
- THEN it MUST accept an additional `querySearcher ports.QuerySearcher` parameter
- AND the `Model` struct MUST store it as `querySearcher ports.QuerySearcher`
- AND the `Model` MUST also keep the existing `searcher` and `spotifySearcher` fields unchanged

#### Scenario: Existing NewModel callers outside this change still compile

- GIVEN existing test code that calls `NewModel(orch, searcher, spotifySearcher, outputDir)`
- WHEN the signature changes to `NewModel(orch, searcher, spotifySearcher, querySearcher, outputDir)`
- THEN all existing callers MUST be updated to pass `nil` or a real `QuerySearcher`
- AND all existing tests MUST be updated to compile

### ADDED Requirement: Keybinding `s` toggles between URL and Search mode

The system MUST bind the `s` key to toggle between `SearchModeURL` and `SearchModeQuery`.

#### Scenario: pressing `s` toggles mode with visual indicator

- GIVEN a `Model` on `ScreenInput` with `searchMode == SearchModeURL`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}`
- THEN `m.searchMode` MUST transition to `SearchModeQuery`
- AND the view MUST reflect the new mode (placeholder text and mode indicator)
- WHEN the same key is pressed again
- THEN `m.searchMode` MUST return to `SearchModeURL`
- AND the view MUST restore the original URL-mode appearance

#### Scenario: placeholder text changes in Search mode

- GIVEN a `Model` with `searchMode == SearchModeQuery`
- WHEN `View()` renders the input screen
- THEN the prompt line MUST display `"Search YouTube Music: "` instead of `"Paste a YouTube or YouTube Music URL:"`
- AND the input widget's placeholder MUST be `"search query..."` instead of `"https://music.youtube.com/..."`

#### Scenario: mode indicator shows current mode

- GIVEN a `Model` with `searchMode == SearchModeQuery`
- WHEN `View()` renders the input screen
- THEN a visual indicator MUST show the current mode, e.g. `"Mode: Search"` or `"Search YouTube Music"`
- WHEN `searchMode == SearchModeURL`
- THEN the indicator MUST show `"Mode: URL"` or similar
- AND the existing `Source` indicator MUST remain visible

#### Scenario: footer/help shows search toggle key

- GIVEN a `Model` on `ScreenInput`
- WHEN `View()` renders the footer
- THEN the footer MUST include a keybinding entry `"s"` describing the search toggle
- AND the keybinding MUST appear regardless of which mode is active

### ADDED Requirement: Enter in Search mode triggers SearchByQuery instead of Search

The system MUST route the Enter key to different commands based on `searchMode`.

#### Scenario: Enter in URL mode triggers existing resolution flow

- GIVEN a `Model` on `ScreenInput` with `searchMode == SearchModeURL`
- WHEN Enter is pressed with a non-empty value
- THEN the existing resolution flow MUST be used (`selectedSearcher().Search()`)
- AND behavior MUST be identical to the pre-change behavior

#### Scenario: Enter in Search mode triggers SearchByQuery

- GIVEN a `Model` on `ScreenInput` with `searchMode == SearchModeQuery`
- WHEN Enter is pressed with a non-empty query
- THEN the system MUST call `m.querySearcher.SearchByQuery(ctx, query, 10)`
- AND the screen MUST transition to `ScreenResolving`
- AND a spinner MUST be shown during resolution
- AND the resolving view MUST display "Searching YouTube Music..." instead of "Resolving URL..."

#### Scenario: empty query in Search mode shows validation error

- GIVEN a `Model` on `ScreenInput` with `searchMode == SearchModeQuery`
- WHEN Enter is pressed with an empty query (or whitespace-only)
- THEN the screen MUST remain on `ScreenInput`
- AND `m.inputErr` MUST be set to "Please enter a search query" or similar

#### Scenario: search results are sent via the same resolveFinishedMsg

- GIVEN a `Model` on `ScreenResolving` initiated from Search mode
- WHEN `SearchByQuery` completes
- THEN the result MUST be sent as `resolveFinishedMsg{tracks: result.Tracks, err: err}`
- AND the existing `handleResolveDone()` MUST handle it with no changes needed
- AND the playlist/download flow MUST be identical to the URL resolution flow

#### Scenario: search error returns to input with error message

- GIVEN a `Model` on `ScreenResolving` initiated from Search mode
- WHEN `SearchByQuery` returns an error with zero tracks
- THEN `m.Screen` MUST return to `ScreenInput`
- AND `m.resolveErr` MUST contain the error message

### ADDED Requirement: toggling mode mid-search clears input and results

The system MUST reset the input field and any previous results when switching modes.

#### Scenario: toggling mode clears input text

- GIVEN a `Model` with `searchMode == SearchModeURL` and non-empty input text
- WHEN `s` is pressed to switch to `SearchModeQuery`
- THEN the input text MUST be cleared
- AND any displayed error (`inputErr` or `resolveErr`) MUST be cleared
- WHEN switching back to `SearchModeURL`
- THEN the input text MUST also be cleared

#### Scenario: toggling mode on playlist screen goes to input

- GIVEN a `Model` on `ScreenPlaylist` with tracks displayed
- WHEN `s` is pressed
- THEN `m.Screen` MUST transition to `ScreenInput`
- AND `m.tracks` MUST be cleared
- AND `m.searchMode` MUST toggle
- AND `m.Input.SetValue("")` MUST be called

### ADDED Requirement: Non-URL suggestion in URL mode

When the user presses Enter in URL mode with text that does not look like a URL, the system MUST show a suggestion to switch to Search mode instead of attempting resolution.

#### Scenario: Enter in URL mode with non-URL text shows suggestion line

- GIVEN a `Model` on `ScreenInput` with `searchMode == SearchModeURL`
- WHEN Enter is pressed with a value that does not look like a URL (no `http://`, `https://`, or `ytdl://` prefix and no `.` in the host part)
- AND the value is not empty
- THEN `m.Screen` MUST remain `ScreenInput`
- AND `m.inputErr` MUST be set to `"That doesn't look like a URL. Press 's' to switch to Search mode."`
- AND no resolution attempt MUST be made

#### Scenario: suggestion disappears when user continues typing

- GIVEN a `Model` with `m.inputErr` set from a non-URL suggestion
- WHEN the user types any character in the input field
- THEN `m.inputErr` MUST be cleared
- AND the suggestion MUST no longer be displayed

#### Scenario: suggestion disappears when user switches mode

- GIVEN a `Model` with `m.inputErr` set from a non-URL suggestion
- WHEN the user presses `s` to toggle search mode
- THEN `m.inputErr` MUST be cleared
- AND the suggestion MUST no longer be displayed

#### Scenario: Enter in URL mode with valid URL resolves normally

- GIVEN a `Model` on `ScreenInput` with `searchMode == SearchModeURL`
- WHEN Enter is pressed with a value starting with `https://`, `http://`, or a known domain pattern
- THEN the existing resolution flow MUST be used
- AND no non-URL suggestion MUST be shown

#### Scenario: "looks like a URL" check is permissive

- GIVEN a `Model` on `ScreenInput` with `searchMode == SearchModeURL`
- WHEN Enter is pressed with a value containing `://` or starting with a known domain
- THEN it MUST be treated as a URL and passed to the searcher
- WHEN Enter is pressed with plain text like `"hello world"` or `"test"`
- THEN it MUST be treated as non-URL and show the suggestion

### ADDED Requirement: Search Results Display

Search results MUST be displayed using the same playlist screen as URL resolution results, with the same selection and download behavior.

#### Scenario: search results displayed as list with checkboxes

- GIVEN a `Model` on `ScreenPlaylist` with tracks from a search
- WHEN `View()` renders the playlist
- THEN tracks MUST be displayed as a selectable list
- AND each track MUST show a checkbox indicator (pending/done)
- AND the same playlist keybindings MUST work (j/k/navigation, Space/toggle, a/all, n/none, Enter/download)

#### Scenario: empty search results shows message

- GIVEN a `Model` on `ScreenResolving` initiated from Search mode
- WHEN `SearchByQuery` returns zero tracks with no error
- THEN `m.Screen` MUST return to `ScreenInput`
- AND `m.resolveErr` MUST contain `"No results found for '<query>'"`

#### Scenario: search error shows error message and returns to input

- GIVEN a `Model` on `ScreenResolving` initiated from Search mode
- WHEN `SearchByQuery` returns an error
- THEN `m.Screen` MUST return to `ScreenInput`
- AND `m.resolveErr` MUST contain the error description

#### Scenario: single search result auto-downloads

- GIVEN a `Model` on `ScreenResolving` initiated from Search mode
- WHEN `SearchByQuery` returns exactly one track
- THEN the existing single-track auto-download logic MUST apply
- AND the screen MUST transition to `ScreenDownloading`

---

## DOMAIN: cmd-entrypoint — ADDED Requirements

Canonical at `openspec/specs/cmd-entrypoint/spec.md`.

### ADDED Requirement: QuerySearcher is wired in main.go

The system MUST instantiate the `querysearcher` adapter and pass it to the TUI model in `main()`.

#### Scenario: main constructs QuerySearcher and passes to TUI

- GIVEN `main()` in `cmd/music-dl/main.go`
- WHEN the DI section executes
- THEN a new `querysearcher.NewQuerySearcher()` MUST be created
- AND it MUST be passed as the 4th argument to `tui.NewModel(orch, searcherImpl, spotifySearcher, querySearcherImpl, outputDir)`
- AND the import `"github.com/Juanstudy/music-downloader/internal/adapters/querysearcher"` MUST be added

#### Scenario: QuerySearcher import does not break compilation

- GIVEN `cmd/music-dl/main.go` with the querysearcher adapter imported
- WHEN compiled with `go build ./cmd/music-dl/`
- THEN it MUST succeed

#### Scenario: QuerySearcher is available but doesn't affect existing flow

- GIVEN the TUI starts with a wired `querySearcher`
- WHEN the user does not press `s` (stays in URL mode)
- THEN the `querySearcher` MUST never be called
- AND all existing flows MUST remain unchanged

---

## DOMAIN: Edge Cases & Error Handling

### REQUIREMENT: Empty query validation

The system MUST validate that the search query is not empty before invoking yt-dlp.

- GIVEN a `Model` on `ScreenInput` with `searchMode == SearchModeQuery`
- WHEN Enter is pressed with an empty or whitespace-only query
- THEN no yt-dlp invocation MUST be made
- AND `m.inputErr` MUST be set to "Please enter a search query"

### REQUIREMENT: Network error handling

The system MUST handle errors from yt-dlp (network failures, timeouts) gracefully.

- GIVEN a `Model` initiating a search query
- WHEN yt-dlp fails due to a network error
- THEN the error MUST be returned from `SearchByQuery`
- AND the TUI MUST display the error message and return to the input screen
- AND the user MUST be able to retry by pressing Enter again

### REQUIREMENT: Very long query handling

The system MUST NOT impose an explicit query length limit beyond what yt-dlp imposes internally.

- GIVEN a `QuerySearcher` instance
- WHEN `SearchByQuery(ctx, query, 10)` is called with a very long query (e.g., >1000 characters)
- THEN the query MUST be passed to yt-dlp as-is
- AND yt-dlp MUST handle any length limitations internally

### REQUIREMENT: Rapid repeated searches are independent

The system MUST allow the user to start a new search while a previous search is in-flight by toggling mode or pressing Enter again.

- GIVEN a `Model` on `ScreenResolving` from a search
- WHEN the user presses `s` to toggle back to URL mode
- THEN the screen MUST return to `ScreenInput`
- AND any previous in-flight search goroutine MAY continue but its result MUST be discarded (checked via Screen state)
- WHEN the user presses Esc on `ScreenResolving`
- THEN the same behavior applies — return to `ScreenInput`, discard pending results

### REQUIREMENT: Results with missing metadata

The system MUST handle search results with partial metadata gracefully via the existing `ParseLine` behavior.

- GIVEN a `QuerySearcher` instance
- WHEN yt-dlp returns JSON lines with missing `channel`/`uploader` or missing `duration`
- THEN `ParseLine` MUST produce a `domain.Media` with empty Artist or zero Duration
- AND the track MUST NOT be skipped
- AND ALL existing `ParseLine` behaviors (tested in adapters-searcher spec) MUST apply

---

## DOMAIN: No-Regression Requirements

### REQUIREMENT: Existing Searcher interface unchanged

The `Searcher` interface in `core/ports/searcher.go` MUST NOT be modified.

#### Scenario: Searcher.Search still works

- GIVEN the existing `Searcher` interface
- WHEN any code calls `Searcher.Search(ctx, url)`
- THEN it MUST behave identically to pre-change behavior
- AND all existing tests MUST pass

### REQUIREMENT: Existing adapters/searcher package unchanged

The `internal/adapters/searcher/` package MUST NOT be modified. `ParseLine()` is reused, not modified.

#### Scenario: searcher adapter tests pass

- GIVEN the `internal/adapters/searcher/` package
- WHEN running `go test ./internal/adapters/searcher/...`
- THEN all existing tests MUST pass without changes

### REQUIREMENT: URL mode behavior unchanged

All URL-mode TUI behavior MUST remain identical to the pre-change state.

#### Scenario: Enter with valid URL resolves normally

- GIVEN a `Model` with `searchMode == SearchModeURL` (default)
- WHEN a valid YouTube/YouTube Music URL is entered and Enter is pressed
- THEN the exact same resolution flow as before MUST execute
- AND the screen transitions MUST be identical

### REQUIREMENT: Download flow unchanged

The download screen, download logic, and orchestrator commands MUST NOT be modified.

#### Scenario: playlist selection and download works identically

- GIVEN tracks displayed in `ScreenPlaylist` (whether from URL or search)
- WHEN the user selects tracks and presses Enter
- THEN the download flow MUST be identical regardless of source
- AND the orchestrator API MUST be called the same way

### REQUIREMENT: All existing tests pass

The change MUST NOT break any existing tests.

#### Scenario: full test suite passes

- GIVEN the full `go test -race ./...` suite
- WHEN run with the change applied
- THEN ALL tests MUST pass (including preflight, searcher, downloader, TUI, service, domain, ports, spotify)

---

## Test Specifications

### Test: QuerySearcher port interface (unit)

**File:** `internal/core/ports/querysearcher_test.go`

| Case | Assertion |
|------|-----------|
| `QuerySearcher` is an interface with `SearchByQuery` | Compile-time check via `var _ ports.QuerySearcher = (*stubQuerySearcher)(nil)` |
| `SearchResult` zero-value is usable with QuerySearcher | Same `SearchResult` type, already tested in `searcher_test.go` |

### Test: querysearcher adapter (unit + integration)

**File:** `internal/adapters/querysearcher/querysearcher_test.go`

**Unit tests (no external dependencies, use exec.Command mocking via custom binary path or interfaces):**

| Case | Input | Expected |
| ------ | ------- | ---------- |
| Empty query returns error | `SearchByQuery(ctx, "", 10)` | Error returned, no yt-dlp invocation |
| Whitespace-only query returns error | `SearchByQuery(ctx, "   ", 10)` | Error returned, no yt-dlp invocation |
| Limit ≤ 0 defaults to 10 | `SearchByQuery(ctx, "test", 0)` | yt-dlp called with `ytmusicsearch:10 test` |
| Positive limit passed through | `SearchByQuery(ctx, "test", 5)` | yt-dlp called with `ytmusicsearch:5 test` |

**Integration tests (require real yt-dlp on `$PATH` — skip with `testing.Short()`):**

| Case | Input | Expected |
| ------ | ------- | ---------- |
| Valid query returns results | `SearchByQuery(ctx, "test", 5)` | Returns ≥0 tracks, no error |
| Empty results (improbable query) | `SearchByQuery(ctx, "xyznonexistent12345", 5)` | Returns empty SearchResult (not error) |
| Special characters | `SearchByQuery(ctx, "rock & roll!", 5)` | Returns results, no shell errors |
| Context cancellation | Cancel context before yt-dlp completes | Error wrapping `context.Canceled` |
| Very long query | `SearchByQuery(ctx, longQuery, 5)` | No crash, returns results or empty |

### Test: TUI Search mode (unit)

**File:** `internal/tui/update_test.go` (additions)

| Case | Initial State | Msg | Expected |
| ------ | -------------- | ----- | ---------- |
| `s` toggles to Search mode | `ScreenInput`, `searchMode=SearchModeURL` | Key `s` | `searchMode=SearchModeQuery`, input cleared |
| `s` toggles back to URL mode | `ScreenInput`, `searchMode=SearchModeQuery` | Key `s` | `searchMode=SearchModeURL`, input cleared |
| Enter in Search mode triggers query search | `ScreenInput`, `searchMode=SearchModeQuery`, query="test" | Enter | Transition to `ScreenResolving`, cmd returned |
| Empty query in Search mode shows error | `ScreenInput`, `searchMode=SearchModeQuery`, query="" | Enter | Stay on `ScreenInput`, `inputErr` set |
| non-URL in URL mode shows suggestion | `ScreenInput`, `searchMode=SearchModeURL`, text="hello world" | Enter | Stay on `ScreenInput`, `inputErr` set to suggestion |
| valid URL in URL mode resolves normally | `ScreenInput`, `searchMode=SearchModeURL`, url="<https://youtube.com/watch?v=xyz>" | Enter | Transition to `ScreenResolving` |
| typing clears suggestion | `ScreenInput`, `searchMode=SearchModeURL`, `inputErr` set from non-URL | Key `a` | `inputErr` cleared |
| toggling mode clears suggestion | `ScreenInput`, `searchMode=SearchModeURL`, `inputErr` set from non-URL | Key `s` | `inputErr` cleared, `searchMode=SearchModeQuery` |
| placeholder changes in Search mode view | `searchMode=SearchModeQuery` | View() | Contains "Search YouTube Music: " |
| footer shows `s` keybinding | `ScreenInput` | View() | Footer includes key for search toggle |
| search results go through playlist screen | Results from search resolution | `resolveFinishedMsg{...}` | Same `handleResolveDone` flow |
| mode toggling on playlist goes to input | `ScreenPlaylist`, tracks present | Key `s` | `ScreenInput`, tracks cleared, mode toggled |

### Test: TUI search results display (unit)

**File:** `internal/tui/view_test.go` (additions if file exists, otherwise inline assertions in update tests)

| Case | Expected |
| ------ | ---------- |
| Search results shown as selectable list with checkboxes | Same playlist rendering as URL results |
| Empty search results shows "No results found" | `resolveErr` contains message, screen returns to input |
| Search error shows error message | `resolveErr` contains error, screen returns to input |

### Test: Wiring (compilation)

| Case | Command | Expected |
| ------ | --------- | ---------- |
| Build succeeds with new adapter | `go build ./cmd/music-dl/` | Exit 0 |
| All packages compile | `go build ./...` | Exit 0 |
| Vet passes | `go vet ./...` | Exit 0 |
| Full test suite passes | `go test -race ./...` | Exit 0 |

### Test: Interface compliance (compile-time)

**File:** `internal/adapters/querysearcher/querysearcher_test.go`

| Case | Assertion |
|------|-----------|
| `querysearcher.QuerySearcher` satisfies `ports.QuerySearcher` | `var _ ports.QuerySearcher = (*QuerySearcher)(nil)` compiles |

---

## MODIFIED Requirements

### MODIFIED Requirement: Package layout follows Bubble Tea screen patterns (internal-tui)

(Full updated requirement text — existing files augmented)

The `internal/tui` package MUST add new fields to the `Model` struct:

```go
type Model struct {
    // ... existing fields ...

    // Query search (added)
    querySearcher ports.QuerySearcher
    searchMode    SearchMode
}
```

And new files:

| File | Responsibility |
| ------ | ---------------- |
| `model.go` | ADD `SearchMode` type, `SearchModeURL`/`SearchModeQuery` constants, `querySearcher` and `searchMode` fields in Model |
| `update.go` | ADD `s` key handling in input screen, non-URL detection logic, search query routing |
| `view.go` | ADD search mode placeholder, mode indicator, footer keybinding |

(Previously: TUI had 5 screens, `sourceMode`, no `querySearcher`, no `searchMode`)

## REMOVED Requirements

None — this change is purely additive and modifies no existing behavior.
