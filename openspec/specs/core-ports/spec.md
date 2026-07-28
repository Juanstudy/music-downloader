# Core Ports Specification

## Purpose

Define the provider interfaces (`Searcher`, `Downloader`, `PreflightChecker`) that form the boundary between the core domain and the outside world. These interfaces are the dependency inversion boundary: adapters implement them, and the service layer depends on them.

## Requirements

### Requirement: Ports package has minimal imports

The `core/ports` package MUST import only `context` and `core/domain`. It MUST NOT import adapters, `core/service`, or any Bubble Tea / TUI packages.

#### Scenario: Package compiles with only domain and context

- GIVEN a file `internal/core/ports/searcher.go`
- WHEN compiled with `go build ./internal/core/ports/`
- THEN it MUST succeed
- AND the only imports MUST be `context` and `core/domain`

### Requirement: Searcher interface with SearchResult

The system MUST provide a `Searcher` interface for resolving a URL into a list of playable tracks.

```go
type SearchResult struct {
    Tracks []domain.Media
    Source string
}

type Searcher interface {
    Search(ctx context.Context, url string) (SearchResult, error)
}
```

#### Scenario: Search takes a URL and returns tracks

- GIVEN a type implementing `Searcher`
- WHEN `Search(ctx, url)` is called with a valid YouTube/YouTube Music URL
- THEN it MUST return `SearchResult` with one or more `Media` items
- AND `Source` MUST indicate the source (e.g. `"youtube"`, `"youtube-music"`)

#### Scenario: Search returns error for invalid URL

- GIVEN a type implementing `Searcher`
- WHEN `Search(ctx, url)` is called with an invalid or non-YouTube URL
- THEN it MUST return a non-nil `error`
- AND the error type SHOULD be `domain.Error` with code `ErrorInvalidURL` when the URL format is recognizable but unsupported

#### Scenario: Search handles playlist and single-video URLs

- GIVEN a type implementing `Searcher`
- WHEN `Search(ctx, url)` is called with a playlist URL
- THEN `SearchResult.Tracks` MUST contain all tracks in the playlist
- WHEN called with a single-video URL
- THEN `SearchResult.Tracks` MUST contain exactly one `Media` item

### Requirement: Downloader interface with DownloadResult

The system MUST provide a `Downloader` interface for downloading a single track to a specified output directory.

```go
type DownloadResult struct {
    Media      domain.Media
    OutputPath string
}

type Downloader interface {
    Download(ctx context.Context, media domain.Media, outputDir string) (DownloadResult, error)
}
```

#### Scenario: Download takes a Media and output directory, returns result

- GIVEN a type implementing `Downloader`
- WHEN `Download(ctx, media, outputDir)` is called with a valid resolved `Media`
- THEN it MUST return `DownloadResult` with the updated `Media` (status `StatusDone`, `OutputPath` set)
- AND `OutputPath` MUST be the absolute or relative path to the downloaded file

#### Scenario: Download returns error on failure

- GIVEN a type implementing `Downloader`
- WHEN `Download(ctx, media, outputDir)` fails (network, disk, or binary error)
- THEN it MUST return a non-nil `error`
- AND the error type SHOULD be `domain.Error` with an appropriate `ErrorCode`

#### Scenario: Download returns error for already-done media

- GIVEN a type implementing `Downloader`
- WHEN `Download(ctx, media, outputDir)` is called with `media.Status == StatusDone`
- THEN the implementation MAY handle this gracefully (re-download or skip)

### Requirement: PreflightChecker interface

The system MUST provide a `PreflightChecker` interface for validating that required external binaries are available before starting the application.

```go
type PreflightError struct {
    Binary string
    Err    error
}

type PreflightChecker interface {
    Check(ctx context.Context) []PreflightError
}
```

#### Scenario: Check returns empty slice when all binaries found

- GIVEN a type implementing `PreflightChecker`
- WHEN `Check(ctx)` is called and all required binaries are on `$PATH`
- THEN it MUST return an empty (or nil) slice

#### Scenario: Check returns errors for missing binaries (non fail-fast)

- GIVEN a type implementing `PreflightChecker`
- WHEN `Check(ctx)` is called and one or more required binaries are missing
- THEN it MUST return a slice with one `PreflightError` per missing binary
- AND `PreflightError.Binary` MUST name the missing binary (e.g., `"yt-dlp"`, `"ffmpeg"`)
- AND `PreflightError.Err` MUST be a non-nil error

#### Scenario: Check collects all errors, does not fail-fast

- GIVEN a type implementing `PreflightChecker`
- WHEN both `yt-dlp` and `ffmpeg` are missing
- THEN the returned slice MUST have length 2
- AND both binaries MUST be represented in the errors

---

## Test Specifications

### Test: SearchResult struct

**File:** `internal/core/ports/searcher_test.go`

| Case | Assertion |
|------|-----------|
| Zero-value `SearchResult{}` has empty Tracks and empty Source | `Tracks` is nil, `Source` is `""` |
| SearchResult populated via struct literal | Fields return set values |

### Test: DownloadResult struct

**File:** `internal/core/ports/downloader_test.go`

| Case | Assertion |
|------|-----------|
| Zero-value `DownloadResult{}` has zero Media and empty OutputPath | `Media` is zero-value, `OutputPath` is `""` |

### Test: PreflightError struct

**File:** `internal/core/ports/preflight_test.go`

| Case | Assertion |
|------|-----------|
| Zero-value `PreflightError{}` has empty Binary and nil Err | `Binary` is `""`, `Err` is nil |

### Test: Interface compliance (compile-time check)

| Case | Assertion |
|------|-----------|
| Adapter types satisfy `Searcher`, `Downloader`, `PreflightChecker` | Compile-time assignment `var _ ports.Searcher = (*adapter)(nil)` compiles |
