# Skeleton Reboot: Hexagonal Architecture for music-dl

**Change:** `skeleton-reboot`  
**Driver:** @juan-arch  
**Status:** Proposed  

Go source was deleted to reboot with a clean hexagonal-architecture foundation following ADR-001 through ADR-006. This proposal defines the layers, interfaces, screen flow, package structure, and test strategy for the greenfield implementation.

---

## What this proposal covers

| Area | Decision |
| --- | --- |
| **Architecture style** | Strict hexagonal (ports & adapters). Core domain depends on nothing external. Adapters wired via explicit dependency injection in `main.go`. |
| **Core domain boundaries** | Domain types (`Media`, `Status`, `Error`), use-case interfaces (ports), and orchestration logic live in `internal/core/`. No `os/exec`, no Bubble Tea imports, no yt-dlp references. |
| **Provider interfaces** | `Searcher` (resolve URL → track list) and `Downloader` (download one track → result) defined in `internal/core/ports/`. Adapters implement them. |
| **Screen/state machine** | 5-screen TUI with deterministic transitions, error states per screen, and a single flat FSM model in the TUI `Model`. |
| **Package structure** | `cmd/`, `internal/core/`, `internal/adapters/`, `internal/tui/`. Each layer in its own Go package with no circular imports. |
| **Test strategy** | Unit tests for core (table-driven, no external deps). Integration tests for adapters (skip in `-short`). TUI state tests via direct `Model.Update()` calls. |

---

## Current state (no Go code)

Zero `.go` files exist. The project has:

- `go.mod` / `go.sum` with Bubble Tea, lipgloss, and bubbles as dependencies.
- `docs/adr/` with six accepted ADRs covering stack, engine, MVP scope, error handling, file naming, and TUI design.
- `.agents/skills/` with Bubble Tea and TUI design skill files.
- No `internal/` structure, no `cmd/`, no adapters, no tests.

---

## Hexagonal architecture layers

### Layer diagram

```
┌──────────────────────────────────────────────────────┐
│                    cmd/music-dl/                      │
│                   main.go (DI wiring)                 │
├──────────┬───────────────────────────┬────────────────┤
│          │                           │                │
│  adapters/  ◄────  core/ports/  ────►  adapters/     │
│  searcher │        (interfaces)      │  downloader    │
│  ytdlp.go  ◄───  Searcher ─────────►  ytdlp.go       │
│          │        Downloader         │                │
│          │        PreflightChecker   │                │
│          │                           │                │
│          └───────────┬───────────────┘                │
│                      │                               │
│               core/service/                           │
│               Orchestrator (use cases)                │
│                      │                               │
├──────────────────────┴────────────────────────────────┤
│                    internal/tui/                       │
│         Bubble Tea Model + Update + View              │
│         (calls core ports via injected service)       │
└──────────────────────────────────────────────────────┘
```

### Layer rules

1. **`internal/core/`** — Zero external dependencies. Imports only stdlib and domain types.
   - `core/domain/` — Pure types: `Media`, `Status`, `Result`, `Error`. No behavior.
   - `core/ports/` — Interfaces: `Searcher`, `Downloader`, `PreflightChecker`. No implementation.
   - `core/service/` — Orchestration logic that composes ports. Stateless, receives port implementations via constructor (explicit DI).

2. **`internal/adapters/`** — Implements `core/ports/`. Each adapter is one file/package. External dependencies live here.
   - `adapters/searcher/ytdlp.go` — Resolves URLs to track lists via `yt-dlp --flat-playlist --dump-json`.
   - `adapters/downloader/ytdlp.go` — Downloads one track via `yt-dlp -x --audio-format mp3 --embed-metadata`.
   - `adapters/preflight/checker.go` — Checks that `yt-dlp` and `ffmpeg` are on `$PATH`.
   - `adapters/filesystem/output.go` — Manages output directory (create if missing, write files).

3. **`internal/tui/`** — Bubble Tea app. Separated from both core and adapters.
   - Knows about `core/ports/` (it receives the service interface).
   - Does NOT know about adapter implementations directly.
   - Does NOT import yt-dlp, os/exec, or filesystem packages.

4. **`cmd/music-dl/main.go`** — Entrypoint.
   - Runs `PreflightChecker` synchronously before initializing the TUI.
   - Wires adapters → core service → TUI model via explicit constructor calls.
   - No business logic. Only composition and startup.

### Dependency graph (compile-time)

```
cmd/music-dl/main.go
  ├── internal/tui/          (imports core/service)
  │     └── internal/core/service/   (imports core/ports, core/domain)
  │           ├── core/ports/        (interfaces only, has domain types)
  │           └── core/domain/       (pure types, no imports)
  │
  ├── internal/adapters/searcher/    (imports core/ports, core/domain)
  ├── internal/adapters/downloader/  (imports core/ports, core/domain)
  └── internal/adapters/preflight/   (imports core/ports)
```

**No circular imports.** Each layer imports only the layer directly below it.

---

## Provider interfaces (core/ports)

### `Searcher` — resolves a URL to a list of tracks

```go
type SearchResult struct {
    Tracks []domain.Media
    Source string // "youtube", "youtube-music"
}

type Searcher interface {
    Search(ctx context.Context, url string) (SearchResult, error)
}
```

- `Search` receives a validated URL, executes `yt-dlp --flat-playlist --dump-json`, parses JSON output into `[]domain.Media`.
- For single-video URLs, returns a slice of one `Media`.
- For playlist URLs, returns all tracks in the playlist.
- Error cases: invalid URL, network error, private/deleted video, age restriction. Adapter maps yt-dlp exit codes to `domain.Error` types.

### `Downloader` — downloads one track

```go
type DownloadResult struct {
    Media  domain.Media
    OutputPath string
}

type Downloader interface {
    Download(ctx context.Context, media domain.Media, outputDir string) (DownloadResult, error)
}
```

- `Download` executes `yt-dlp -x --audio-format mp3 --embed-metadata -o "{artist} - {title}.mp3" <url>`.
- Single responsibility: one track, one call.
- Error cases: yt-dlp not found, network failure, disk full, corrupted output. Adapter maps to `domain.Error`.
- Output path is constructed from `outputDir + "/" + filename` as returned by yt-dlp's `--print filename` after download.

### `PreflightChecker` — validates dependencies at startup

```go
type PreflightError struct {
    Binary string // "yt-dlp" or "ffmpeg"
    Err    error
}

type PreflightChecker interface {
    Check(ctx context.Context) []PreflightError
}
```

- `Check` runs `which yt-dlp` and `which ffmpeg` (or equivalent `exec.LookPath`).
- Returns a slice — collects all missing binaries, does not fail-fast.
- If any errors are returned, the TUI never starts; main.go prints the errors and exits with code 1.

---

## Domain types (core/domain)

```go
type Status int

const (
    StatusPending Status = iota
    StatusResolving
    StatusResolved
    StatusDownloading
    StatusDone
    StatusFailed
)

type Media struct {
    URL        string
    Title      string
    Artist     string
    Duration   time.Duration
    Source     string    // "youtube", "youtube-music"
    Status     Status
    Error      string    // human-readable error if Status == StatusFailed
    OutputPath string    // set after successful download
}

type Error struct {
    Code    ErrorCode
    Message string
    Track   string  // URL of the track that failed, empty if pre-flight
}

type ErrorCode int

const (
    ErrorGeneric        ErrorCode = iota
    ErrorNetwork
    ErrorInvalidURL
    ErrorBinaryNotFound
    ErrorTrackUnavailable
    ErrorAgeRestricted
    ErrorDiskFull
)
```

---

## Screen flow (state machine)

### Transition diagram

```
                    ┌──────────────────────────────────────────┐
                    │                                          │
                    ▼                                          │
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────────┐ │
│  Input   │──►│Resolving │──►│ Playlist │──►│ Downloading  │─┘
│ (URL)    │   │(spinner)  │   │(select)  │   │(sequential)  │
└──────────┘   └──────────┘   └──────────┘   └──────────────┘
     ▲              │                              │
     │              ▼                              ▼
     │         ┌──────────┐                  ┌──────────┐
     │         │🛑 Error  │                  │   Done   │
     │         │ (inline)  │                  │(summary) │
     │         └──────────┘                  └──────────┘
     │                                            │
     └────────────────────────────────────────────┘
```

### Screen details

| Screen | State entry | User action | Transition | Error behavior |
| --- | --- | --- | --- | --- |
| **Input** | `screen: input` | User types/pastes URL | Enter → Resolving | Invalid URL format detected before transition. Show inline error, stay on Input, let user correct. |
| **Resolving** | `screen: resolving` | Spinner + "Resolving URL..." | OK → Playlist; Error → Input | If resolve fails (network, private video), show error inline with option to go back to Input via Esc. |
| **Playlist** | `screen: playlist` | List of tracks with selection checkboxes. j/k navigate, Space toggle, "a" select all, "n" select none. | Enter → Downloading; Esc → Input | If all tracks deselected, Enter does nothing (or shows hint). Empty playlist (0 tracks) → show message, Esc back. |
| **Downloading** | `screen: downloading` | Sequential download of selected tracks. Each shows "[artist - title] ... ✓/✗" as it completes. | Last track done → Done (auto); Esc → warning if downloads in progress | Per-track failure: mark as StatusFailed, continue to next. Non-fatal errors (disk full) may stop the queue. |
| **Done** | `screen: done` | Summary: total, succeeded, failed. List of failed tracks with error messages. | Enter → Input (new download); q → quit | N/A (terminal screen). |

### State model in TUI

```go
type Screen int

const (
    ScreenInput Screen = iota
    ScreenResolving
    ScreenPlaylist
    ScreenDownloading
    ScreenDone
)

type Model struct {
    // Navigation
    Screen      Screen
    PrevScreen  Screen
    Width       int
    Height      int

    // Input
    InputText   string
    InputError  string

    // Resolving
    ResolvingError string

    // Playlist
    Tracks      []domain.Media
    Cursor      int
    Scroll      int

    // Downloading
    CurrentDownload int       // index into Tracks being downloaded
    DownloadComplete bool     // all done, transition ready

    // Done
    TotalTracks int
    Succeeded   int
    Failed      int
    FailedTracks []domain.Media
}
```

### Edge cases covered

| Edge case | Behavior |
| --- | --- |
| **Empty URL** | Enter does nothing (or shows "paste a URL first" hint in footer) |
| **Non-YouTube URL** | Input screen validates with regex before transition. Shows inline error: "Only YouTube and YouTube Music URLs are supported in MVP." |
| **Very long playlist (+100 tracks)** | Resolving shows spinner + estimated count. Playlist screen scrolls. No pagination in MVP. |
| **0 tracks after resolve** | Playlist screen shows "No tracks found at this URL" + Esc back to Input |
| **All tracks deselected, Enter pressed** | Footer hint or toast: "Select at least one track" |
| **yt-dlp dies mid-download** | Track marked as StatusFailed, next track starts. If all fail, transition to Done with 0 success. |
| **User presses q mid-download** | Confirmation prompt: "Download in progress. Quit anyway? (y/N)" |
| **Terminal resize** | All screens handle `tea.WindowSizeMsg`, recalculate layout. Minimum size gate: 80x24, otherwise show resize message. |
| **Esc on Downloading screen** | If nothing downloading yet (first track queued but not started), go back to Playlist. If downloads started, nothing happens (disable Esc during active download). |
| **URL with no metadata** | yt-dlp still returns a title. Use channel name as artist fallback per ADR-005. |
| **Duplicate filename** | yt-dlp overwrites by default. Documented limitation in MVP (ADR-005). |

---

## Package structure

```
music-downloader/
├── cmd/
│   └── music-dl/
│       └── main.go              ← Entrypoint, DI wiring, preflight
├── internal/
│   ├── core/
│   │   ├── domain/
│   │   │   └── media.go         ← Media, Status, Error types
│   │   ├── ports/
│   │   │   ├── searcher.go      ← Searcher interface
│   │   │   ├── downloader.go    ← Downloader interface
│   │   │   └── preflight.go     ← PreflightChecker interface
│   │   └── service/
│   │       ├── orchestrator.go  ← Compose Searcher + Downloader
│   │       └── orchestrator_test.go
│   ├── adapters/
│   │   ├── searcher/
│   │   │   ├── ytdlp.go         ← yt-dlp --flat-playlist --dump-json
│   │   │   ├── ytdlp_test.go    ← integration test (skip -short)
│   │   │   └── parse.go         ← JSON parsing helper
│   │   │   └── parse_test.go    ← unit test for parsing
│   │   ├── downloader/
│   │   │   ├── ytdlp.go         ← yt-dlp -x --audio-format mp3
│   │   │   └── ytdlp_test.go    ← integration test (skip -short)
│   │   ├── preflight/
│   │   │   ├── checker.go       ← exec.LookPath for yt-dlp + ffmpeg
│   │   │   └── checker_test.go  ← unit test (t.TempDir, fake PATH)
│   │   └── filesystem/
│   │       ├── output.go        ← output dir creation, file ops
│   │       └── output_test.go   ← unit test (t.TempDir)
│   └── tui/
│       ├── model.go             ← Screen enum, Model struct, Init
│       ├── update.go            ← Update loop, key routing, async cmds
│       ├── update_test.go       ← state transition tests
│       ├── view.go              ← Screen rendering: all 5 screens
│       ├── styles.go            ← Semantic color slots + Lipgloss styles
│       └── keys.go              ← Keybinding constants + help text
├── docs/
│   └── adr/                     ← ADR files (unchanged)
├── go.mod
└── go.sum
```

### Package import rules

| Package | Imports from |
| --- | --- |
| `cmd/music-dl` | `internal/core/ports`, `internal/core/service`, `internal/adapters/*`, `internal/tui` |
| `internal/tui` | `internal/core/service`, `internal/core/domain` |
| `internal/core/service` | `internal/core/ports`, `internal/core/domain` |
| `internal/core/ports` | `internal/core/domain` |
| `internal/core/domain` | (stdlib only) |
| `internal/adapters/*` | `internal/core/ports`, `internal/core/domain` |

---

## Test strategy

### Unit tests (core domain + logic)

| Test | Location | Pattern | External deps? |
| --- | --- | --- | --- |
| Domain types | `core/domain/` | Table-driven, construct and assert fields | No |
| Orchestrator service | `core/service/orchestrator_test.go` | Mock `Searcher` + `Downloader`, test compose logic | No |
| JSON parsing | `adapters/searcher/parse_test.go` | Table-driven with fixture JSON strings | No |
| Preflight checker | `adapters/preflight/checker_test.go` | `t.TempDir()` + `PATH` manipulation | No (uses fake PATH) |
| Filesystem output | `adapters/filesystem/output_test.go` | `t.TempDir()` | No |

### Integration tests (adapters)

| Test | Location | Pattern | Skip in -short? |
| --- | --- | --- | --- |
| yt-dlp resolver | `adapters/searcher/ytdlp_test.go` | Calls real yt-dlp with test URLs | Yes |
| yt-dlp downloader | `adapters/downloader/ytdlp_test.go` | Calls real yt-dlp, checks output file | Yes |

### TUI state transition tests

| Test | Location | Pattern |
| --- | --- | --- |
| Screen transitions | `tui/update_test.go` | Direct `Model.Update()` with `tea.KeyMsg`, assert `Model.Screen` and state changes |
| Error states | `tui/update_test.go` | Inject `resolveFailedMsg`, `downloadFailedMsg`, assert error display |
| Async message handling | `tui/update_test.go` | Send `resolveDoneMsg`, `downloadProgressMsg`, `downloadDoneMsg` to `Model.Update()` |

**Golden file tests** are out of scope for MVP. They may be added post-MVP for rendered view output.

---

## Risks and mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| **yt-dlp JSON format changes** | Searcher adapter breaks | Pin a minimum yt-dlp version in docs. Parse JSON with lenient field extraction (don't require all fields). |
| **Bubble Tea API changes** | TUI breaks | Use tagged release (`v1.3.10` in go.mod). Major upgrades via separate PR with migration guide. |
| **Circular dependencies in hexagonal layers** | Compile failure | Enforce strict import rules in CI (use `go vet` + custom import linter). |
| **Testing adapter code requires real yt-dlp** | CI needs yt-dlp installed | Document CI dependency. Integration tests skip with `-short`. |

---

## Rollback

Since all Go source was deleted, there is nothing to roll back to. If this proposal needs to be rolled back:

1. Revert `openspec/changes/skeleton-reboot/proposal.md` (git revert).
2. Update Engram to mark this change as `abandoned`.

No running code is affected — there is none.

---

## Success criteria

- [ ] `go build ./cmd/music-dl/` succeeds and produces a binary.
- [ ] Running the binary shows the Input screen.
- [ ] Pasting a valid YouTube URL resolves to a playlist/single track.
- [ ] User can select/deselect tracks and download sequentially.
- [ ] Downloaded files appear in `~/Music/music-dl/` as `{artist} - {title}.mp3`.
- [ ] Missing yt-dlp or ffmpeg shows a clear error before the TUI starts.
- [ ] Invalid URLs show an inline error on the Input screen.
- [ ] All unit tests pass with `go test -short ./internal/...`.
- [ ] Core packages have zero imports from adapters, TUI, or external libraries (stdlib only).
- [ ] `go vet ./...` passes with no warnings.

---

## Non-goals (explicitly out of scope for skeleton-reboot)

- Progress bars or detailed download progress feedback (post-MVP).
- Concurrent downloads (post-MVP).
- Queue persistence between sessions (post-MVP).
- CLI flags for batch download (post-MVP).
- Spotify, SoundCloud, or other source providers (post-MVP).
- Configurable file naming or output paths (post-MVP).
- Search-by-name functionality (post-MVP).
- Golden file tests for TUI rendering (post-MVP).
- CI/CD pipeline setup (separate change).

---

## Next steps

1. Approve this proposal.
2. Proceed to `sdd-spec` to write detailed specifications for each package.
3. `sdd-design` for specific architectural decisions (DI wiring, async message flow, yt-dlp output parsing).
4. `sdd-tasks` to break implementation into concrete work items.
5. `sdd-apply` to write the actual Go code.
