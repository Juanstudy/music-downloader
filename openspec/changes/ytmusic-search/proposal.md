# Proposal: YouTube Music Query Search

**Change:** `ytmusic-search`
**Status:** Proposed
**Date:** 2026-07-30

---

## 1. Problem / Opportunity

The current music-dl TUI only accepts URLs. Users paste a YouTube, YouTube Music, or Spotify URL and the app resolves it into tracks. There is no way to *discover* music by typing a text query — users must already know the exact URL of a video, playlist, or album.

Many common listening scenarios start with an idea, not a URL: "I want to listen to rock baladas from the 90s", "find that song that goes 'never gonna give you up'", or "show me popular jazz piano covers". Without query search, the app cannot serve these discovery-driven workflows.

Adding a dedicated **Search mode** turns music-dl into a music discovery tool, not just a URL resolver.

## 2. Target Users & Situations

| User | Situation |
| ------ | ----------- |
| Casual listener | Wants to find and download music by genre, mood, or era without knowing specific URLs |
| Power user | Knows the track name but not the exact YouTube URL; types it directly |
| Playlist discoverer | Wants to search YouTube Music for a concept playlist (e.g. "best of 80s pop") and download individual tracks |
| Current user | Lands on the input screen, types a non-URL text, gets no useful feedback today |

## 3. Current-State Gap

- The `Searcher` port (`core/ports/searcher.go`) has a single method: `Search(ctx, url string)` — it expects a URL.
- The `parseLine` function in `internal/adapters/searcher/parse.go` already handles yt-dlp JSON output perfectly, but there is no entry point for a free-text query.
- The input screen always renders a URL placeholder: `"Paste a YouTube or YouTube Music URL:"`
- If a user types text instead of a URL, the existing `Searcher` is invoked with that text as a URL — yt-dlp will likely fail or return nothing useful.
- The Spotify adapter already uses `ytsearch:` as a *proof of concept* (see `internal/adapters/spotify/resolve.go`), but this is hidden inside the Spotify resolution flow and not exposed as a user-facing search feature.

## 4. Proposed Solution

### 4.1 New Port: `QuerySearcher`

A new interface in `core/ports/`:

```go
type QuerySearcher interface {
    SearchByQuery(ctx context.Context, query string, limit int) (SearchResult, error)
}
```

- Lives alongside the existing `Searcher` interface — does not replace or modify it.
- Returns the same `SearchResult` type (reuse `ports.SearchResult`).
- `limit` controls how many results yt-dlp returns (default: 10).

### 4.2 New Adapter: `querysearcher`

A new package `internal/adapters/querysearcher/` that:

- Shells out to `yt-dlp --flat-playlist --dump-json --ignore-errors "ytmusicsearch:{limit} {query}"`
- Reuses `searcher.ParseLine()` to parse each JSON line — **zero changes** to the parse function.
- Handles error states: yt-dlp not found, empty results, network errors, etc.

**Why `ytmusicsearch:` instead of `ytsearch:`?**  
`ytmusicsearch:` is specific to YouTube Music and returns music-focused results (songs, albums, artists). `ytsearch:` returns generic YouTube results (videos, shorts, etc.). Since music-dl is a music downloader, YouTube Music search is the correct source.

### 4.3 TUI Changes

A new `SearchMode` toggle (distinct from the existing `SourceMode`):

| Mode | Behavior |
|------|----------|
| **URL mode** (default) | Current behavior — paste a URL, resolve tracks |
| **Search mode** | Type a text query, results shown as a selectable list |

**User flow:**

1. User opens music-dl → sees the input screen in URL mode by default
2. User presses a keybinding (e.g. `s` or `Tab`-like toggle) to switch to Search mode
3. Input placeholder changes: `"Search YouTube Music: "`
4. User types a query (e.g. `"rock baladas 90s"`) and presses Enter
5. TUI shows a resolving spinner → results appear as a selectable list
6. User picks tracks with Space, presses Enter to download
7. User can toggle back to URL mode at any time

**When a non-URL is typed in URL mode:**  
Show a suggestion: `"That doesn't look like a URL. Press 's' to switch to Search mode."` (no auto-switch).

### 4.4 Shared Infrastructure Reuse

- `searcher.ParseLine()` → shared, no changes
- Playlist screen → reused as-is for search results
- Download flow → unchanged
- Footer/help view → updated to show Search mode keybinding

## 5. Scope / First Slice

| In scope | Out of scope |
| ---------- | -------------- |
| `QuerySearcher` interface in `core/ports/` | Semantic/embedding search |
| `querysearcher` adapter with ytmusicsearch: | Search-box with autocomplete |
| Toggle between URL/Search mode in TUI | Pagination (more results beyond the limit) |
| Results shown as selectable playlist list | Search history |
| Suggestion when non-URL typed in URL mode | Favorites or saved searches |
| Reuse of `ParseLine()` and playlist screen | Spotify search integration |
| Updated footer/help with search-mode keys | Search result caching |

## 6. Non-Goals (Explicit)

- NOT semantic/embedding search — keyword search via `ytmusicsearch:` only.
- NOT a search-box UI with autocomplete — just a plain text input.
- NO pagination — `ytmusicsearch:10` is a fixed result set.
- Does NOT touch the Spotify adapter or its resolution flow.
- Does NOT change the existing `Searcher` interface — backwards compatible.
- Does NOT add search history, favorites, or persistent state.
- Does NOT add a search results counter beyond what yt-dlp returns.

## 7. Product Constraints

| Constraint | Decision |
| ------------ | ---------- |
| Requires internet? | Yes — yt-dlp searches YouTube Music live |
| Requires yt-dlp? | Yes — yt-dlp must be installed (already a requirement) |
| Requires YouTube Music availability? | Yes — `ytmusicsearch:` depends on yt-dlp's YouTube Music extractor, which may be blocked in some regions |
| Offline fallback? | No — search is inherently online |
| Performance | Search is a single yt-dlp invocation per query (same as URL resolution) |
| Rate limiting | No API key needed — yt-dlp scrapes YouTube Music directly (subject to same throttling as any yt-dlp operation) |

## 8. Business Trade-offs

| Trade-off | Implication |
| ----------- | ------------- |
| `ytmusicsearch:` depends on yt-dlp's YouTube Music extractor | If yt-dlp changes its search format, the adapter may need updates |
| `ytmusicsearch:` vs `ytsearch:` | YouTube Music search is more focused for music but may miss content only on standard YouTube |
| No pagination | Simple implementation, but users who want more than 10 results are limited |
| Separate `QuerySearcher` port (not merged into `Searcher`) | Cleaner separation of concerns, but more interfaces to wire in `main.go` |

## 9. Edge Cases

| Edge Case | Behavior |
| ----------- | ---------- |
| **Empty query** | Show error: "Please enter a search query" (same as empty URL) |
| **Query too long** | yt-dlp handles long queries; no explicit limit enforced beyond yt-dlp's own limits |
| **Special characters in query** | yt-dlp handles shell escaping; the adapter uses `exec.Command` so shell injection is not a concern |
| **No results** | Show "No results found for '<query>'" and return to input screen |
| **Network error during search** | Show error message, return to input screen, allow retry |
| **yt-dlp not found** | Preflight check already handles this at startup |
| **YouTube Music blocked in region** | yt-dlp will return an error; show it as a resolve error |
| **Rapid repeated searches** | Each search is a new yt-dlp invocation; context cancellation handled by existing `exec.CommandContext` |
| **Results with missing titles** | `ParseLine` skips them silently (same as current behavior) |
| **Switch mode mid-search** | Toggling mode clears the input and any previous results |
| **Partial results** | yt-dlp returns partial results on some errors; handled by existing logic in `Searcher.Search` |

## 10. Risks

| Risk | Mitigation |
| ------ | ------------ |
| `ytmusicsearch:` format changes in yt-dlp | Pin yt-dlp version in docs; the adapter reuses `ParseLine` which is stable across JSON schema changes |
| TUI complexity from mode toggle | Keep mode switching simple (single keybinding `s`); clear visual indicator of active mode |
| User confusion between Search mode and Source mode | Search mode and Source mode are orthogonal: Source mode selects the *backend*; Search mode selects the *input type* |
| Scope creep (pagination, history, autocomplete) | First slice is deliberately minimal; future iterations can add these |

## 11. Rollback

This change is additive — it adds new interfaces, adapter, and TUI mode without modifying existing code paths. Rollback means:

1. Revert the TUI changes (model, update, view, keys, messages)
2. Remove the `querysearcher` adapter package
3. Remove the `QuerySearcher` port interface
4. The existing `Searcher` interface, `searcher` adapter, and TUI URL mode remain untouched

No existing functionality is affected during the change.

## 12. Success Criteria

| Criterion | Measurement |
| ----------- | ------------- |
| Query search returns results | `SearchByQuery("test")` returns ≥1 track from yt-dlp |
| Results parse correctly | Every returned track has valid `Title`, `Artist`, `URL` |
| Search mode toggle works | Keybinding switches between URL and Search mode with visual indicator |
| Non-URL suggestion in URL mode | Typing a non-URL shows the suggestion, not a crash |
| Empty query rejected | Error shown, no yt-dlp invocation |
| No regression in URL mode | All existing URL resolutions still work as before |
| Tests pass | `go test -race ./...` passes |
