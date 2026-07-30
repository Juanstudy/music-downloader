# Core Service Specification

## Purpose

Define the `Orchestrator` that composes `Searcher` and `Downloader` port implementations into executable use cases: resolving a URL to a list of tracks, and downloading selected tracks sequentially.

## Requirements

### Requirement: Orchestrator struct with explicit DI constructor

The system MUST provide an `Orchestrator` struct that receives its dependencies via constructor.

```go
type Orchestrator struct {
    searcher   ports.Searcher
    downloader ports.Downloader
}

func NewOrchestrator(s ports.Searcher, d ports.Downloader) *Orchestrator
```

#### Scenario: Orchestrator constructor wires dependencies

- GIVEN a `Searcher` implementation and a `Downloader` implementation
- WHEN `NewOrchestrator(s, d)` is called
- THEN it MUST return a non-nil `*Orchestrator`
- AND the returned instance MUST use the provided `s` for searches and `d` for downloads

### Requirement: ResolveTrack method

The system MUST provide a `ResolveTrack` method that uses the `Searcher` to resolve a URL.

```go
func (o *Orchestrator) ResolveTrack(ctx context.Context, url string) ([]domain.Media, error)
```

#### Scenario: ResolveTrack returns tracks for a valid URL

- GIVEN an `Orchestrator` with a `Searcher` that returns tracks
- WHEN `ResolveTrack(ctx, url)` is called with a valid URL
- THEN it MUST return a non-nil, non-empty `[]domain.Media`
- AND each `Media` MUST have `Status == StatusResolved`
- AND each `Media` MUST have `Source` set

#### Scenario: ResolveTrack propagates searcher errors

- GIVEN an `Orchestrator` with a `Searcher` that returns an error
- WHEN `ResolveTrack(ctx, url)` is called
- THEN it MUST return nil tracks and the error from the `Searcher`

#### Scenario: ResolveTrack validates URL before calling Searcher

- GIVEN an `Orchestrator`
- WHEN `ResolveTrack(ctx, "")` is called with an empty string
- THEN it MUST return a `domain.Error` with code `ErrorInvalidURL`
- WHEN called with a non-empty but clearly invalid URL (e.g., `"not-a-url"`)
- THEN it MAY validate before calling the searcher, returning `domain.Error{Code: ErrorInvalidURL}`

### Requirement: DownloadTracks method

The system MUST provide a `DownloadTracks` method that downloads multiple tracks sequentially using the `Downloader`.

```go
type Result struct {
    Media  domain.Media
    Err    error
}

func (o *Orchestrator) DownloadTracks(ctx context.Context, tracks []domain.Media, outputDir string) <-chan Result
```

#### Scenario: DownloadTracks returns results via channel as each completes

- GIVEN an `Orchestrator` with a `Downloader` that succeeds
- WHEN `DownloadTracks(ctx, []domain.Media{...}, "output/")` is called
- THEN it MUST return a receive-only `<-chan Result`
- AND exactly one `Result` MUST be sent per track
- AND each `Result.Media.Status` MUST be `StatusDone` on success or `StatusFailed` on failure

#### Scenario: DownloadTracks processes sequentially (one at a time)

- GIVEN an `Orchestrator` with a slow `Downloader`
- WHEN `DownloadTracks(ctx, tracks, "output/")` is called with 3 tracks
- THEN track 2 MUST NOT begin downloading until track 1's `Result` has been received from the channel
- (Verified with a mock `Downloader` that blocks until a signal is received)

#### Scenario: DownloadTracks continues on per-track failure

- GIVEN an `Orchestrator` with a `Downloader` that fails on the second track
- WHEN `DownloadTracks(ctx, []domain.Media{a, b, c}, "output/")` is called
- THEN it MUST send 3 `Result`s on the channel
- AND `Result{Media: b, Err: <error>}` MUST be sent for the failed track
- AND track c MUST still be attempted (failures do not abort the queue)

#### Scenario: DownloadTracks closes the channel after all tracks

- GIVEN an `Orchestrator`
- WHEN `DownloadTracks(ctx, tracks, "output/")` is called
- THEN after the last `Result` is received, the next receive from the channel MUST return `false` (channel closed)

#### Scenario: DownloadTracks respects context cancellation

- GIVEN an `Orchestrator`
- WHEN the context passed to `DownloadTracks` is cancelled mid-download
- THEN the channel MUST be closed without processing remaining tracks
- AND any in-flight download SHOULD be cancelled

---

## Test Specifications

### Test: Orchestrator with mock dependencies

**File:** `internal/core/service/orchestrator_test.go`

**Mock setup:**

- `mockSearcher` implementing `ports.Searcher` with configurable `SearchFunc`
- `mockDownloader` implementing `ports.Downloader` with configurable `DownloadFunc`

| Test Case | Input | Expected |
| ----------- | ------- | ---------- |
| `ResolveTrack` with valid URL | URL string, mock searcher returns 2 tracks | Returns 2 `Media` items, `Status == StatusResolved` |
| `ResolveTrack` with searcher error | URL string, mock searcher returns error | Nil tracks, error propagated |
| `ResolveTrack` with empty URL | Empty string | `domain.Error` with `ErrorInvalidURL` |
| `DownloadTracks` all succeed | 3 tracks, mock downloader succeeds | 3 `Result`s on channel, all `StatusDone` |
| `DownloadTracks` one fails | 3 tracks, mock fails on second | 3 results, second has error, third succeeds |
| `DownloadTracks` context cancelled | 3 tracks, cancel after first | Channel closes, remaining not processed |
| `DownloadTracks` empty tracks | Empty slice | Channel sends nothing, immediately closed |
