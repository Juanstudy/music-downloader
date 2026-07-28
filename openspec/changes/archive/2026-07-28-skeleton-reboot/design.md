# Skeleton Reboot — Detailed Design

**Change:** `skeleton-reboot`
**Driver:** @juan-arch  
**Status:** Design  
**Design Decisions Cover:** Async message flow, DI wiring, yt-dlp JSON parsing, sequential download orchestration, TUI state machine edge cases, package-level decisions.

---

## Table of Contents

1. [Async Message Flow & TUI Command Patterns](#1-async-message-flow--tui-command-patterns)
2. [DI Wiring in main.go](#2-di-wiring-in-maingo)
3. [yt-dlp JSON Parsing](#3-yt-dlp-json-parsing)
4. [Sequential Download Orchestration](#4-sequential-download-orchestration)
5. [TUI State Machine & Edge Cases](#5-tui-state-machine--edge-cases)
6. [Package-Level Design Decisions](#6-package-level-design-decisions)
7. [File-by-File Summary](#7-file-by-file-summary)
8. [Open Questions for sdd-tasks](#8-open-questions-for-sdd-tasks)

---

## 1. Async Message Flow & TUI Command Patterns

### 1.1 Message Types (`internal/tui/messages.go`)

```go
// resolveFinishedMsg is sent when the URL resolution goroutine completes.
type resolveFinishedMsg struct {
    tracks []domain.Media
    err    error
}

// trackDownloadedMsg is sent after each individual track download completes.
// The TUI chains one download per message; Update() triggers the next track.
type trackDownloadedMsg struct {
    index int       // index into Model.Tracks
    media domain.Media
    err   error
}
```

**Design decision:** Two message types only for MVP. No `downloadProgressMsg` (reserved post-MVP). The sequential nature means the TUI drives the loop — each `trackDownloadedMsg` triggers the next `downloadTrackCmd` in `Update()`.

### 1.2 Command Construction

#### resolveCmd

```go
// resolveCmd creates a tea.Cmd that calls Searcher via the Orchestrator.
// It runs in a goroutine and sends a single resolveFinishedMsg back.
func resolveCmd(orchestrator *service.Orchestrator, url string) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()
        tracks, err := orchestrator.ResolveTrack(ctx, url)
        return resolveFinishedMsg{tracks: tracks, err: err}
    }
}
```

**Stale-message guard:** Update checks `m.Screen == ScreenResolving` before processing `resolveFinishedMsg`. If the user pressed Esc during resolve, the screen is `ScreenInput` and the message is silently dropped. This avoids the need to store a `context.CancelFunc` on the Model.

#### downloadTrackCmd

```go
// downloadTrackCmd downloads one track and sends trackDownloadedMsg.
func downloadTrackCmd(o *service.Orchestrator, track domain.Media, outputDir string, index int) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()
        updated, err := o.DownloadTrack(ctx, track, outputDir)
        return trackDownloadedMsg{index: index, media: updated, err: err}
    }
}
```

### 1.3 Update() Flow for Async Messages

```
resolveFinishedMsg:
  1. If m.Screen != ScreenResolving → drop (stale)
  2. If err != nil → set ResolvingError, screen = ScreenInput, return
  3. If len(tracks) == 0 → set hint, screen = ScreenPlaylist (empty state)
  4. Set tracks, screen = ScreenPlaylist

trackDownloadedMsg:
  1. Update m.Tracks[msg.index] with msg.media (sets Status/OutputPath/Error)
  2. Increment counters (Succeeded/Failed)
  3. m.CurrentDownload = msg.index + 1
  4. If msg.index+1 < len(selectedTracks) → start next track with downloadTrackCmd
  5. Else → screen = ScreenDone, return nil cmd
```

### 1.4 Esc/Cancel Design

| Scenario | Behavior |
| --- | --- |
| **Esc during Resolving** | `m.Screen = ScreenInput`. The resolve goroutine continues but its `resolveFinishedMsg` is dropped by the stale check. |
| **q during Downloading** | `m.ConfirmingQuit = true`. Second `q` or `y` → `tea.Quit`. `n` or `Esc` → `m.ConfirmingQuit = false`. |
| **Esc on Input screen** | `tea.Quit` (terminal confirmation: "Quit? (y/N)" can be added post-MVP). |

---

## 2. DI Wiring in main.go

### 2.1 Constructor Call Sequence

```go
package main

import (
    "context"
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"

    "github.com/Juanstudy/music-downloader/internal/adapters/downloader"
    "github.com/Juanstudy/music-downloader/internal/adapters/filesystem"
    "github.com/Juanstudy/music-downloader/internal/adapters/preflight"
    "github.com/Juanstudy/music-downloader/internal/adapters/searcher"
    "github.com/Juanstudy/music-downloader/internal/core/service"
    "github.com/Juanstudy/music-downloader/internal/tui"
)

func main() {
    ctx := context.Background()

    // Step 1: Preflight — check yt-dlp and ffmpeg before anything else
    checker := preflight.NewChecker("yt-dlp", "ffmpeg")
    if errs := checker.Check(ctx); len(errs) > 0 {
        for _, err := range errs {
            fmt.Fprintf(os.Stderr, "❌ %s not found: install it via 'brew install %s' or 'pip install %s'\n",
                err.Binary, err.Binary, err.Binary)
        }
        os.Exit(1)
    }

    // Step 2: Filesystem — ensure output directory exists
    output, err := filesystem.NewOutput("~/Music/music-dl")
    if err != nil {
        fmt.Fprintf(os.Stderr, "❌ Invalid output path: %v\n", err)
        os.Exit(1)
    }
    if err := output.Ensure(ctx); err != nil {
        fmt.Fprintf(os.Stderr, "❌ Cannot create output directory: %v\n", err)
        os.Exit(1)
    }

    // Step 3: Build adapters (no config needed in MVP — yt-dlp defaults)
    s := searcher.NewSearcher()
    d := downloader.NewDownloader()

    // Step 4: Build core service (composes ports via DI)
    orchestrator := service.NewOrchestrator(s, d)

    // Step 5: Build TUI model
    model := tui.NewModel(orchestrator, output.FullPath())

    // Step 6: Start Bubble Tea program
    p := tea.NewProgram(model, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "❌ TUI error: %v\n", err)
        os.Exit(1)
    }
}
```

### 2.2 Adapter Constructors

| Constructor | Signature | Behavior |
| --- | --- | --- |
| `preflight.NewChecker(binaries ...string)` | `func NewChecker(binaries ...string) *Checker` | Stores binary list (default `["yt-dlp", "ffmpeg"]`). `Check()` runs `exec.LookPath` for each. |
| `filesystem.NewOutput(basePath string)` | `func NewOutput(basePath string) (*Output, error)` | Expands tilde via `os.UserHomeDir()`, resolves to absolute path via `filepath.Abs()`. Errors on non-existent home dir. |
| `searcher.NewSearcher()` | `func NewSearcher() *Searcher` | No config — stores default binary path `"yt-dlp"`. |
| `downloader.NewDownloader()` | `func NewDownloader() *Downloader` | No config — stores default binary path `"yt-dlp"`. |
| `service.NewOrchestrator(s ports.Searcher, d ports.Downloader)` | `func NewOrchestrator(s ports.Searcher, d ports.Downloader) *Orchestrator` | Stores interfaces. No config. |
| `tui.NewModel(o *service.Orchestrator, outputDir string)` | `func NewModel(o *service.Orchestrator, outputDir string) Model` | Returns `Model` value (Bubble Tea convention). Initial screen is `ScreenInput`. |

### 2.3 Error Handling at Each Wiring Step

| Step | Error | Behavior |
| --- | --- | --- |
| Preflight.Check | Binary not found | Print per-binary error, `os.Exit(1)` |
| filesystem.NewOutput | Empty path, bad tilde | `fmt.Fprintf + os.Exit(1)` |
| filesystem.Ensure | Permission denied, no space | `fmt.Fprintf + os.Exit(1)` |
| tea.NewProgram | Terminal issue | `fmt.Fprintf + os.Exit(1)` from `p.Run()` |

**Decision:** Fail-fast before TUI starts. The TUI only initializes when the environment is ready. This avoids the user reaching a screen that can't function.

---

## 3. yt-dlp JSON Parsing

### 3.1 Parsing Architecture

`adapters/searcher/` has two files:

```
adapters/searcher/
├── ytdlp.go     ← exec.Command, JSON line collection, error mapping
└── parse.go     ← pure JSON line → domain.Media (no exec, no io)
```

**Separation rationale:**

- `parse.go` is a pure function: `ParseLine(line string) (domain.Media, error)`. Testable with string fixtures, no external deps.
- `ytdlp.go` calls `exec.Command`, reads stdout line by line, calls `ParseLine` per line, aggregates results. Integration test requires real yt-dlp.

### 3.2 Raw yt-dlp JSON Structure

For `--flat-playlist --dump-json`, each JSON line has:

```json
{
  "id": "dQw4w9WgXcQ",
  "title": "Never Gonna Give You Up",
  "duration": 212.0,
  "channel": "Rick Astley",
  "channel_id": "UCuAXFkgsw1L7xaCfnd5JJOw",
  "uploader": "Rick Astley",
  "creator": "Rick Astley",
  "webpage_url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  "thumbnail": "https://i.ytimg.com/vi/dQw4w9WgXcQ/maxresdefault.jpg",
  "playlist": null,
  "playlist_index": null
}
```

Single video URL also emits one JSON line (same format). For private/deleted/age-restricted videos with `--ignore-errors`, yt-dlp may omit that video from output (no error line emitted in `--flat-playlist` mode).

### 3.3 ParseLine Implementation (`parse.go`)

```go
package searcher

import (
    "encoding/json"
    "time"

    "github.com/Juanstudy/music-downloader/internal/core/domain"
)

// rawTrack is the intermediate struct for yt-dlp JSON decoding.
// Uses pointers for nullable fields (playlist_index can be null).
type rawTrack struct {
    ID          string  `json:"id"`
    Title       string  `json:"title"`
    Duration    float64 `json:"duration"`
    Channel     string  `json:"channel"`
    Uploader    string  `json:"uploader"`
    Creator     string  `json:"creator"`
    WebpageURL  string  `json:"webpage_url"`
    Thumbnail   string  `json:"thumbnail"`
    Playlist    *string `json:"playlist"`
    PlaylistIdx *int    `json:"playlist_index"`

    // Unknown fields are silently dropped (lenient)
}

// ParseLine parses one line of yt-dlp --dump-json output into domain.Media.
// Returns error only on JSON syntax errors or missing critical fields.
func ParseLine(line string) (domain.Media, error) {
    var raw rawTrack
    if err := json.Unmarshal([]byte(line), &raw); err != nil {
        return domain.Media{}, err
    }

    m := domain.Media{
        URL:      raw.WebpageURL,
        Title:    raw.Title,
        Duration: time.Duration(raw.Duration * float64(time.Second)),
        Source:   "youtube",
        Status:   domain.StatusPending,
    }

    // Artist extraction: channel > uploader > creator
    switch {
    case raw.Channel != "":
        m.Artist = raw.Channel
    case raw.Uploader != "":
        m.Artist = raw.Uploader
    default:
        m.Artist = raw.Creator
    }

    // Fallback for single-video URLs: use ID to construct URL if webpage_url missing
    if m.URL == "" && raw.ID != "" {
        m.URL = "https://www.youtube.com/watch?v=" + raw.ID
    }

    return m, nil
}
```

### 3.4 Field Extraction Priority (ADR-005)

| Priority | Field | Source | Example |
| --- | --- | --- | --- |
| 1st | `channel` | Channel name | "Rick Astley" |
| 2nd | `uploader` | Uploader name (often same as channel) | "Rick Astley" |
| 3rd | `creator` | Creator field (least populated) | "Rick Astley" |
| Fallback | `""` | Empty string if none available | (shows as "Unknown Artist" in UI) |

### 3.5 Duration Handling

- yt-dlp emits `duration` as `float64` seconds (e.g., `212.0`).
- Convert: `time.Duration(raw.Duration * float64(time.Second))`.
- Missing/zero duration = `0s` (display as "--:--" in TUI).

### 3.6 Lenient Parsing Principle

- Use `json.Unmarshal` into the struct — unknown yt-dlp fields are silently dropped.
- Only fail on JSON syntax errors or missing `id` + `webpage_url` (critical for download).
- Missing `title` → empty string (still downloadable, shows URL as fallback).
- Missing `duration` → `0s` (display "Unknown").
- This follows ADR-005: "pin minimum yt-dlp version in docs, parse leniently."

---

## 4. Sequential Download Orchestration

### 4.1 Service Interface

```go
// internal/core/service/orchestrator.go

package service

import (
    "context"
    "strings"

    "github.com/Juanstudy/music-downloader/internal/core/domain"
    "github.com/Juanstudy/music-downloader/internal/core/ports"
)

type Orchestrator struct {
    searcher   ports.Searcher
    downloader ports.Downloader
}

func NewOrchestrator(s ports.Searcher, d ports.Downloader) *Orchestrator {
    return &Orchestrator{searcher: s, downloader: d}
}

// ResolveTrack validates the URL and delegates to Searcher.
func (o *Orchestrator) ResolveTrack(ctx context.Context, url string) ([]domain.Media, error) {
    url = strings.TrimSpace(url)
    if url == "" {
        return nil, domain.Error{
            Code:    domain.ErrorInvalidURL,
            Message: "URL cannot be empty",
        }
    }
    if !isSupportedURL(url) {
        return nil, domain.Error{
            Code:    domain.ErrorInvalidURL,
            Message: "Only YouTube and YouTube Music URLs are supported",
        }
    }

    result, err := o.searcher.Search(ctx, url)
    if err != nil {
        return nil, err
    }

    for i := range result.Tracks {
        result.Tracks[i].Status = domain.StatusResolved
        result.Tracks[i].Source = result.Source
    }
    return result.Tracks, nil
}

// DownloadTrack downloads a single track and returns the updated Media.
func (o *Orchestrator) DownloadTrack(ctx context.Context, media domain.Media, outputDir string) (domain.Media, error) {
    media.Status = domain.StatusDownloading

    result, err := o.downloader.Download(ctx, media, outputDir)
    if err != nil {
        media.Status = domain.StatusFailed
        media.Error = err.Error()
        return media, err
    }

    media.Status = domain.StatusDone
    media.OutputPath = result.OutputPath
    return media, nil
}
```

### 4.2 URL Validation (`isSupportedURL`)

```go
// isSupportedURL validates that the URL is from YouTube or YouTube Music.
func isSupportedURL(url string) bool {
    if strings.HasPrefix(url, "https://www.youtube.com/") ||
        strings.HasPrefix(url, "https://youtube.com/") ||
        strings.HasPrefix(url, "https://youtu.be/") ||
        strings.HasPrefix(url, "https://music.youtube.com/") ||
        strings.HasPrefix(url, "https://www.youtube.com/watch") {
        return true
    }
    return false
}
```

**Decision:** Use `strings.HasPrefix` (simpler, testable) instead of regex. Covers all MVP cases. Post-MVP can add more patterns.

### 4.3 TUI-Driven Sequential Loop

The TUI does NOT consume a channel. Instead, it chains commands:

```
User presses Enter on Playlist screen
  ↓
Update() sets m.Screen = ScreenDownloading
  ↓
Update() returns downloadTrackCmd(orchestrator, tracks[0], outputDir, 0)
  ↓
tea.Batch runs the command in a goroutine
  ↓
trackDownloadedMsg{index: 0, media: updated, err: nil} sent to Update()
  ↓
Update() updates track 0, increments counter
  ↓
if index+1 < len(selected tracks) → downloadTrackCmd(orchestrator, tracks[1], ...)
  ↓
... repeat until done ...
  ↓
Last track → Update() sets m.Screen = ScreenDone, returns nil
```

**Rationale for chained commands over channel consumption:**

- Each `trackDownloadedMsg` is a single Bubble Tea event that Update processes synchronously with the model.
- The TUI can update per-track status immediately.
- Cancellation is clean: no goroutine is iterating a channel; just stop chaining.
- Progress-indicator friendly: between chained commands, the TUI shows the current state.

### 4.4 Channel-Based Alternative (Optional, for TUI independence)

The Orchestrator also exposes a channel-based version for future use (e.g., batch CLI mode):

```go
func (o *Orchestrator) DownloadTracks(ctx context.Context, tracks []domain.Media, outputDir string) <-chan domain.Media {
    ch := make(chan domain.Media)
    go func() {
        defer close(ch)
        for _, track := range tracks {
            select {
            case <-ctx.Done():
                return
            default:
            }
            updated, _ := o.DownloadTrack(ctx, track, outputDir)
            select {
            case <-ctx.Done():
                return
            case ch <- updated:
            }
        }
    }()
    return ch
}
```

**Decision:** Not used by TUI in MVP. Included in the Orchestrator for completeness and extensibility.

---

## 5. TUI State Machine & Edge Cases

### 5.1 Complete Screen Transition Map

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  Input ──(Enter + valid URL)──► Resolving ──(resolve OK)──► Playlist
│   ▲                                │                            │
│   │                                │ (Esc)                      │ (Enter
│   │                                ▼                            │  + selected
│   │◄─────────────────────────── Input (ignore stale resolve)     │  tracks)
│   │                                                             ▼
│   │◄────────────── (Enter) ──────────────────────────────── Downloading
│   │                                                             │
│   │                                                             │ (all done)
│   │                                                             ▼
│   │◄────────────────────────────── (Enter) ────────────────  Done
│   │                                                             │
│   │                                                             │ (q)
│   └─────────────────────────────────────────────────────────────┘ quit
│
│  Any screen: Ctrl+C → quit (handled by Bubble Tea)
│  Input: Esc → quit
│  Resolving: Esc → back to Input
│  Playlist: Esc → back to Input
│  Downloading: q → confirm dialog → quit
│  Done: q → quit
│
└──────────────────────────────────────────────────────────────────┘
```

### 5.2 Edge Case Handling

#### Esc During Resolving

```
User presses Esc while resolvePending goroutine is running.
→ m.Screen = ScreenInput (immediate)
→ Goroutine sends resolveFinishedMsg to Update()
→ Update checks: m.Screen == ScreenResolving? No → drop message
→ Result: stale resolve result discarded, user is back at Input
```

#### q During Downloading (Confirmation Dialog)

```go
case tea.KeyMsg:
    if m.Screen == ScreenDownloading && key == "q" {
        if !m.ConfirmingQuit {
            m.ConfirmingQuit = true
            return m, nil
        }
        // Second q or y confirms
        return m, tea.Quit
    }
    if m.ConfirmingQuit {
        if key == "n" || key == "esc" {
            m.ConfirmingQuit = false
        }
        return m, nil
    }
```

The confirmation renders as an overlay on the Downloading screen:

```
 Download in progress...

 [artist - title] ... downloading

 Quit? (y/N)
```

#### Terminal Resize Below 80×24

```go
case tea.WindowSizeMsg:
    m.Width = msg.Width
    m.Height = msg.Height
    if m.Width < 80 || m.Height < 24 {
        // Show "terminal too small" message (View handles this)
    }
    return m, nil
```

View checks:

```go
if m.Width < 80 || m.Height < 24 {
    return renderResizeMessage(m)
}
```

Resize message: centered on screen, "Terminal too small. Resize to at least 80×24."

#### All Tracks Deselected → Enter Shows Hint

```go
case tea.KeyMsg:
    if m.Screen == ScreenPlaylist && key == "enter" {
        count := countSelected(m.Tracks)
        if count == 0 {
            m.InputError = "Select at least one track with Space"
            return m, nil
        }
        // Proceed to download
    }
```

Hint appears as a status line in the footer, cleared on next keypress.

#### Empty Playlist After Resolve

```go
case resolveFinishedMsg:
    if len(msg.tracks) == 0 {
        m.ResolvingError = "No tracks found at this URL"
        m.Screen = ScreenInput
        return m, nil
    }
```

Alternatively, could show a dedicated "no tracks" message on Playlist screen with an option to go back.

#### URL with No Metadata (No Title, No Artist, No Duration)

```go
// In TUI display:
title := track.Title
if title == "" {
    title = "(untitled)"
}
artist := track.Artist
if artist == "" {
    artist = "Unknown Artist"
}
```

#### URL Format Invalid (Shows Inline Error)

```go
case tea.KeyMsg:
    if m.Screen == ScreenInput && key == "enter" {
        url := strings.TrimSpace(m.InputText)
        if url == "" {
            m.InputError = "Paste a YouTube URL first"
            return m, nil
        }
        if !isValidURL(url) {
            m.InputError = "Only YouTube and YouTube Music URLs are supported"
            return m, nil
        }
        // Proceed to resolve
    }
```

Inline error clears on next keypress.

#### yt-dlp Dies Mid-Download

- Track marked as `StatusFailed`, error message stored in `Media.Error`.
- `trackDownloadedMsg` carries the error.
- Update handles it like any other failure: mark failed, increment counters, start next track.
- If ALL tracks fail: `Succeeded = 0, Failed = len(selected)`, screen transitions to Done showing zero success.

#### Duplicate Filename

- yt-dlp overwrites by default. Documented limitation per ADR-005.
- The TUI does NOT check for filename collisions in MVP.

### 5.3 Full Model Struct

```go
type Model struct {
    // Injected dependencies
    orchestrator *service.Orchestrator
    outputDir    string

    // Navigation
    Screen      Screen
    PrevScreen  Screen
    Width       int
    Height      int

    // Input screen
    InputText   string
    InputError  string      // shown as inline error, cleared on next keypress

    // Resolving screen
    ResolvingError string   // shown if resolve failed

    // Playlist screen
    Tracks      []domain.Media
    Cursor      int         // tracks selection index

    // Downloading screen
    CurrentDownload int     // index of track currently being downloaded
    ConfirmingQuit bool     // show quit confirmation overlay

    // Done screen
    Succeeded   int
    Failed      int
    FailedTracks []domain.Media
}
```

---

## 6. Package-Level Design Decisions

### 6.1 parse.go vs ytdlp.go Separation

| File | Responsibility | Dependencies | Testability |
| --- | --- | --- | --- |
| `parse.go` | `ParseLine(line string) (domain.Media, error)` | `encoding/json`, `core/domain` | Pure unit test (string input → struct assert) |
| `ytdlp.go` | `Search(ctx, url)` → collect stdout → parse lines | `os/exec`, `context`, `parse.go` | Integration test (skip `-short`) |

**Decision:** `parse.go` is internal to the `searcher` package. Not exported. `ytdlp.go` imports `parse.go` internally.

### 6.2 Searcher and Downloader Binary Path Config

Both adapters default to `"yt-dlp"` on `$PATH`. The searcher uses `yt-dlp --flat-playlist --dump-json --ignore-errors`. The downloader uses `yt-dlp -x --audio-format mp3 --embed-metadata`.

```go
// adapters/searcher/ytdlp.go
const defaultBinary = "yt-dlp"

type Searcher struct {
    binary string
}

func NewSearcher() *Searcher {
    return &Searcher{binary: defaultBinary}
}

// adapters/downloader/ytdlp.go
type Downloader struct {
    binary string
}

func NewDownloader() *Downloader {
    return &Downloader{binary: defaultBinary}
}
```

**Decision:** No shared config struct. Each adapter has its own `binary` field with the same default. If post-MVP we need configurable binary paths, we add a `Config` struct. YAGNI applies here.

### 6.3 Filesystem Adapter: Tilde Expansion & Relative Paths

```go
package filesystem

import (
    "os"
    "path/filepath"
    "strings"
)

type Output struct {
    basePath string
}

func NewOutput(basePath string) (*Output, error) {
    if basePath == "" {
        return nil, errors.New("output path cannot be empty")
    }

    // Tilde expansion
    if strings.HasPrefix(basePath, "~/") {
        home, err := os.UserHomeDir()
        if err != nil {
            return nil, fmt.Errorf("cannot expand tilde: %w", err)
        }
        basePath = filepath.Join(home, basePath[2:])
    }

    // Resolve to absolute path (handles relative paths)
    abs, err := filepath.Abs(basePath)
    if err != nil {
        return nil, fmt.Errorf("cannot resolve path: %w", err)
    }

    return &Output{basePath: abs}, nil
}

func (o *Output) FullPath() string {
    return o.basePath
}

func (o *Output) Ensure(ctx context.Context) error {
    return os.MkdirAll(o.basePath, 0755)
}
```

**Decision:** Tilde expansion done at construction time. The `Output` struct stores the fully expanded absolute path. `Ensure()` is idempotent (uses `MkdirAll`).

### 6.4 Preflight Checker Binary List

```go
type Checker struct {
    binaries []string
}

func NewChecker(binaries ...string) *Checker {
    return &Checker{binaries: binaries}
}

func (c *Checker) Check(ctx context.Context) []ports.PreflightError {
    var errs []ports.PreflightError
    for _, b := range c.binaries {
        if _, err := exec.LookPath(b); err != nil {
            errs = append(errs, ports.PreflightError{Binary: b, Err: err})
        }
    }
    return errs
}
```

**Decision:** Binary list is configurable via variadic `NewChecker` constructor. `main.go` passes `"yt-dlp", "ffmpeg"`. Post-MVP, another binary can be added without changing the checker code.

### 6.5 Searcher yt-dlp Flags

```go
// Search invokes yt-dlp --flat-playlist --dump-json --ignore-errors.
// --ignore-errors: skip unavailable videos in playlists instead of failing.
func (s *Searcher) Search(ctx context.Context, url string) (ports.SearchResult, error) {
    args := []string{
        "--flat-playlist",
        "--dump-json",
        "--ignore-errors",
        url,
    }
    cmd := exec.CommandContext(ctx, s.binary, args...)
    // ...
}
```

**Rationale for `--ignore-errors`:**

- In playlist mode, a single unavailable video should not fail the whole resolve.
- Without it, `yt-dlp --flat-playlist --dump-json` stops at the first private/deleted video.
- With it, unavailable videos are simply omitted from JSON output.

### 6.6 Downloader yt-dlp Flags

```go
// Download invokes:
//   yt-dlp -x --audio-format mp3 --embed-metadata -o "{artist} - {title}.mp3" <url>
func (d *Downloader) Download(ctx context.Context, media domain.Media, url string) (ports.DownloadResult, error) {
    // Use the Media's URL for download, media.TrackURL if available
    url = media.URL

    outputTmpl := fmt.Sprintf("%s/%%(artist)s - %%(title)s.%%(ext)s", outputDir)

    args := []string{
        "-x",                       // extract audio
        "--audio-format", "mp3",    // convert to mp3
        "--embed-metadata",         // embed tags
        "--print", "filename",      // print final filename for detection
        "-o", outputTmpl,
        url,
    }
    cmd := exec.CommandContext(ctx, d.binary, args...)
    // Capture stdout for filename, stderr for progress
    // ...
}
```

**Output path detection:** Use `--print filename` which prints the final output path after download. This avoids having to parse yt-dlp's stderr output.

### 6.7 Preflight Checker Binary List

Default: `["yt-dlp", "ffmpeg"]`. Configurable via `NewChecker` variadic constructor.

### 6.8 Domain Types: Error vs ErrorCode

```go
type ErrorCode int

const (
    ErrorGeneric        ErrorCode = iota // 0
    ErrorNetwork                         // 1
    ErrorInvalidURL                      // 2
    ErrorBinaryNotFound                  // 3
    ErrorTrackUnavailable                // 4
    ErrorAgeRestricted                   // 5
    ErrorDiskFull                        // 6
)

type Error struct {
    Code    ErrorCode
    Message string
    Track   string // URL of the track that failed, empty if pre-flight/global
}
```

**Decision:** `domain.Error` implements the `error` interface via a value receiver:

```go
func (e Error) Error() string {
    return e.Message
}
```

This allows adapters to return `domain.Error` as a standard `error`, and callers can type-assert to check the code:

```go
if domainErr, ok := err.(domain.Error); ok && domainErr.Code == domain.ErrorBinaryNotFound {
    // handle missing binary
}
```

---

## 7. File-by-File Summary

```
music-downloader/
├── cmd/
│   └── music-dl/
│       └── main.go                       ← DI wiring (6 steps), preflight, error exit
├── internal/
│   ├── core/
│   │   ├── domain/
│   │   │   └── media.go                  ← Media, Status, Error, ErrorCode types
│   │   ├── ports/
│   │   │   ├── searcher.go               ← SearchResult, Searcher interface
│   │   │   ├── downloader.go             ← DownloadResult, Downloader interface
│   │   │   └── preflight.go              ← PreflightError, PreflightChecker interface
│   │   └── service/
│   │       ├── orchestrator.go           ← Orchestrator (ResolveTrack, DownloadTrack, DownloadTracks)
│   │       └── orchestrator_test.go      ← Mock Searcher + Downloader, table-driven
│   ├── adapters/
│   │   ├── searcher/
│   │   │   ├── ytdlp.go                  ← exec.Command, collect stdout, aggregate results
│   │   │   ├── ytdlp_test.go             ← integration (skip -short)
│   │   │   ├── parse.go                  ← ParseLine (pure JSON line → Media)
│   │   │   └── parse_test.go             ← table-driven with string fixtures
│   │   ├── downloader/
│   │   │   ├── ytdlp.go                  ← exec.Command, --print filename, detect output
│   │   │   └── ytdlp_test.go             ← integration (skip -short)
│   │   ├── preflight/
│   │   │   ├── checker.go                ← exec.LookPath for listed binaries
│   │   │   └── checker_test.go           ← t.TempDir + fake PATH
│   │   └── filesystem/
│   │       ├── output.go                 ← NewOutput (tilde expansion), Ensure, FullPath
│   │       └── output_test.go            ← t.TempDir
│   └── tui/
│       ├── model.go                      ← Screen enum, Model struct, Init, NewModel
│       ├── update.go                     ← Update loop, key routing, cmd chaining
│       ├── update_test.go                ← direct Model.Update() with tea.KeyMsg + custom msgs
│       ├── view.go                       ← 5 screen renderers + resize message
│       ├── styles.go                     ← semantic color slots via lipgloss.AdaptiveColor
│       ├── keys.go                       ← keybinding constants + help text per screen
│       └── messages.go                   ← resolveFinishedMsg, trackDownloadedMsg
├── docs/
│   └── adr/                              ← existing ADRs (unchanged)
├── go.mod
└── go.sum
```

### 7.1 `messages.go` (New File)

Purpose: centralize all custom `tea.Msg` types so they can be imported and referenced without circular deps.

```go
package tui

import (
    "github.com/Juanstudy/music-downloader/internal/core/domain"
)

type resolveFinishedMsg struct {
    tracks []domain.Media
    err    error
}

type trackDownloadedMsg struct {
    index int
    media domain.Media
    err   error
}
```

### 7.2 `styles.go` (New File)

```go
package tui

import (
    "github.com/charmbracelet/lipgloss"
)

// Semantic color slots via lipgloss.AdaptiveColor (dark/light auto adaptation).
// Maps directly to the table in ADR-006 §4.
var (
    colorDefault = lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#c0caf5"}
    colorMuted   = lipgloss.AdaptiveColor{Light: "#6c6c8a", Dark: "#565f89"}
    colorAccent  = lipgloss.AdaptiveColor{Light: "#2e7dff", Dark: "#7aa2f7"}
    colorSuccess = lipgloss.AdaptiveColor{Light: "#1b8a3d", Dark: "#9ece6a"}
    colorError   = lipgloss.AdaptiveColor{Light: "#cc241d", Dark: "#f7768e"}
    colorWarning = lipgloss.AdaptiveColor{Light: "#d79921", Dark: "#e0af68"}
    colorInfo    = lipgloss.AdaptiveColor{Light: "#458588", Dark: "#7dcfff"}

    // Prebuilt styles
    styleBase     = lipgloss.NewStyle().Foreground(colorDefault)
    styleMuted    = lipgloss.NewStyle().Foreground(colorMuted)
    styleAccent   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
    styleSuccess  = lipgloss.NewStyle().Foreground(colorSuccess)
    styleError    = lipgloss.NewStyle().Foreground(colorError)
    styleWarning  = lipgloss.NewStyle().Foreground(colorWarning)
    styleCursor   = lipgloss.NewStyle().Foreground(colorAccent).Background(lipgloss.Color("#364a82"))
    titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Padding(0, 1)
    errorStyle    = lipgloss.NewStyle().Foreground(colorError).Italic(true)
)
```

### 7.3 `keys.go` (New File)

```go
package tui

// keyHelp returns the contextual footer text for the active screen.
func keyHelp(screen Screen) string {
    switch screen {
    case ScreenInput:
        return "[Enter] resolve  [Esc] quit"
    case ScreenResolving:
        return "[Esc] cancel"
    case ScreenPlaylist:
        return "[j/k] navigate  [Space] toggle  [a] all  [n] none  [Enter] download  [Esc] back"
    case ScreenDownloading:
        return "[q] quit"
    case ScreenDone:
        return "[Enter] new download  [q] quit"
    default:
        return ""
    }
}
```

### 7.4 Test File Structure

| Test | File | Pattern | Deps |
| --- | --- | --- | --- |
| Domain types | `core/domain/media_test.go` | Table-driven field assertions | stdlib |
| Orchestrator | `core/service/orchestrator_test.go` | Mock Searcher + Downloader, table-driven scenarios | `core/domain`, `core/ports` (mocked) |
| JSON parsing | `adapters/searcher/parse_test.go` | Table-driven with `\n`-delimited fixture strings | `core/domain` |
| Preflight checker | `adapters/preflight/checker_test.go` | `t.TempDir()` + `os.Setenv("PATH", ...)` | stdlib |
| Filesystem output | `adapters/filesystem/output_test.go` | `t.TempDir()` | stdlib |
| yt-dlp searcher | `adapters/searcher/ytdlp_test.go` | Real yt-dlp call, skip with `-short` | real yt-dlp binary |
| yt-dlp downloader | `adapters/downloader/ytdlp_test.go` | Real yt-dlp call, skip with `-short` | real yt-dlp binary |
| TUI state transitions | `tui/update_test.go` | Direct `Model.Update(msg)`, assert `Model.Screen` + state fields | `core/service` (mock orchestrator) |

---

## 8. Open Questions for sdd-tasks

These are decisions deferred to implementation time (non-blocking for design, but tasks should explicitly address them):

1. **Spinner implementation**: Does the TUI use Bubble Tea's `tea.Spinner` or a custom lipgloss spinner rendered in view? Decision: use `bubbles/spinner` for consistency.

2. **Text input on Input screen**: Does the TUI use Bubble Tea's `tea.TextInput` (from bubbles) or a manual text buffer? Decision: use `bubbles/textinput` for cursor support and pasting.

3. **Help overlay rendering**: Is `?` handled via `tea.WindowSizeMsg` height-check overlay, or a separate screen? Decision: overlay via lipgloss.JoinVertical positioned on top of current view.

4. **Tick refresh during download**: Should the TUI use a periodic `tea.Tick` to update the view while downloading (for visual feedback), or just respond to `trackDownloadedMsg`? Decision: No tick needed in MVP — each completed track triggers a redraw via the message.

5. **Ctrl+C handling**: Bubble Tea handles `Ctrl+C` by default (sends SIGINT). Add explicit handler to restore terminal state if needed? Decision: Bubble Tea's default is sufficient — skip explicit handler.
