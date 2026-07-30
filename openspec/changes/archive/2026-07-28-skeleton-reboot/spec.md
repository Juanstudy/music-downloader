# Skeleton Reboot — Consolidated Specification

## Overview

Greenfield implementation of hexagonal architecture for music-dl. This spec covers 8 domain areas:

1. `core/domain/` — Pure types with stdlib only
2. `core/ports/` — Provider interfaces (Searcher, Downloader, PreflightChecker)
3. `core/service/` — Orchestrator composing ports
4. `adapters/searcher/` — yt-dlp JSON search adapter
5. `adapters/downloader/` — yt-dlp audio download adapter
6. `adapters/preflight/` — Binary availability checker
7. `adapters/filesystem/` — Output directory management
8. `internal/tui/` — 5-screen Bubble Tea application
9. `cmd/music-dl/` — Entrypoint with DI wiring

Full domain-level specs under: `openspec/changes/skeleton-reboot/specs/{domain}/spec.md`

---

## 1. core/domain — Domain Types

**File:** `openspec/changes/skeleton-reboot/specs/core-domain/spec.md`

### Types

```go
type Status int
const (
    StatusPending Status = iota     // 0
    StatusResolving                 // 1
    StatusResolved                  // 2
    StatusDownloading               // 3
    StatusDone                      // 4
    StatusFailed                    // 5
)

type Media struct {
    URL        string
    Title      string
    Artist     string
    Duration   time.Duration
    Source     string
    Status     Status
    Error      string
    OutputPath string
}

type ErrorCode int
const (
    ErrorGeneric        ErrorCode = iota  // 0
    ErrorNetwork                          // 1
    ErrorInvalidURL                       // 2
    ErrorBinaryNotFound                   // 3
    ErrorTrackUnavailable                 // 4
    ErrorAgeRestricted                    // 5
    ErrorDiskFull                         // 6
)

type Error struct {
    Code    ErrorCode
    Message string
    Track   string
}
```

### Key constraints

- Zero external imports (stdlib `"time"` only)
- No exported functions, constructors, or helper methods
- Types created via struct literals

---

## 2. core/ports — Provider Interfaces

**File:** `openspec/changes/skeleton-reboot/specs/core-ports/spec.md`

### Searcher

```go
type SearchResult struct {
    Tracks []domain.Media
    Source string  // "youtube", "youtube-music"
}
type Searcher interface {
    Search(ctx context.Context, url string) (SearchResult, error)
}
```

### Downloader

```go
type DownloadResult struct {
    Media      domain.Media
    OutputPath string
}
type Downloader interface {
    Download(ctx context.Context, media domain.Media, outputDir string) (DownloadResult, error)
}
```

### PreflightChecker

```go
type PreflightError struct {
    Binary string
    Err    error
}
type PreflightChecker interface {
    Check(ctx context.Context) []PreflightError
}
```

---

## 3. core/service — Orchestrator

**File:** `openspec/changes/skeleton-reboot/specs/core-service/spec.md`

```go
type Orchestrator struct { ... }
func NewOrchestrator(s ports.Searcher, d ports.Downloader) *Orchestrator
func (o *Orchestrator) ResolveTrack(ctx context.Context, url string) ([]domain.Media, error)
func (o *Orchestrator) DownloadTracks(ctx context.Context, tracks []domain.Media, outputDir string) <-chan Result
```

### ResolveTrack behavior

- Validates URL (non-empty, valid format) → returns `domain.Error{ErrorInvalidURL}` on invalid
- Delegates to `Searcher.Search()`
- Sets each track's `Status` to `StatusResolved`

### DownloadTracks behavior

- Sequential processing: one track at a time via `Downloader.Download()`
- Returns results via channel; closes channel when done
- Continues on per-track failure (non-aborting)
- Respects context cancellation

---

## 4. adapters/searcher — yt-dlp JSON Search

**File:** `openspec/changes/skeleton-reboot/specs/adapters-searcher/spec.md`

- Invokes: `yt-dlp --flat-playlist --dump-json --ignore-errors <url>`
- Parses JSON lines into `domain.Media`
- Artist extraction priority: `channel` → `uploader` → `creator`
- Duration: float64 seconds → `time.Duration`
- `parse.go` has pure `ParseLine(line string) (domain.Media, error)` helper
- Errors: `ErrorBinaryNotFound` (missing yt-dlp), error on non-zero exit
- **Integration test** skips with `-short`

---

## 5. adapters/downloader — yt-dlp Audio Download

**File:** `openspec/changes/skeleton-reboot/specs/adapters-downloader/spec.md`

- Invokes: `yt-dlp -x --audio-format mp3 --embed-metadata -o "{artist} - {title}.mp3" <url>`
- Output path determined via `--print filename` or post-download discovery
- Sets `Media.Status` to `StatusDone` on success, `StatusFailed` on error
- Error types: `ErrorBinaryNotFound`, network/disk errors
- **Integration test** skips with `-short`

---

## 6. adapters/preflight — Binary Checker

**File:** `openspec/changes/skeleton-reboot/specs/adapters-preflight/spec.md`

- Uses `exec.LookPath` for `["yt-dlp", "ffmpeg"]` (configurable)
- Collects all missing binaries (non fail-fast)
- **Unit test** with `t.TempDir()` + fake PATH

---

## 7. adapters/filesystem — Output Directory

**File:** `openspec/changes/skeleton-reboot/specs/adapters-filesystem/spec.md`

- `NewOutput(basePath)` with tilde expansion
- `Ensure(ctx)` creates directory tree
- `FullPath()` returns expanded absolute path
- Default path: `~/Music/music-dl`
- **Unit test** with `t.TempDir()`

---

## 8. internal/tui — 5-Screen Bubble Tea App

**File:** `openspec/changes/skeleton-reboot/specs/internal-tui/spec.md`

### Screens (state machine)

```
Input → Resolving → Playlist → Downloading → Done
  ↑         ↑                                |
  └─────────┴────────────────────────────────┘
```

### Key behaviors

- **Input**: URL text entry, validation, Enter → Resolving, Esc → quit
- **Resolving**: Spinner, async resolve via `resolveResultMsg`, Esc → Input
- **Playlist**: Track list with j/k navigation, Space toggle, a/n select all/none, Enter → Downloading
- **Downloading**: Sequential progress, per-track `downloadProgressMsg`, auto → Done
- **Done**: Summary, Enter → Input, q → quit

### Model structure

```go
type Model struct {
    orchestrator   *service.Orchestrator
    Screen, PrevScreen Screen
    Width, Height  int
    InputText, InputError string
    ResolvingError string
    Tracks         []domain.Media
    Cursor, Scroll int
    CurrentDownload int
    DownloadComplete bool
    TotalTracks, Succeeded, Failed int
    FailedTracks   []domain.Media
}
```

### Test pattern: direct `Model.Update()` with `tea.KeyMsg` and custom messages

- All screen transitions tested (9 transitions)
- Playlist selection (toggle, select-all, select-none)
- Window resize
- Ctrl+C quit from any screen

---

## 9. cmd/music-dl — Entrypoint

**File:** `openspec/changes/skeleton-reboot/specs/cmd-entrypoint/spec.md`

### main() flow

1. Build adapters: `preflight.NewChecker()`, `filesystem.NewOutput(...)`, `searcher.NewSearcher()`, `downloader.NewDownloader()`
2. Run preflight check → exit 1 with errors if any binary missing
3. Ensure output directory → exit 1 on failure
4. `service.NewOrchestrator(searcher, downloader)`
5. `tui.NewModel(orchestrator)`
6. `tea.NewProgram(model, tea.WithAltScreen()).Run()`

---

## Cross-Cutting Constraints

### Import rules

| Package | Imports from |
| --------- | ------------- |
| `cmd/music-dl` | All adapters, `core/service`, `core/ports`, `internal/tui` |
| `internal/tui` | `core/service`, `core/domain` |
| `core/service` | `core/ports`, `core/domain` |
| `core/ports` | `core/domain` |
| `core/domain` | stdlib only |
| `adapters/*` | `core/ports`, `core/domain` |

### Test strategy

- **Unit tests** (`-short`): domain types, orchestrator (mocks), JSON parsing, preflight (fake PATH), filesystem (TempDir)
- **Integration tests** (skip `-short`): yt-dlp searcher, yt-dlp downloader (require real binaries)
- **TUI tests**: direct `Model.Update()` with message fixtures

### Compilation verification

- `go build ./cmd/music-dl/` produces binary
- `go vet ./...` passes with no warnings
- `go test -short ./internal/...` passes all unit tests
