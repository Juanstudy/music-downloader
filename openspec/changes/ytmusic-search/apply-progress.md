# Apply Progress: ytmusic-search — PR1 + PR2 + PR3

**Date:** 2026-07-30
**Batches:** PR1 (Port layer, ~40 lines), PR2 (Adapter layer, ~220 lines), PR3 (TUI + Wiring, ~345 lines)

---

## PR1 Completed

### Task 1: QuerySearcher Interface + Compliance Test

**RED phase:** Wrote `querysearcher_test.go` first — failed to compile with `undefined: QuerySearcher` as expected.

**GREEN phase:** Wrote `querysearcher.go` with `QuerySearcher` interface containing `SearchByQuery(ctx, query, limit) (SearchResult, error)`.

**REFACTOR phase:** No refactoring needed — both files follow existing codebase conventions (same style as `searcher.go`, minimal imports).

**TDD Cycle Evidence:**

| Phase | Result |
| ------- | -------- |
| RED | `go test -race ./internal/core/ports/...` → `build failed: undefined: QuerySearcher` |
| GREEN | `go test -race ./internal/core/ports/...` → `ok` |
| REFACTOR | No changes needed |
| Full suite | `go build ./...` → ok, `go vet ./...` → ok, `go test -race ./...` → all packages pass |

**Files created:**

- `internal/core/ports/querysearcher.go` — `QuerySearcher` interface (~15 lines)
- `internal/core/ports/querysearcher_test.go` — compile-time compliance test with `stubQuerySearcher` (~25 lines)

---

## PR2 Completed

| Task | Status | Files |
|------|--------|-------|
| T2: Adapter implementation | ✅ Done | 1 new file |
| T3: Adapter unit + integration tests | ✅ Done | 1 new file |

### Task 2: QuerySearcher Adapter Implementation

**RED phase:** Wrote `querysearcher_test.go` in `internal/adapters/querysearcher/` first — 9 compilation errors (types didn't exist yet).

**GREEN phase:** Wrote `querysearcher.go` with:

- `QuerySearcher` struct with `binary string` field
- `NewQuerySearcher()` constructor (defaults to `"yt-dlp"`)
- `SearchByQuery(ctx, query, limit)` method following EXACT error handling pattern from `ytdlp.go`
- Empty/whitespace query → `domain.Error{Code: ErrorInvalidURL, Message: "query must not be empty"}`
- limit ≤ 0 defaults to 10
- Builds YouTube Music search URL: `https://music.youtube.com/search?q=<query>` with `--playlist-end N`
- This triggers yt-dlp's `youtube:music:search_url` extractor (more portable than `ytmusicsearch:`)
- `exec.CommandContext` with `--flat-playlist --dump-json --ignore-errors --no-warnings`
- `StdoutPipe` + `bufio.Scanner` with large buffer (1MB initial, 10MB max)
- Reuses `searcher.ParseLine()` from existing package — zero changes to parse.go
- Captures stderr via `strings.Builder`
- Partial results on `cmd.Wait()` error with stderr diagnostics
- Source always `"youtube-music"`

**REFACTOR phase:** No refactoring needed.

### Task 3: Adapter Tests

**Test cases implemented:**

- Unit: empty query, whitespace query, limit default, source verification
- Integration: valid query, context cancellation, special characters, very long query
- Compile-time compliance: `var _ ports.QuerySearcher = (*QuerySearcher)(nil)`

---

## PR3 Completed

| Task | Status | Files |
| ------ | -------- | ------- |
| T4: TUI model changes | ✅ Done | model.go |
| T5: Update handlers | ✅ Done | update.go |
| T6: View changes | ✅ Done | view.go, keys.go |
| T7: TUI update tests | ✅ Done | update_test.go |
| T8: Wiring | ✅ Done | main.go |
| T9: Build + test verification | ✅ Done | — |

### Task 4: TUI Model Changes (model.go)

- Added `SearchMode` type with `SearchModeURL`/`SearchModeQuery` constants
- Added `querySearcher ports.QuerySearcher` field to Model
- Added `searchMode SearchMode` field to Model
- Updated `NewModel` signature to accept `querySearcher ports.QuerySearcher` as 4th param
- Initialized `querySearcher` and `searchMode: SearchModeURL` in `NewModel`
- Placeholder set based on mode

### Task 5: Update Handlers (update.go)

- Added `s` key interception in `handleInputKeys` (before input widget processes it)
- Modified `tea.KeyEnter` case: search mode routes to `startQuerySearch()`, URL mode checks `://`
- Non-URL detection: if `!strings.Contains(val, "://")` → sets `inputErr` with suggestion
- Added `toggleSearchMode()` method — toggles mode, clears input/errors, returns to ScreenInput
- Added `startQuerySearch()` method — validates query, sets ScreenResolving, returns searchResolveCmd
- Added `searchResolveCmd()` function — calls `querySearcher.SearchByQuery`, returns `resolveFinishedMsg`
- Added `s` handler in `handlePlaylistKeys` — clears tracks, toggles mode, goes to input
- Added `s` handler in `handleResolvingKeys` — clears state, toggles mode, goes to input

### Task 6: View Changes (view.go + keys.go)

- Dynamic prompt text in `renderInputView` based on `searchMode`
- Search mode indicator ("Search: Search (s to switch)" / "Search: URL") in `renderInputView`
- `inputErr` rendering in `renderInputView` (was previously not displayed)
- `renderSearchMode()` helper method
- Dynamic "Searching YouTube Music..." message in `renderResolvingView`
- `s` keybinding in `renderFooter` on input screen
- Added `{"s", "Toggle search mode"}` to `helpContent` in keys.go

### Task 7: TUI Tests (update_test.go)

**New test helper:**

```go
type stubQuerySearcher struct {
    result ports.SearchResult
    err    error
}
func (s *stubQuerySearcher) SearchByQuery(ctx context.Context, query string, limit int) (ports.SearchResult, error) {
    return s.result, s.err
}
```

**10 new test cases — ALL PASS:**

| # | Test | Description |
| --- | ------ | ------------- |
| 1 | `TestSearchMode_ToggleOnInput` | `s` on input toggles to query mode, clears input |
| 2 | `TestSearchMode_ToggleOnPlaylist` | `s` on playlist goes to input, mode toggled |
| 3 | `TestSearchMode_ToggleOnResolving` | `s` on resolving goes to input |
| 4 | `TestSearchMode_ToggleTwice` | Two toggles return to URL mode |
| 5 | `TestSearchMode_EnterTriggersSearch` | Enter in search mode → ScreenResolving + cmd |
| 6 | `TestSearchMode_EmptyQuery` | Enter with empty query → inputErr |
| 7 | `TestURLMode_NonURLSuggestion` | Enter with plain text in URL mode → inputErr with suggestion |
| 8 | `TestURLMode_ValidURLStillResolves` | Enter with valid URL still resolves normally |
| 9 | `TestSearchMode_SearchResultsFlow` | resolveFinishedMsg with search results shows playlist |
| 10 | `TestSearchMode_SeamlessToggle` | Multiple toggles, state is consistent |

### Task 8: Wiring (main.go)

- Added import for `querysearcher` adapter
- Added `querySearcherImpl := querysearcher.NewQuerySearcher()` after existing searcher
- Updated `NewModel` call to pass `querySearcherImpl` as 4th argument

### Task 9: Full Build + Test Verification

| Command | Result |
| --------- | -------- |
| `go build ./...` | ✅ PASS |
| `go build ./cmd/music-dl/` | ✅ PASS |
| `go vet ./...` | ✅ PASS (no output) |
| `go test -race -short ./...` | ✅ ALL 10 packages PASS |

## TDD Cycle Evidence (PR3)

| Phase | Result |
| ------- | -------- |
| RED | Wrote 10 test cases → 48 compilation errors (SearchMode type, fields, methods missing) |
| GREEN | Wrote production code (model.go, update.go, view.go, keys.go, main.go) → all 32 tests pass (22 existing + 10 new) |
| REFACTOR | No refactoring needed — code follows existing conventions exactly |

## Files Modified (PR3)

- `internal/tui/model.go` — SearchMode type, querySearcher/searchMode fields, NewModel signature
- `internal/tui/update.go` — s key handlers, non-URL detection, toggleSearchMode, startQuerySearch, searchResolveCmd
- `internal/tui/view.go` — dynamic prompt, mode indicator, inputErr rendering, search resolving message, footer s key
- `internal/tui/keys.go` — s binding in help overlay
- `internal/tui/update_test.go` — 10 new test cases for search mode
- `cmd/music-dl/main.go` — wire querysearcher.NewQuerySearcher()

## Deviations from Design

**None.** Implementation follows the design and spec exactly.

## Remaining Tasks

All tasks complete. Ready for `verify` phase.

## Workload / PR Boundary

- **PR1 (~40 lines):** ✅ COMPLETE — Port layer
- **PR2 (~220 lines):** ✅ COMPLETE — Adapter layer
- **PR3 (~345 lines):** ✅ COMPLETE — TUI changes + wiring
