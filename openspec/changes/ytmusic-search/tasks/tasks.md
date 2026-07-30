# Tasks: YouTube Music Query Search

**Change:** `ytmusic-search`
**Date:** 2026-07-30
**Estimated total:** ~620 lines (port 40 + adapter 220 + TUI 345 + wiring 15)
**Status:** Draft

---

## Review Workload Forecast

| Field | Value |
| ------- | ------- |
| Estimated changed lines | ~620 (10 files: 4 new, 6 modified) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

```text
Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High
```

> **Recommendation:** Split into 3 chained PRs. Each PR is reviewable and autonomous.  
> **Estimated lines per PR:** PR 1 ~60 lines, PR 2 ~220 lines, PR 3 ~340 lines.

---

## Task Overview

```
Task 1: Port interface + compile-time test ........... 40 lines  (no deps)
Task 2: Adapter implementation ....................... 90 lines  (depends: T1)
Task 3: Adapter unit + integration tests ............ 130 lines  (depends: T2)
Task 4: TUI model changes (SearchMode, fields, NewModel) . 45 lines (depends: T1)
Task 5: Update handlers (s key, search flow) ........ 130 lines  (depends: T4)
Task 6: View changes (placeholders, indicators) ..... 65 lines  (depends: T4)
Task 7: TUI update tests ........................... 120 lines  (depends: T4,T5,T6)
Task 8: Wire querysearcher in main.go ............... 15 lines  (depends: T2,T4)
Task 9: Full build + test pass ......................  —        (depends: T1-T8)
```

---

## PR Split

### PR 1 — Port layer only (~40 lines)

- **File:** `openspec/changes/ytmusic-search/PR1.md` (split manifest)
- **Tasks:** T1
- **Files:** `internal/core/ports/querysearcher.go` (NEW), `internal/core/ports/querysearcher_test.go` (NEW)
- **Review scope:** Interface definition, compile-time compliance
- **Verification:** `go test -race ./internal/core/ports/...`

### PR 2 — Adapter layer (~220 lines)

- **File:** `openspec/changes/ytmusic-search/PR2.md` (split manifest)
- **Tasks:** T2, T3
- **Files:** `internal/adapters/querysearcher/querysearcher.go` (NEW), `internal/adapters/querysearcher/querysearcher_test.go` (NEW)
- **Review scope:** yt-dlp invocation correctness, error handling, ParseLine reuse
- **Verification:** `go test -race ./internal/adapters/querysearcher/...`

### PR 3 — TUI changes + wiring (~345 lines)

- **File:** `openspec/changes/ytmusic-search/PR3.md` (split manifest)
- **Tasks:** T4, T5, T6, T7, T8, T9
- **Files:** `internal/tui/model.go`, `internal/tui/update.go`, `internal/tui/view.go`, `internal/tui/keys.go`, `internal/tui/update_test.go`, `cmd/music-dl/main.go`
- **Review scope:** Search mode toggle, non-URL detection, visual indicators, NewModel wiring
- **Verification:** `go build ./cmd/music-dl/ && go vet ./... && go test -race ./...`

---

## Task 1: Add QuerySearcher port interface + compile-time test

**Files:**

- `internal/core/ports/querysearcher.go` (NEW)
- `internal/core/ports/querysearcher_test.go` (NEW)

**Approach:** TDD — write the port test first (RED stage), then the interface (GREEN stage).

**TDD Sequence:**

1. RED: Write `querysearcher_test.go` with `stubQuerySearcher` and `var _ ports.QuerySearcher = (*stubQuerySearcher)(nil)` compile-time check
2. GREEN: Write `querysearcher.go` with the `QuerySearcher` interface containing `SearchByQuery(ctx context.Context, query string, limit int) (SearchResult, error)`
3. REFACTOR: Verify imports are minimal (only `"context"`), doc comment is present, interface follows existing conventions

**Implementation Details:**

- Package: `ports` in `internal/core/ports/`
- Interface `QuerySearcher` with single method `SearchByQuery`
- Reuses `ports.SearchResult` — no new result types
- Code structure mimics existing `ports/searcher.go`:

```go
package ports

import "context"

// QuerySearcher searches YouTube Music by free-text query.
type QuerySearcher interface {
 SearchByQuery(ctx context.Context, query string, limit int) (SearchResult, error)
}
```

- Test uses a `stubQuerySearcher` struct that implements `SearchByQuery` with empty valid return
- Add `var _ ports.QuerySearcher = (*stubQuerySearcher)(nil)` compile-time assertion

**Estimated lines:** ~40 lines total (15 interface + 25 test)

**Dependencies:** None

**Acceptance Criteria:**

- [x] `internal/core/ports/querysearcher.go` exists with `QuerySearcher` interface
- [x] `internal/core/ports/querysearcher_test.go` compiles with compile-time compliance check
- [x] `go test -race ./internal/core/ports/...` passes
- [x] No new types introduced — only interface + reuse of `SearchResult`

**Risk:** Low. Pure additive, no callers yet.

**Owner:** implementation

---

## Task 2: Implement querysearcher adapter

**Files:**

- `internal/adapters/querysearcher/querysearcher.go` (NEW)

**Approach:** Write the adapter struct and `SearchByQuery` method that shells out to yt-dlp with `ytmusicsearch:` prefix.

**Implementation Details:**

- Package: `querysearcher` in `internal/adapters/querysearcher/`
- Struct `QuerySearcher` with `binary string` field (defaults to `"yt-dlp"`)
- Constructor `NewQuerySearcher() *QuerySearcher`
- `SearchByQuery` method:
  1. Validate empty/whitespace-only query → return `domain.Error{Code: domain.ErrorInvalidURL, Message: "query must not be empty"}`
  2. Normalize limit ≤ 0 → 10
  3. Build search arg: `fmt.Sprintf("ytmusicsearch:%d %s", limit, query)`
  4. `exec.CommandContext(ctx, s.binary, "--flat-playlist", "--dump-json", "--ignore-errors", "--no-warnings", searchArg)`
  5. Pipe stdout via `cmd.StdoutPipe()`, use `bufio.Scanner` with large buffer
  6. Call `searcher.ParseLine(line)` for each line (import from `internal/adapters/searcher`)
  7. Collect successfully parsed `domain.Media` items
  8. Capture stderr via `cmd.StderrPipe()` or `cmd.CombinedOutput()` for error diagnostics
  9. On `cmd.Wait()` error with zero tracks → return error with stderr content
  10. On context cancellation → return error wrapping `context.Canceled`
  11. Empty yt-dlp output → return `(ports.SearchResult{Tracks: nil, Source: "youtube-music"}, nil)`
  12. Set `SearchResult.Source` to `"youtube-music"` hardcoded

**Key imports needed:**

```go
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
```

**Error handling pattern** mirrors existing `internal/adapters/searcher/ytdlp.go`.

**Estimated lines:** ~90 lines (struct + constructor + method + error handling)

**Dependencies:** Task 1 (ports.QuerySearcher interface)

**Acceptance Criteria:**

- [x] `internal/adapters/querysearcher/querysearcher.go` compiles
- [x] `go build ./internal/adapters/querysearcher/...` passes
- [x] Empty query returns error without invoking yt-dlp
- [x] Limit 0 or negative defaults to 10
- [x] Uses `exec.CommandContext` (not shell escaping) — verified by code review
- [x] Uses `searcher.ParseLine()` from existing searcher package — no duplicate parsing
- [x] `SearchResult.Source` is always `"youtube-music"`

**Risk:** Low. Follows existing searcher adapter pattern exactly.

**Owner:** implementation

---

## Task 3: Write adapter unit + integration tests

**Files:**

- `internal/adapters/querysearcher/querysearcher_test.go` (NEW)

**Approach:** TDD — RED tests first (unit tests for validation logic without yt-dlp), then GREEN implementation, then integration tests with `testing.Short()` guard.

**TDD Sequence:**

1. RED: Write unit tests that fail (validation tests call `SearchByQuery` but error is returned before yt-dlp runs)
2. GREEN: Tests pass against Task 2 implementation
3. TRIANGULATE: Add integration tests with `testing.Short()` guard

**Unit Tests (no yt-dlp required, pure validation):**

| Test Name | Input | Expected |
| ----------- | ------- | ---------- |
| `TestEmptyQuery_ReturnsError` | `SearchByQuery(ctx, "", 10)` | Error returned, no yt-dlp |
| `TestWhitespaceQuery_ReturnsError` | `SearchByQuery(ctx, "   ", 10)` | Error returned, no yt-dlp |
| `TestCompileTime_InterfaceCompliance` | `var _ ports.QuerySearcher = (*QuerySearcher)(nil)` | Compiles |
| `TestSearchResult_SourceIsYoutubeMusic` | Call with binary override to `"echo"` + args to inspect `"ytmusicsearch:10 test"` | Source is `"youtube-music"` |

**Integration Tests (require yt-dlp on $PATH — guard with `testing.Short()`):**

| Test Name | Input | Expected |
| ----------- | ------- | ---------- |
| `TestSearch_ValidQuery_ReturnsResults` | `SearchByQuery(ctx, "test", 5)` | ≥0 tracks, no error |
| `TestSearch_EmptyResults_NotError` | `SearchByQuery(ctx, "xyznonexistent12345", 5)` | Empty SearchResult, nil error |
| `TestSearch_SpecialCharacters` | `SearchByQuery(ctx, "rock & roll!", 5)` | Returns results, no shell errors |
| `TestSearch_ContextCancellation` | Cancel context mid-flight | Error wrapping `context.Canceled` |
| `TestSearch_VeryLongQuery` | >1000 char query | No crash, returns or empty |

**Pattern for unit tests (using binary override for arg inspection):**

```go
// QuerySearcher with overridable binary for testing
type QuerySearcher struct {
    binary string
}
// Test can construct: &QuerySearcher{binary: "echo"} to inspect args
```

**Estimated lines:** ~130 lines

**Dependencies:** Task 2 (adapter implementation)

**Acceptance Criteria:**

- [x] All unit tests pass with `go test -short ./internal/adapters/querysearcher/...`
- [x] All integration tests pass with `go test ./internal/adapters/querysearcher/...` (when yt-dlp present)
- [x] Interface compliance compile-time check present
- [x] Empty query test verifies no yt-dlp invocation
- [x] Context cancellation test wraps `context.Canceled`
- [x] Integration tests use `testing.Short()` skip guard

**Risk:** Low-medium (integration tests depend on yt-dlp availability)

**Owner:** implementation

---

## Task 4: Add SearchMode type, model fields, update NewModel signature

**Files:**

- `internal/tui/model.go` (MODIFIED)

**Approach:** TDD — write the failing test in update_test.go first (Task 7 will expand), then implement model changes here.

**Changes to `model.go`:**

1. Add `SearchMode` type with constants before `Model` struct:

   ```go
   type SearchMode int

   const (
       SearchModeURL SearchMode = iota
       SearchModeQuery
   )
   ```

2. Add two new fields to `Model`:

   ```go
   querySearcher ports.QuerySearcher
   searchMode    SearchMode
   ```

3. Change `NewModel` signature:

   ```go
   // Before:
   func NewModel(orch *service.Orchestrator, youtubeSearcher, spotifySearcher ports.Searcher, outputDir string) Model
   // After:
   func NewModel(orch *service.Orchestrator, youtubeSearcher, spotifySearcher ports.Searcher, querySearcher ports.QuerySearcher, outputDir string) Model
   ```

4. Initialize `querySearcher` and `searchMode: SearchModeURL` in `NewModel`
5. Set initial placeholder in `NewModel`:

   ```go
   ti.Placeholder = "https://music.youtube.com/..." // stays as-is for default URL mode
   ```

6. Add `getDefaultPlaceholder()` helper:

   ```go
   func getDefaultPlaceholder(mode SearchMode) string {
       if mode == SearchModeQuery {
           return "search query..."
       }
       return "https://music.youtube.com/..."
   }
   ```

**Estimated lines:** ~45 lines

**Dependencies:** Task 1 (ports.QuerySearcher type)

**Risks:**

- `NewModel` signature change breaks all existing callers → must update: `main.go` (Task 8) and all test helpers that construct `Model` directly (test models use struct literal, not `NewModel`, so they don't break — but any `NewModel` call in tests must be updated)

**Acceptance Criteria:**

- [ ] `SearchMode` type + `SearchModeURL` / `SearchModeQuery` constants exist
- [ ] `Model` has `querySearcher ports.QuerySearcher` and `searchMode SearchMode` fields
- [ ] `NewModel` accepts `querySearcher ports.QuerySearcher` parameter (4th arg)
- [ ] `go build ./internal/tui/...` compiles after changes
- [ ] Default `searchMode` is `SearchModeURL`
- [ ] `searchMode` is independent from `sourceMode` (separate fields, no cross-coupling)

**Owner:** implementation

---

## Task 5: Implement search mode toggle and query search in update.go

**Files:**

- `internal/tui/update.go` (MODIFIED)

**Approach:** TDD with Task 7 tests. Add minimum code to make tests pass, then refactor.

**Changes:**

### 5.1 Add `s` key handler in `handleInputKeys`

Insert before the `switch msg.Type` block in `handleInputKeys`:

```go
case tea.KeyMsg:
    // Search mode toggle
    if msg.String() == "s" {
        return m.toggleSearchMode()
    }
    switch msg.Type {
    // ... existing switch ...
```

### 5.2 Modify `KeyEnter` case for URL vs Search mode

In the `case tea.KeyEnter:` block, after empty validation:

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
    // URL mode: non-URL detection
    if !strings.Contains(val, "://") {
        m.inputErr = "That doesn't look like a URL. Press 's' to switch to Search mode."
        return m, nil
    }
    return m.startResolve(val)
```

### 5.3 Add `toggleSearchMode()` method

```go
func (m Model) toggleSearchMode() (tea.Model, tea.Cmd) {
    if m.searchMode == SearchModeURL {
        m.searchMode = SearchModeQuery
        m.Input.Placeholder = "search query..."
    } else {
        m.searchMode = SearchModeURL
        m.Input.Placeholder = "https://music.youtube.com/..."
    }
    m.Input.SetValue("")
    m.inputErr = ""
    m.resolveErr = ""
    // If on playlist or resolving screen, go back to input
    if m.Screen != ScreenInput {
        m.tracks = nil
        m.cursor = 0
        m.scroll = 0
        m.Screen = ScreenInput
        m.PrevScreen = ScreenInput
        m.Input.Focus()
    }
    return m, nil
}
```

### 5.4 Add `startQuerySearch()` method

```go
func (m Model) startQuerySearch(query string) (tea.Model, tea.Cmd) {
    m.Screen = ScreenResolving
    m.PrevScreen = ScreenInput
    m.Input.Blur()
    m.InputID++
    return m, searchResolveCmd(m.querySearcher, query, 10)
}
```

### 5.5 Add `searchResolveCmd()` function

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

### 5.6 Add `s` handler in `handlePlaylistKeys`

In the `msg.String()` switch, add:

```go
case "s":
    m.Screen = ScreenInput
    m.tracks = nil
    m.cursor = 0
    m.scroll = 0
    m.filter = ""
    m.resolveErr = ""
    m.Input.SetValue("")
    m.Input.Placeholder = getDefaultPlaceholder(
        SearchModeQuery - m.searchMode + SearchModeURL, // toggle
    )
    if m.searchMode == SearchModeURL {
        m.searchMode = SearchModeQuery
    } else {
        m.searchMode = SearchModeURL
    }
    m.Input.Focus()
    return m, nil
```

Or simpler — just call `toggleSearchMode()` from playlist too.

### 5.7 Add `s` handler in `handleResolvingKeys`

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

Or again — call `toggleSearchMode()`.

**Cleaner approach:** Reuse `toggleSearchMode()` everywhere. It already handles the Screen check, clearing, and mode toggle.

### 5.8 Clear inputErr on any key press (for suggestion disappearance)

In `handleInputKeys`, before the existing key handling, if `m.inputErr != ""` and the key is not Enter and not `s`, clear it:

```go
case tea.KeyMsg:
    if msg.String() == "s" {
        return m.toggleSearchMode()
    }
    // Clear inputErr on any key press (suggestion disappears when typing)
    m.inputErr = ""
    switch msg.Type {
```

**Estimated lines:** ~130 lines

**Dependencies:** Task 4 (model changes)

**Risks:**

- `s` key interception must happen BEFORE input widget processes it (race condition)
- Toggle mid-search must properly discard pending results (Screen state check)
- Non-URL detection uses simple `contains("://")` — spec says this is intentional

**Acceptance Criteria:**

- [ ] `s` toggles between `SearchModeURL` and `SearchModeQuery`
- [ ] Toggling clears input text and errors
- [ ] Toggling on playlist/resolving screen goes back to input
- [ ] Enter in Search mode calls `querySearcher.SearchByQuery`
- [ ] Empty query in Search mode shows validation error
- [ ] Non-URL in URL mode shows suggestion message
- [ ] Valid URL in URL mode resolves normally (no regression)
- [ ] Typing clears the suggestion
- [ ] `go build ./internal/tui/...` passes

**Owner:** implementation

---

## Task 6: Update views for search mode (placeholder, indicators, footer, resolving message)

**Files:**

- `internal/tui/view.go` (MODIFIED)
- `internal/tui/keys.go` (MODIFIED)

**Approach:** Start with a RED view test (if applicable), then implement view changes to support the search mode state set by Task 5.

**Changes to `view.go`:**

### 6.1 Dynamic prompt in `renderInputView`

Replace hardcoded prompt:

```go
// Before:
b.WriteString("Paste a YouTube or YouTube Music URL:\n\n")

// After:
if m.searchMode == SearchModeQuery {
    b.WriteString("Search YouTube Music:\n\n")
} else {
    b.WriteString("Paste a YouTube or YouTube Music URL:\n\n")
}
```

### 6.2 Mode indicator in `renderInputView`

Add after source mode indicator:

```go
b.WriteString(mutedStyle.Render("Search: ") + m.renderSearchMode())
```

Add new method:

```go
func (m Model) renderSearchMode() string {
    if m.searchMode == SearchModeQuery {
        return emphStyle.Render("Search") + mutedStyle.Render(" (s to switch)")
    }
    return emphStyle.Render("URL")
}
```

### 6.3 Render `inputErr` in `renderInputView`

Add before footer:

```go
if m.inputErr != "" {
    b.WriteString("\n")
    b.WriteString(errorStyle.Render("✗ " + m.inputErr))
    b.WriteString("\n")
}
```

### 6.4 Dynamic resolving message in `renderResolvingView`

Change from:

```go
if m.sourceMode == SourceSpotify {
    b.WriteString(" Resolving via Spotify...\n\n")
} else {
    b.WriteString(" Resolving URL...\n\n")
}
```

To:

```go
if m.sourceMode == SourceSpotify {
    b.WriteString(" Resolving via Spotify...\n\n")
} else if m.searchMode == SearchModeQuery {
    b.WriteString(" Searching YouTube Music...\n\n")
} else {
    b.WriteString(" Resolving URL...\n\n")
}
```

### 6.5 Add `s` keybinding to footer in `renderFooter`

In the `if m.Screen == ScreenInput` block:

```go
if m.Screen == ScreenInput {
    keys = append(keys,
        keyStyle.Render("Enter")+" "+keyDescStyle.Render("resolve"),
        keyStyle.Render("s")+" "+keyDescStyle.Render("search"),
        keyStyle.Render("Tab")+" "+keyDescStyle.Render("source"),
    )
}
```

### 6.6 Add search binding to `keys.go`

Add to `helpContent` slice:

```go
{"s", "Toggle search mode"},
```

**Estimated lines:** ~65 lines total (60 view.go + 5 keys.go)

**Dependencies:** Task 4 (model changes for `SearchMode`, `inputErr`, `searchMode`)

**Risks:**

- Placeholder must sync with actual mode (set in `toggleSearchMode()`)
- Footers must not duplicate `s` binding across multiple screens

**Acceptance Criteria:**

- [ ] Search mode shows `"Search YouTube Music:"` prompt
- [ ] URL mode shows existing `"Paste a YouTube or YouTube Music URL:"` prompt
- [ ] Mode indicator shows `"Search: URL"` or `"Search: Search (s to switch)"`
- [ ] `inputErr` is rendered below the input (non-URL suggestion visible)
- [ ] Resolving screen shows `"Searching YouTube Music..."` for search mode
- [ ] Footer on input screen shows `s` keybinding
- [ ] Help overlay in `keys.go` includes search binding
- [ ] `go build ./internal/tui/...` passes

**Owner:** implementation

---

## Task 7: Write TUI update tests for search mode

**Files:**

- `internal/tui/update_test.go` (MODIFIED)

**Approach:** TDD — write RED tests that define expected behavior, then verify they pass when all TUI changes (Tasks 4-6) are complete.

**New test helpers:**

```go
type stubQuerySearcher struct {
    result ports.SearchResult
    err    error
}

func (s *stubQuerySearcher) SearchByQuery(ctx context.Context, query string, limit int) (ports.SearchResult, error) {
    return s.result, s.err
}
```

**New test cases (additive — all existing 22 tests unchanged):**

| # | Test Name | Initial State | Msg | Expected |
| --- | ----------- | --------------- | ----- | ---------- |
| 23 | `TestSearchMode_ToggleOnInput` | ScreenInput, searchMode=URL | `KeyRunes('s')` | searchMode=Query, input cleared |
| 24 | `TestSearchMode_ToggleOnPlaylist` | ScreenPlaylist with tracks | `KeyRunes('s')` | ScreenInput, tracks cleared, mode toggled |
| 25 | `TestSearchMode_ToggleOnResolving` | ScreenResolving | `KeyRunes('s')` | ScreenInput, mode toggled |
| 26 | `TestSearchMode_ToggleTwice` | ScreenInput, searchMode=URL | `s` then `s` | searchMode=URL (back to original) |
| 27 | `TestSearchMode_EnterTriggersSearch` | ScreenInput, searchMode=Query, Input="test" | KeyEnter | ScreenResolving, non-nil cmd |
| 28 | `TestSearchMode_EmptyQueryShowsError` | ScreenInput, searchMode=Query, Input="" | KeyEnter | Stay on input, inputErr set |
| 29 | `TestURLMode_NonURLShowsSuggestion` | ScreenInput, searchMode=URL, Input="hello world" | KeyEnter | Stay on input, inputErr="That doesn't look like a URL..." |
| 30 | `TestURLMode_ValidURLStillResolves` | ScreenInput, searchMode=URL, Input="<https://youtube.com/>..." | KeyEnter | ScreenResolving, non-nil cmd |
| 31 | `TestSearchMode_InputErrClearsOnTyping` | ScreenInput, inputErr set, searchMode=URL | `KeyRunes('a')` | inputErr cleared |
| 32 | `TestSearchMode_ToggleClearsInputErr` | ScreenInput, inputErr set from non-URL | `KeyRunes('s')` | inputErr cleared, mode toggled |
| 33 | `TestSearchMode_SearchResultsFlowViaResolveDone` | Search results from search | resolveFinishedMsg | Same playlist/download flow |
| 34 | `TestSearchMode_EmptyResults` | Search returns zero tracks, nil error | resolveFinishedMsg | ScreenInput, resolveErr="No results found" |
| 35 | `TestSearchMode_SearchError` | Search returns error | resolveFinishedMsg | ScreenInput, resolveErr set |

**Note:** Test 33 tests that `handleResolveDone` handles search results identically to URL results — this verifies the reuse of `resolveFinishedMsg`.

**Estimated lines:** ~120 lines

**Dependencies:** Tasks 4, 5, 6 (model, update, view changes)

**Risks:**

- Must not break existing 22 tests — all test assertions must remain passing
- `Model` struct literal construction in existing tests doesn't break with new fields (Go zero-values)

**Acceptance Criteria:**

- [ ] All existing 22 tests still pass
- [ ] ~13 new test cases compile and pass
- [ ] `go test -race ./internal/tui/...` passes
- [ ] `stubQuerySearcher` implements `ports.QuerySearcher` correctly

**Owner:** implementation

---

## Task 8: Wire querysearcher in main.go

**Files:**

- `cmd/music-dl/main.go` (MODIFIED)

**Approach:** Add the import and instantiation, update `NewModel` call.

**Changes:**

1. Add import:

   ```go
   "github.com/Juanstudy/music-downloader/internal/adapters/querysearcher"
   ```

2. After `searcherImpl := searcher.NewSearcher()`:

   ```go
   querySearcherImpl := querysearcher.NewQuerySearcher()
   ```

3. Update `NewModel` call:

   ```go
   // Before:
   m := tui.NewModel(orch, searcherImpl, spotifySearcher, outputDir)
   // After:
   m := tui.NewModel(orch, searcherImpl, spotifySearcher, querySearcherImpl, outputDir)
   ```

**Estimated lines:** ~15 lines (+1 import, +1 instantiation, +1 argument update)

**Dependencies:** Task 2 (querysearcher.NewQuerySearcher), Task 4 (NewModel signature change)

**Risks:** None. Straightforward wiring.

**Acceptance Criteria:**

- [ ] `go build ./cmd/music-dl/` succeeds
- [ ] `go vet ./cmd/music-dl/` passes
- [ ] Binary runs without import errors
- [ ] QuerySearcher is instantiated but never called in URL mode (no behavioral change)

**Owner:** implementation

---

## Task 9: Full build and test pass

**Files:** None (verification only)

**Approach:** Run the full verification suite after all tasks are complete.

**Steps:**

1. `go build ./...` — all packages compile
2. `go vet ./...` — all packages pass vet
3. `go test -race ./...` — full test suite passes (including new tests)
4. Spot-check: `go build ./cmd/music-dl/` produces a working binary

**Verify no-regression guarantees:**

- Existing `ports.Searcher` unchanged
- Existing `searcher.ParseLine` unchanged (imported, not modified)
- Existing TUI URL mode behavior unchanged
- Existing download flow unchanged
- All existing 22 TUI tests pass

**Dependencies:** Tasks 1-8

**Acceptance Criteria:**

- [ ] `go build ./...` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `go test -race ./...` exits 0
- [ ] All existing tests pass (no regression)
- [ ] No changes to `internal/adapters/searcher/` or `internal/core/domain/`
- [ ] No changes to `Searcher` interface or `SearchResult` type
- [ ] Binary produced by `go build ./cmd/music-dl/` is runnable

**Risk:** Low — dependent on all prior tasks passing.

**Owner:** implementation

---

## Risk Assessment

| Task | Risk | Mitigation |
| ------ | ------ | ------------ |
| T1 (Port interface) | Low | Pure additive, no callers |
| T2 (Adapter) | Low | Follows existing searcher pattern exactly |
| T3 (Adapter tests) | Low-Medium | Integration tests depend on yt-dlp; unit tests cover validation |
| T4 (Model changes) | Medium | `NewModel` signature breaks callers — must update main.go and any test helpers |
| T5 (Update handlers) | Medium | `s` key interception must happen before input widget; toggle state management |
| T6 (View changes) | Low | Pure rendering changes driven by model state |
| T7 (TUI tests) | Medium | Must not break 22 existing tests; 13 new test cases |
| T8 (Wiring) | Low | Simple DI wiring |
| T9 (Build + test) | Low | Depends on all prior tasks passing |

**Overall risk:** Medium — mostly additive with clean separation, but the TUI mode toggle introduces modal state complexity. Chained PRs reduce risk by isolating the TUI changes to the last PR.

---

## Rollback Notes

Each PR is independently revertible:

- **PR 1:** Remove `internal/core/ports/querysearcher.go` and `querysearcher_test.go` — zero impact on existing code.
- **PR 2:** Remove `internal/adapters/querysearcher/` directory — zero impact on existing code.
- **PR 3:** Revert changes to `model.go`, `update.go`, `view.go`, `keys.go`, `update_test.go`, `main.go` — existing URL mode is untouched during the change, so rollback is safe.

No existing functionality is affected at any intermediate state. Each PR compiles and tests pass independently.
