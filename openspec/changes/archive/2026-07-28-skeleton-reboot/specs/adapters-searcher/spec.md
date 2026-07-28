# Adapters — Searcher (yt-dlp) Specification

## Purpose

Implement the `Searcher` port by invoking `yt-dlp --flat-playlist --dump-json` and parsing the JSON output into `[]domain.Media`.

## Requirements

### Requirement: Searcher invokes yt-dlp with correct flags

The system MUST provide a `Searcher` implementation that shells out to `yt-dlp`.

```go
type Searcher struct{}

func NewSearcher() *Searcher
func (s *Searcher) Search(ctx context.Context, url string) (ports.SearchResult, error)
```

#### Scenario: Search calls yt-dlp with --flat-playlist --dump-json

- GIVEN a `Searcher` instance
- WHEN `Search(ctx, url)` is called
- THEN the executable `yt-dlp` MUST be invoked
- AND the arguments MUST include `--flat-playlist`
- AND the arguments MUST include `--dump-json`
- AND the URL MUST be passed as the last argument
- AND `--ignore-errors` MUST be included to skip unavailable tracks in playlists without aborting

#### Scenario: Search parses yt-dlp JSON output into Media items

- GIVEN a `Searcher` instance
- WHEN `yt-dlp` returns valid JSON lines (one JSON object per line)
- THEN each line MUST be parsed into a `domain.Media`
- AND `Media.URL` MUST be set from the JSON `"url"` or `"webpage_url"` field
- AND `Media.Title` MUST be set from the JSON `"title"` field
- AND `Media.Artist` MUST be set from the JSON `"channel"` or `"uploader"` or `"creator"` field (first non-empty wins)
- AND `Media.Duration` MUST be set from the JSON `"duration"` field (float seconds → `time.Duration`)
- AND `Media.Source` MUST be set to `"youtube"` or `"youtube-music"` based on the URL host
- AND `Media.Status` MUST be set to `StatusResolved`

#### Scenario: Search handles single-video URL (single JSON object)

- GIVEN a `Searcher` instance
- WHEN the URL is a single video (not a playlist)
- THEN `yt-dlp --flat-playlist --dump-json` returns exactly one JSON object
- AND `SearchResult.Tracks` MUST have length 1

#### Scenario: Search handles playlist URL (multiple JSON lines)

- GIVEN a `Searcher` instance
- WHEN the URL is a playlist
- THEN `yt-dlp --flat-playlist --dump-json` returns one JSON line per track
- AND `SearchResult.Tracks` MUST have one `Media` per line

#### Scenario: Search returns error when yt-dlp is not found

- GIVEN a `Searcher` instance
- WHEN `yt-dlp` is not on `$PATH`
- THEN `Search` MUST return a `domain.Error` with code `ErrorBinaryNotFound`

#### Scenario: Search returns error on yt-dlp non-zero exit

- GIVEN a `Searcher` instance
- WHEN `yt-dlp` exits with a non-zero status
- THEN `Search` MUST return a non-nil `error`
- AND the error SHOULD include stderr output for diagnostics

#### Scenario: Search handles partial JSON parse failures gracefully

- GIVEN a `Searcher` instance
- WHEN yt-dlp outputs a mix of valid and invalid JSON lines (e.g., a private video in a playlist)
- THEN `Search` MUST skip invalid JSON lines (with `--ignore-errors`)
- AND return successfully parsed tracks
- AND SHOULD NOT abort the entire search on a single bad line

### Requirement: JSON parsing is in a separate helper file

The system MUST provide a `parse.go` file with pure functions for parsing yt-dlp JSON output, separate from the execution logic.

```go
// ParseLine parses a single JSON line from yt-dlp --dump-json output.
func ParseLine(line string) (domain.Media, error)
```

#### Scenario: ParseLine handles complete JSON with all fields

- GIVEN a JSON string with `webpage_url`, `title`, `channel`, `duration`
- WHEN `ParseLine(jsonString)` is called
- THEN it MUST return a fully populated `domain.Media`
- AND `Status` MUST be `StatusResolved`

#### Scenario: ParseLine handles minimal JSON (only required fields)

- GIVEN a JSON string with only `webpage_url` and `title`
- WHEN `ParseLine(jsonString)` is called
- THEN it MUST return a `domain.Media` with URL and Title set
- AND Artist MUST be `""` (optional field)
- AND Duration MUST be `0` (optional field)
- AND no error MUST be returned

#### Scenario: ParseLine handles JSON with float duration

- GIVEN a JSON string with `"duration": 245.5`
- WHEN `ParseLine(jsonString)` is called
- THEN `Media.Duration` MUST be approximately 245.5 seconds

#### Scenario: ParseLine returns error for invalid JSON

- GIVEN a non-JSON string
- WHEN `ParseLine(badString)` is called
- THEN it MUST return a `domain.Error` with code `ErrorGeneric`

#### Scenario: ParseLine extracts artist from multiple fields

- GIVEN JSON with `"channel": "Some Channel"`, `"uploader": "Some Uploader"`, `"creator": "Some Creator"`
- WHEN `ParseLine(jsonString)` is called
- THEN `Media.Artist` MUST be `"Some Channel"` (channel first priority)
- WHEN only `uploader` is present
- THEN `Media.Artist` MUST be the uploader value
- WHEN only `creator` is present
- THEN `Media.Artist` MUST be the creator value

---

## Test Specifications

### Test: JSON parsing (unit)

**File:** `internal/adapters/searcher/parse_test.go`

| Case | Input | Expected |
| ------ | ------- | ---------- |
| Complete JSON with all fields | Full JSON string | Fully populated `domain.Media`, `StatusResolved` |
| Minimal JSON (url + title only) | JSON with only `webpage_url` and `title` | Media with URL and Title, Artist="" |
| JSON with float duration | `{"duration": 180.0}` | Duration = 3 minutes |
| Invalid JSON | `"not json"` | `domain.Error` with `ErrorGeneric` |
| JSON with channel field | `{"channel": "Artist Name"}` | Artist = "Artist Name" |
| JSON with uploader (no channel) | `{"uploader": "Artist Name"}` | Artist = "Artist Name" |
| JSON with creator (no channel/uploader) | `{"creator": "Artist Name"}` | Artist = "Artist Name" |
| JSON with no artist fields | `{"title": "Song"}` | Artist = "" |

### Test: Searcher execution (integration)

**File:** `internal/adapters/searcher/ytdlp_test.go`

**Skip condition:** `testing.Short()` — integration tests require real `yt-dlp` on `$PATH`.

| Case | URL | Expected |
| ------ | ----- | ---------- |
| Single video resolution | Valid YouTube video URL | Returns 1 track |
| Playlist resolution | Valid YouTube playlist URL (≥2 tracks) | Returns ≥2 tracks |
| Invalid URL | `"https://example.com"` | Error returned |
| Empty URL | `""` | Error returned |
