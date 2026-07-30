# Tasks: skeleton-reboot — Hexagonal Architecture Implementation

## Review Workload Forecast

| Field | Value |
| ------- | ------- |
| Estimated changed lines | ~1,800–2,000 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 → PR 4 → PR 5 → PR 6 |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |

```text
Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High
```

### Chain Architecture

```
feature/skeleton-reboot (accumulator branch)
├── PR #1  Core Domain Types + Port Interfaces     (~235 lines)
├── PR #2  Orchestrator Service                     (~320 lines)
├── PR #3  Pure Adapter Logic                       (~395 lines)
├── PR #4  yt-dlp Integration Adapters              (~290 lines)
├── PR #5  TUI Foundation + State Transition Tests  (~370 lines)
└── PR #6  TUI Update + View + Entrypoint           (~440 lines)
```

Each PR targets the feature branch (`feature/skeleton-reboot`). Only the feature branch merges to `main` when all PRs are approved and stacked. Each PR compiles, passes `go vet`, and its unit tests pass with `go test -short ./internal/...` independently.

All **parent ownership markers** (<!-- sdd-owner: parent -->) indicate actions the orchestrator/parent must perform (bounded review, approval, merge). All other tasks are owned by implementation.

---

## PR 1: Core Domain Types and Port Interfaces (~235 lines)

**Start state:** No `internal/` directory exists. `go.mod` with dependencies is present.

**End state:** `internal/core/domain/` and `internal/core/ports/` compile, `go test -short` passes, `go vet` passes.

**Verification:** `go build ./internal/core/... && go vet ./internal/core/... && go test -short ./internal/core/...`

**Rollback:** `git rm -r internal/core/domain/ internal/core/ports/ && git checkout HEAD -- go.mod`

---

### Task 1.1 — Write domain type tests (RED)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/core/domain/media_test.go` |
| Tests to write | See below |
| Dependencies | None (stdlib only) |
| Ownership | `implementation` |

**Test file:** `internal/core/domain/media_test.go`

| Test case | What it asserts |
| ----------- | ----------------- |
| `StatusPending` is 0, `StatusResolving` is 1, …, `StatusFailed` is 5 | Sequential iota values |
| All status constants are type `Status` | Compile-time typed const |
| `ErrorGeneric` is 0, `ErrorNetwork` is 1, …, `ErrorDiskFull` is 6 | Sequential iota values |
| All error code constants are type `ErrorCode` | Compile-time typed const |
| Zero-value `Media{}` has all empty/zero fields | `URL == ""`, `Status == StatusPending`, etc. |
| Media from struct literal returns all set values | Each field matches constructor |
| `domain.Error` implements `error` interface | `.Error()` returns the message string |
| `domain.Error` with Track set | Track field accessible and correct |

**Build verification:** `go test ./internal/core/domain/` — expected FAIL (no source file yet).

---

### Task 1.2 — Implement domain types to pass tests (GREEN)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/core/domain/media.go` |
| Dependencies | 1.1 (tests exist and define expected behaviour) |
| Ownership | `implementation` |

**What to implement:**

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
    URL, Title, Artist string
    Duration           time.Duration
    Source             string
    Status             Status
    Error, OutputPath  string
}

type ErrorCode int
const (
    ErrorGeneric ErrorCode = iota
    ErrorNetwork
    ErrorInvalidURL
    ErrorBinaryNotFound
    ErrorTrackUnavailable
    ErrorAgeRestricted
    ErrorDiskFull
)

type Error struct {
    Code    ErrorCode
    Message string
    Track   string
}

func (e Error) Error() string { return e.Message }
```

**Constraints:**

- Import stdlib only (`"time"`)
- No exported functions, constructors, or helper methods
- Types created via struct literals only

**Build verification:** `go test ./internal/core/domain/` PASS, `go vet ./internal/core/domain/` PASS.

---

### Task 1.3 — Define port interfaces (Searcher, Downloader, PreflightChecker)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/core/ports/searcher.go`, `internal/core/ports/downloader.go`, `internal/core/ports/preflight.go` |
| Dependencies | 1.2 (ports import domain types) |
| Ownership | `implementation` |

**`searcher.go`:**

```go
type SearchResult struct {
    Tracks []domain.Media
    Source string
}
type Searcher interface {
    Search(ctx context.Context, url string) (SearchResult, error)
}
```

**`downloader.go`:**

```go
type DownloadResult struct {
    Media      domain.Media
    OutputPath string
}
type Downloader interface {
    Download(ctx context.Context, media domain.Media, outputDir string) (DownloadResult, error)
}
```

**`preflight.go`:**

```go
type PreflightError struct {
    Binary string
    Err    error
}
type PreflightChecker interface {
    Check(ctx context.Context) []PreflightError
}
```

**Imports:** `"context"`, `core/domain`

**Build verification:** `go build ./internal/core/ports/` PASS, `go vet ./internal/core/ports/` PASS.

---

### Task 1.4 — Write port struct tests and compile-time checks

| Field | Value |
| ------- | ------- |
| Files to create | `internal/core/ports/searcher_test.go`, `internal/core/ports/downloader_test.go`, `internal/core/ports/preflight_test.go` |
| Dependencies | 1.3 |
| Ownership | `implementation` |

**Test cases:**

| File | Cases |
| ------ | ------- |
| `searcher_test.go` | Zero-value `SearchResult{}` has nil Tracks and empty Source |
| `downloader_test.go` | Zero-value `DownloadResult{}` has zero Media and empty OutputPath |
| `preflight_test.go` | Zero-value `PreflightError{}` has empty Binary and nil Err |

**Build verification:** `go test -short ./internal/core/...` PASS, `go vet ./internal/core/...` PASS.

---

### Task 1.5 — Start or reuse bounded review <!-- sdd-owner: parent -->

- [ ] Run bounded review on PR 1 diff. <!-- sdd-owner: parent -->
- [ ] Approve and merge PR 1 into `feature/skeleton-reboot`. <!-- sdd-owner: parent -->

---

## PR 2: Orchestrator Service (~320 lines)

**Start state:** PR 1 merged. `core/domain` and `core/ports` exist and compile.

**End state:** `core/service` compiles with full Orchestrator, passes tests with mock dependencies.

**Verification:** `go test -short ./internal/core/...` PASS, `go vet ./internal/core/...` PASS.

**Rollback:** `git rm -r internal/core/service/`.

**Checklist:**

- [x] Task 2.1 — Write orchestrator tests with mocks (RED)
- [x] Task 2.2 — Implement Orchestrator to pass tests (GREEN)

---

### Task 2.1 — Write orchestrator tests with mocks (RED)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/core/service/orchestrator_test.go` |
| Dependencies | PR 1 (domain + ports) |
| Ownership | `implementation` |

**Mock setup inside test file:**

```go
type mockSearcher struct {
    searchFunc func(ctx context.Context, url string) (ports.SearchResult, error)
}
func (m *mockSearcher) Search(ctx context.Context, url string) (ports.SearchResult, error) {
    return m.searchFunc(ctx, url)
}

type mockDownloader struct {
    downloadFunc func(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error)
}
func (m *mockDownloader) Download(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error) {
    return m.downloadFunc(ctx, media, outputDir)
}
```

**Table-driven test cases for `ResolveTrack`:**

| Case | Input | Mock behaviour | Expected |
| ------ | ------- | ---------------- | ---------- |
| Valid URL returns tracks | `"https://youtube.com/playlist?list=..."` | Returns 2 tracks | 2 Media items, each `Status == StatusResolved`, Source set |
| Searcher returns error | Any URL | Returns error | nil tracks, error propagated |
| Empty URL | `""` | Not called | `domain.Error{ErrorInvalidURL}` |
| Non-YouTube URL | `"not-a-valid-url"` | Not called | `domain.Error{ErrorInvalidURL}` |

**Table-driven test cases for `DownloadTracks`:**

| Case | Input | Mock behaviour | Expected |
| ------ | ------- | ---------------- | ---------- |
| All succeed | 3 tracks | Each succeeds | 3 Results on channel, all `StatusDone` |
| One fails in middle | 3 tracks | Second fails | 3 Results, second has error, third succeeds |
| Context cancelled | 3 tracks, cancel after first | — | Channel closes, remaining not processed |
| Empty tracks | `[]domain.Media{}` | — | Channel sends nothing, immediately closed |

**Sequential ordering test** (verify track N+1 doesn't start before track N completes):

Use a blocking mock that signals when it's called and waits for a Go signal from the test goroutine. This proves sequential processing.

**Build verification:** `go test ./internal/core/service/` — expected FAIL (no orchestrator.go yet).

---

### Task 2.2 — Implement Orchestrator to pass tests (GREEN)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/core/service/orchestrator.go` |
| Dependencies | 2.1 |
| Ownership | `implementation` |

**Functions to implement:**

```go
func NewOrchestrator(s ports.Searcher, d ports.Downloader) *Orchestrator
func (o *Orchestrator) ResolveTrack(ctx context.Context, url string) ([]domain.Media, error)
func (o *Orchestrator) DownloadTrack(ctx context.Context, media domain.Media, outputDir string) (domain.Media, error)
func (o *Orchestrator) DownloadTracks(ctx context.Context, tracks []domain.Media, outputDir string) <-chan Result
```

**`ResolveTrack` behaviour:**

1. `strings.TrimSpace(url)`, check empty → `domain.Error{ErrorInvalidURL}`
2. `isSupportedURL(url)` → `domain.Error{ErrorInvalidURL}` for non-YouTube URLs
3. Delegate to `o.searcher.Search(ctx, url)`
4. Set each track's `Status = StatusResolved` and `Source`
5. Return tracks

**`isSupportedURL` helper:**

```go
func isSupportedURL(url string) bool { ... }
// Match prefixes: https://www.youtube.com/, https://youtube.com/,
// https://youtu.be/, https://music.youtube.com/,
// https://www.youtube.com/watch
```

**`DownloadTrack` behaviour:**

1. Set `media.Status = StatusDownloading`
2. `o.downloader.Download(ctx, media, outputDir)`
3. On success: `Status = StatusDone`, `OutputPath = result.OutputPath`
4. On error: `Status = StatusFailed`, `Error = err.Error()`

**`DownloadTracks` behaviour:**

1. Create channel
2. Goroutine: iterate tracks, call `DownloadTrack` sequentially
3. Send each result on channel
4. Respect context cancellation (select on `<-ctx.Done()`)
5. Close channel when done

**Build verification:** `go test ./internal/core/service/` PASS, `go vet ./internal/core/service/` PASS. `go test -short ./internal/core/...` PASS.

---

### Task 2.3 — Start or reuse bounded review <!-- sdd-owner: parent -->

- [ ] Run bounded review on PR 2 diff. <!-- sdd-owner: parent -->
- [ ] Approve and merge PR 2 into `feature/skeleton-reboot`. <!-- sdd-owner: parent -->

---

## PR 3: Pure Adapter Logic (~395 lines)

**Start state:** PR 2 merged. `core/service` exists with Orchestrator.

**End state:** Three pure-logic adapter packages compile and pass unit tests with no external binary dependencies.

**Verification:** `go test -short ./internal/adapters/...` PASS, `go vet ./internal/adapters/...` PASS.

**Rollback:** `git rm -r internal/adapters/searcher/parse.go internal/adapters/preflight/ internal/adapters/filesystem/`.

**Checklist:**

- [x] Task 3.1 — Write JSON parse tests (RED)
- [x] Task 3.2 — Implement ParseLine to pass tests (GREEN)
- [x] Task 3.3 — Write preflight checker tests (RED)
- [x] Task 3.4 — Implement preflight checker (GREEN)
- [x] Task 3.5 — Write filesystem output tests (RED)
- [x] Task 3.6 — Implement filesystem output (GREEN)

---

### Task 3.1 — Write JSON parse tests (RED)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/searcher/parse_test.go` |
| Dependencies | PR 1 (domain types) |
| Ownership | `implementation` |

**Table-driven test cases:**

| Case | Input JSON | Expected |
| ------ | ----------- | ---------- |
| Complete JSON with all fields | `{"webpage_url":"...","title":"Never Gonna Give You Up","channel":"Rick Astley","duration":212.0,"id":"dQw4w9WgXcQ"}` | Fully populated Media, `StatusResolved` |
| Minimal JSON (url + title) | `{"webpage_url":"https://...","title":"Song"}` | Media with URL+Title, Artist="" |
| JSON with float duration | `{"webpage_url":"...","duration":180.5}` | `Duration ≈ 180.5s` |
| Invalid JSON | `"this is not json"` | Non-nil error |
| Channel field | `{"webpage_url":"...","channel":"Artist Channel"}` | Artist = "Artist Channel" |
| Uploader (no channel) | `{"webpage_url":"...","uploader":"Uploader Name"}` | Artist = "Uploader Name" |
| Creator (no channel/uploader) | `{"webpage_url":"...","creator":"Creator Name"}` | Artist = "Creator Name" |
| No artist fields | `{"webpage_url":"...","title":"Song"}` | Artist = "" |
| Missing webpage_url, has id | `{"id":"dQw4w9WgXcQ","title":"Song"}` | URL constructed from id |
| Zero duration | `{"webpage_url":"...","duration":0}` | Duration = 0 |

**Build verification:** `go test ./internal/adapters/searcher/` — expected FAIL (no parse.go yet).

---

### Task 3.2 — Implement ParseLine to pass tests (GREEN)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/searcher/parse.go` |
| Dependencies | 3.1 |
| Ownership | `implementation` |

**Implementation details:**

- Package `searcher` (shared with `ytdlp.go` in PR 4)
- Internal `rawTrack` struct with JSON tags for yt-dlp fields
- Artist extraction: `channel` → `uploader` → `creator` (first non-empty wins)
- Duration: `float64` seconds → `time.Duration`
- Missing `webpage_url` fallback: construct from `id`
- Lenient parsing: unknown fields silently dropped

**Build verification:** `go test ./internal/adapters/searcher/` PASS.

---

### Task 3.3 — Write preflight checker tests (RED)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/preflight/checker_test.go` |
| Dependencies | PR 1 (ports) |
| Ownership | `implementation` |

**Pattern:** `t.TempDir()` + `os.Setenv("PATH", tmpDir)`.

| Case | Setup | Expected |
| ------ | ------- | ---------- |
| Both binaries present | Create `yt-dlp` and `ffmpeg` scripts in TempDir, set PATH | Empty slice |
| Both missing | PATH = empty TempDir | 2 errors (yt-dlp, ffmpeg) |
| Only yt-dlp present | Create `yt-dlp` only | 1 error (ffmpeg) |
| Only ffmpeg present | Create `ffmpeg` only | 1 error (yt-dlp) |
| Empty binary list | `Checker{Binaries: []}` | Empty slice |
| Context cancelled | Pre-cancelled context | Returns immediately (accept error or empty) |

**Build verification:** `go test ./internal/adapters/preflight/` — expected FAIL.

---

### Task 3.4 — Implement preflight checker (GREEN)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/preflight/checker.go` |
| Dependencies | 3.3 |
| Ownership | `implementation` |

```go
type Checker struct{ binaries []string }

func NewChecker(binaries ...string) *Checker
func (c *Checker) Check(ctx context.Context) []ports.PreflightError
```

**Behaviour:**

- `Check` calls `exec.LookPath(binary)` for each configured binary
- Collects all errors (non fail-fast)
- Returns `[]ports.PreflightError`

**Build verification:** `go test ./internal/adapters/preflight/` PASS.

---

### Task 3.5 — Write filesystem output tests (RED)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/filesystem/output_test.go` |
| Dependencies | PR 1 |
| Ownership | `implementation` |

**Pattern:** `t.TempDir()` for all filesystem operations.

| Case | Setup | Expected |
| ------ | ------- | ---------- |
| `Ensure` creates directory | `Output{BasePath: filepath.Join(t.TempDir(), "sub", "dir")}` | Directory exists after `Ensure()`, no error |
| `Ensure` on existing directory | Create dir first, then `Ensure()` | No error |
| `FullPath` with absolute path | `Output{BasePath: t.TempDir()}` | Returns same absolute path |
| `FullPath` with tilde path | `Output{BasePath: "~/sub/dir"}` | Returns expanded path (uses `os.UserHomeDir()`) |
| `NewOutput` with empty path | `""` | Error returned |
| `NewOutput` with relative path | `"relative/path"` | Resolves to absolute via `filepath.Abs` |

**Build verification:** `go test ./internal/adapters/filesystem/` — expected FAIL.

---

### Task 3.6 — Implement filesystem output (GREEN)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/filesystem/output.go` |
| Dependencies | 3.5 |
| Ownership | `implementation` |

```go
type Output struct{ basePath string }

func NewOutput(basePath string) (*Output, error)
func (o *Output) FullPath() string
func (o *Output) Ensure(ctx context.Context) error
```

**Behaviour:**

- `NewOutput`: tilde expansion via `os.UserHomeDir()` + `filepath.Join`, resolve to absolute path
- `Ensure`: `os.MkdirAll(o.basePath, 0755)` (idempotent)
- No external dependencies beyond stdlib

**Build verification:** `go test ./internal/adapters/filesystem/` PASS, `go test -short ./internal/adapters/...` PASS, `go vet ./internal/adapters/...` PASS.

---

### Task 3.7 — Start or reuse bounded review <!-- sdd-owner: parent -->

- [ ] Run bounded review on PR 3 diff. <!-- sdd-owner: parent -->
- [ ] Approve and merge PR 3 into `feature/skeleton-reboot`. <!-- sdd-owner: parent -->

---

## PR 4: yt-dlp Integration Adapters (~290 lines)

**Start state:** PR 3 merged. `parse.go`, `preflight`, `filesystem` adapters exist.

**End state:** `searcher/ytdlp.go` and `downloader/ytdlp.go` compile. Integration tests exist and are skip-able with `-short`.

**Verification:** `go build ./internal/adapters/...`, `go vet ./internal/adapters/...`, `go test -short ./internal/adapters/...` PASS (skips integration).

**Rollback:** `git rm internal/adapters/searcher/ytdlp.go internal/adapters/searcher/ytdlp_test.go internal/adapters/downloader/ytdlp.go internal/adapters/downloader/ytdlp_test.go`.

---

### Task 4.1 — Implement yt-dlp searcher adapter

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/searcher/ytdlp.go` |
| Dependencies | 3.2 (parse.go in same package), PR 1 (domain, ports) |
| Ownership | `implementation` |

```go
type Searcher struct{ binary string }

func NewSearcher() *Searcher

func (s *Searcher) Search(ctx context.Context, url string) (ports.SearchResult, error)
```

**`Search` implementation:**

1. Build command: `exec.CommandContext(ctx, s.binary, "--flat-playlist", "--dump-json", "--ignore-errors", url)`
2. Capture stdout
3. Read line by line (scanner), call `ParseLine(line)` per line
4. Collect valid results, skip parse errors
5. Detect source from URL host (`music.youtube.com` → `"youtube-music"`, otherwise `"youtube"`)
6. Error mapping: `exec.ErrNotFound` → `domain.Error{ErrorBinaryNotFound}`, non-zero exit → `domain.Error{ErrorGeneric}` with stderr

**Build verification:** `go build ./internal/adapters/searcher/` PASS.

---

### Task 4.2 — Write yt-dlp searcher integration test

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/searcher/ytdlp_test.go` |
| Dependencies | 4.1 |
| Ownership | `implementation` |

**Skip condition:** `if testing.Short() { t.Skip("Skipping integration test") }`

| Case | URL | Expected |
| ------ | ----- | ---------- |
| Single video resolution | Valid public YouTube video URL | Returns 1 track |
| Playlist resolution | Valid YouTube playlist with ≥2 tracks | Returns ≥2 tracks |
| Invalid URL | `"https://example.com"` | Error returned |
| Empty URL | `""` | Error returned |

**Note:** Use well-known stable test URLs (e.g., yt-dlp maintainer's test videos). Document URL sources in comments.

---

### Task 4.3 — Implement yt-dlp downloader adapter

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/downloader/ytdlp.go` |
| Dependencies | PR 1 (domain, ports) |
| Ownership | `implementation` |

```go
type Downloader struct{ binary string }

func NewDownloader() *Downloader

func (d *Downloader) Download(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error)
```

**`Download` implementation:**

1. Build output template: `fmt.Sprintf("%s/%%(artist)s - %%(title)s.%%(ext)s", outputDir)`
2. Command: `exec.CommandContext(ctx, d.binary, "-x", "--audio-format", "mp3", "--embed-metadata", "--print", "filename", "-o", outputTmpl, media.URL)`
3. Capture `--print filename` stdout for actual output path
4. Error mapping: same as searcher (`ErrorBinaryNotFound`, `ErrorGeneric`)
5. On success: build `DownloadResult` with `StatusDone` and actual `OutputPath`

**Build verification:** `go build ./internal/adapters/downloader/` PASS.

---

### Task 4.4 — Write yt-dlp downloader integration test

| Field | Value |
| ------- | ------- |
| Files to create | `internal/adapters/downloader/ytdlp_test.go` |
| Dependencies | 4.3 |
| Ownership | `implementation` |

**Skip condition:** `if testing.Short() { t.Skip("Skipping integration test") }`

| Case | Input | Expected |
|------|-------|----------|
| Download single track | Valid resolved Media, t.TempDir output | Returns `DownloadResult` with `StatusDone`, file exists |
| Invalid URL | Media with bad URL | Non-nil error |

**Build verification:** `go test -short ./internal/adapters/...` PASS, `go vet ./internal/adapters/...` PASS.

**Checklist:**

- [x] Task 4.1 — Implement yt-dlp searcher adapter
- [x] Task 4.2 — Write yt-dlp searcher integration test
- [x] Task 4.3 — Implement yt-dlp downloader adapter
- [x] Task 4.4 — Write yt-dlp downloader integration test

---

### Task 4.5 — Start or reuse bounded review <!-- sdd-owner: parent -->

- [ ] Run bounded review on PR 4 diff. <!-- sdd-owner: parent -->
- [ ] Approve and merge PR 4 into `feature/skeleton-reboot`. <!-- sdd-owner: parent -->

---

## PR 5: TUI Foundation + State Transition Tests (~370 lines)

**Start state:** PR 4 merged. All adapters and core layers exist.

**End state:** `internal/tui/` package compiles with stub `Update()` and all transition tests written (some fail).

**Verification:** Package compiles. `go vet ./internal/tui/` PASS. `go test ./internal/tui/` may fail (expected RED).

**Rollback:** `git rm -r internal/tui/`.

**Checklist:**

- [x] Task 5.1 — Create TUI model, messages, styles, and keys
- [x] Task 5.2 — Write TUI state transition tests (RED, no Update yet)
- [x] Task 5.3 — Create stub update.go (compiles, tests fail)

---

### Task 5.1 — Create TUI model, messages, styles, and keys

| Field | Value |
| ------- | ------- |
| Files to create | `internal/tui/model.go`, `internal/tui/messages.go`, `internal/tui/styles.go`, `internal/tui/keys.go` |
| Dependencies | PR 2 (imports `service.Orchestrator` and `domain`) |
| Ownership | `implementation` |

**`model.go`:**

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
    orchestrator  *service.Orchestrator
    outputDir     string
    Screen, PrevScreen Screen
    Width, Height int
    InputText, InputError string
    ResolvingError string
    Tracks        []domain.Media
    Cursor        int
    CurrentDownload int
    ConfirmingQuit bool
    Succeeded, Failed int
    FailedTracks  []domain.Media
}

func NewModel(o *service.Orchestrator, outputDir string) Model
func (m Model) Init() tea.Cmd
```

**`messages.go`:**

```go
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

**`styles.go`:** All semantic style slots using `lipgloss.AdaptiveColor`. See design §7.2.

**`keys.go`:** Keybinding constants plus `keyHelp(screen Screen) string` helper. See design §7.3.

**Build verification:** `go build ./internal/tui/` PASS (with stub update.go from 5.3).

---

### Task 5.2 — Write TUI state transition tests (RED, no Update yet)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/tui/update_test.go` |
| Dependencies | 5.1 (skeleton update.go stub must exist for compilation) |
| Ownership | `implementation` |

**Test pattern:** Direct `Model.Update(msg)` calls with `tea.KeyMsg` and custom message sends. Use a real service.Orchestrator with mock searcher/downloader (reuse mock pattern from PR 2 or inline).

**Screen transition test cases (9 transitions):**

| Case | Msg | Initial | Expected screen | Expected side effects |
| ------ | ----- | --------- | ---------------- | ---------------------- |
| Enter with valid URL → Resolving | `tea.KeyMsg{Type: tea.KeyEnter}` | Input, non-empty URL | Resolving | Cmd returned (resolveCmd) |
| Esc on Input → Quit | `tea.KeyMsg{Type: tea.KeyEsc}` | Input | — | `tea.Quit` cmd returned |
| Resolve success → Playlist | `resolveFinishedMsg{tracks: [...]}` | Resolving | Playlist | `m.Tracks` set |
| Resolve failure → Input | `resolveFinishedMsg{err: ...}` | Resolving | Input | `m.InputError` set |
| Enter on Playlist → Downloading | `tea.KeyMsg{Type: tea.KeyEnter}` | Playlist, tracks selected | Downloading | Cmd returned |
| All done → Done | `trackDownloadedMsg` (last track) | Downloading | Done | Counters updated |
| Enter on Done → Input | `tea.KeyMsg{Type: tea.KeyEnter}` | Done | Input | Tracks cleared, counters reset |
| Empty URL stays Input | `tea.KeyMsg{Type: tea.KeyEnter}` | Input, empty URL | Input | `InputError` set |
| Esc on Playlist → Input | `tea.KeyMsg{Type: tea.KeyEsc}` | Playlist | Input | Tracks cleared |
| Esc on Resolving → Input | `tea.KeyMsg{Type: tea.KeyEsc}` | Resolving | Input | Cursor reset |

**Playlist selection tests:**

| Case | Msg | Initial | Expected |
| ------ | ----- | --------- | ---------- |
| Space toggles selection | `tea.KeyMsg{Type: tea.KeySpace}` | Track at cursor, StatusPending | StatusResolved |
| 'a' selects all | `tea.KeyMsg{Runes: []rune{'a'}}` | All pending | All resolved |
| 'n' selects none | `tea.KeyMsg{Runes: []rune{'n'}}` | All resolved | All pending |
| Enter with no selection | `tea.KeyMsg{Type: tea.KeyEnter}` | No selected tracks | Stay on Playlist, no cmd |

**Window resize tests:**

| Case | Msg | Expected |
|------|-----|----------|
| Resize updates dimensions | `tea.WindowSizeMsg{Width: 100, Height: 40}` | `m.Width == 100`, `m.Height == 40` |

**Ctrl+C quit tests (from every screen):**

| Case | Screen | Expected |
| ------ | -------- | ---------- |
| Ctrl+C on Input | Input | `tea.Quit` |
| Ctrl+C on Playlist | Playlist | `tea.Quit` |
| Ctrl+C on Downloading | Downloading | `tea.Quit` |
| Ctrl+C on Done | Done | `tea.Quit` |

**Build verification:** `go test ./internal/tui/` — test file compiles, tests may fail (expected RED).

---

### Task 5.3 — Create stub update.go (compiles, tests fail)

| Field | Value |
| ------- | ------- |
| Files to create | `internal/tui/update.go` |
| Dependencies | 5.1, 5.2 |
| Ownership | `implementation` |

**Stub implementation:**

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Stub: handle only WindowSizeMsg to set dimensions
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.Width = msg.Width
        m.Height = msg.Height
        return m, nil
    case tea.KeyMsg:
        if msg.Type == tea.KeyCtrlC {
            return m, tea.Quit
        }
    }
    return m, nil
}
```

**Purpose:** Package compiles. Transition tests fail because Update doesn't route properly yet. This is the intentional RED state.

**Build verification:** `go build ./internal/tui/` PASS, `go vet ./internal/tui/` PASS.

---

### Task 5.4 — Start or reuse bounded review <!-- sdd-owner: parent -->

- [ ] Run bounded review on PR 5 diff (TUI types, tests, stub). <!-- sdd-owner: parent -->
- [ ] Approve and merge PR 5 into `feature/skeleton-reboot`. <!-- sdd-owner: parent -->

---

## PR 6: TUI Update + View + Entrypoint (~440 lines)

**Start state:** PR 5 merged. TUI types exist, stub Update() compiles, transition tests exist.

**End state:** Full TUI works, `go build ./cmd/music-dl/` produces binary, `go test -short ./...` PASS.

**Verification:** `go build ./cmd/music-dl/`, `go vet ./...`, `go test -short ./internal/...`.

**Rollback:** `git checkout HEAD~1 -- internal/tui/ cmd/music-dl/`.

---

### Task 6.1 — Implement full Update() to pass transition tests (GREEN)

| Field | Value |
| ------- | ------- |
| Files to modify | `internal/tui/update.go` — replace stub with full implementation |
| Dependencies | 5.2 (tests exist), 5.1 (types) |
| Ownership | `implementation` |

**Implement the full `Update()` message loop:**

```
tea.KeyMsg → route by m.Screen:
  ScreenInput:
    - printable chars → append to InputText
    - backspace → remove last char
    - enter with empty URL → set InputError, stay
    - enter with invalid URL → set InputError, stay
    - enter with valid URL → ScreenResolving, return resolveCmd
    - esc → tea.Quit

  ScreenResolving:
    - esc → ScreenInput (stale guard: later resolveFinishedMsg dropped)
    - (no other keys meaningful during resolve)

  ScreenPlaylist:
    - j/↓ → cursor down
    - k/↑ → cursor up
    - space → toggle selection (StatusPending ↔ StatusResolved)
    - 'a' → select all
    - 'n' → select none
    - enter with 0 selected → set hint, stay
    - enter with ≥1 selected → ScreenDownloading, start download chain
    - esc → ScreenInput, clear tracks

  ScreenDownloading:
    - 'q' → toggle ConfirmingQuit, second q → tea.Quit
    - esc (CurrentDownload == 0) → ScreenPlaylist
    - esc (in progress, CurrentDownload > 0) → ignore

  ScreenDone:
    - enter → ScreenInput, reset state
    - 'q' → tea.Quit
    - esc → tea.Quit

tea.WindowSizeMsg → update m.Width, m.Height

resolveFinishedMsg:
  - if m.Screen != ScreenResolving → drop (stale)
  - if err → ScreenInput + ResolvingError
  - if len(tracks) == 0 → ScreenInput + "No tracks found"
  - else → ScreenPlaylist + tracks

trackDownloadedMsg:
  - update m.Tracks[msg.index] with result
  - increment Succeeded/Failed
  - m.CurrentDownload = msg.index + 1
  - if more tracks → return downloadTrackCmd for next
  - else → ScreenDone
```

**Async command functions:**

```go
func (m Model) resolveURL() tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()
        tracks, err := m.orchestrator.ResolveTrack(ctx, m.InputText)
        return resolveFinishedMsg{tracks: tracks, err: err}
    }
}

func (m Model) downloadNextTrack(index int) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()
        updated, err := m.orchestrator.DownloadTrack(ctx, m.Tracks[index], m.outputDir)
        return trackDownloadedMsg{index: index, media: updated, err: err}
    }
}
```

**Input validation helper (inline or in same file):**

```go
func isValidYouTubeURL(url string) bool { ... }
// Same prefix matching as isSupportedURL in orchestrator
```

**Build verification:** `go test ./internal/tui/` PASS (all transition tests now pass).

---

### Task 6.2 — Implement View() for all 5 screens

| Field | Value |
| ------- | ------- |
| Files to create | `internal/tui/view.go` |
| Dependencies | 6.1 (uses Model fields populated by Update) |
| Ownership | `implementation` |

**`View()` rendering:**

```go
func (m Model) View() string {
    if m.Width < 80 || m.Height < 24 {
        return renderResizeMessage(m)
    }
    switch m.Screen {
    case ScreenInput:
        return renderInput(m)
    case ScreenResolving:
        return renderResolving(m)
    case ScreenPlaylist:
        return renderPlaylist(m)
    case ScreenDownloading:
        return renderDownloading(m)
    case ScreenDone:
        return renderDone(m)
    default:
        return ""
    }
}
```

**Screen renderers:**

| Screen | Content |
| -------- | --------- |
| **Input** | Title ("🎵 music-dl"), URL prompt, `InputText` display (with cursor), `InputError` (styled red, if set), footer with `keyHelp` |
| **Resolving** | Title, spinner message ("Resolving URL..."), Esc hint in footer |
| **Playlist** | Track list with checkboxes (`[✓]` / `[ ]`), cursor highlight, scroll indicator if needed, footer with full key help |
| **Downloading** | Each track with status icon: `⏳` pending, `⬇` downloading, `✓` done, `✗` failed. Quit confirmation overlay if `ConfirmingQuit` |
| **Done** | Summary header ("Download Complete"), total/succeeded/failed counts, failed track list, footer: Enter=new, q=quit |

**Resize message:** Centered text "Terminal too small. Resize to at least 80×24."

**Build verification:** `go build ./internal/tui/` PASS.

---

### Task 6.3 — Implement main.go entrypoint

| Field | Value |
| ------- | ------- |
| Files to create | `cmd/music-dl/main.go` |
| Dependencies | All packages (PR 1–6) |
| Ownership | `implementation` |

**`main()` flow:**

1. `ctx := context.Background()`
2. Preflight: `preflight.NewChecker("yt-dlp", "ffmpeg").Check(ctx)` — print errors to stderr, `os.Exit(1)` if any
3. Output: `filesystem.NewOutput("~/Music/music-dl")` — `os.Exit(1)` on error
4. `output.Ensure(ctx)` — `os.Exit(1)` on error
5. `searcher.NewSearcher()`, `downloader.NewDownloader()`
6. `service.NewOrchestrator(searcher, downloader)`
7. `tui.NewModel(orchestrator, output.FullPath())`
8. `tea.NewProgram(model, tea.WithAltScreen()).Run()` — `os.Exit(1)` on error

**Build verification:** `go build ./cmd/music-dl/` produces binary.

---

### Task 6.4 — Run full project build verification

| Field | Value |
| ------- | ------- |
| Commands to run | See below |
| Dependencies | 6.3 |
| Ownership | `implementation` |

```bash
go build ./cmd/music-dl/
go vet ./...
go test -short ./internal/...
```

**Expected:** All PASS. No warnings.

---

### Task 6.5 — Verify with short tests

| Field | Value |
| ------- | ------- |
| Command | `go test -short ./...` |
| Dependencies | 6.4 |
| Ownership | `implementation` |

Verify that integration tests (yt-dlp searcher + downloader) are correctly skipped with `-short`.

---

### Task 6.6 — Start or reuse bounded review <!-- sdd-owner: parent -->

- [ ] Run bounded review on PR 6 diff (full TUI + entrypoint). <!-- sdd-owner: parent -->
- [ ] Verify `go build ./cmd/music-dl/` succeeds. <!-- sdd-owner: parent -->
- [ ] Verify `go vet ./...` passes. <!-- sdd-owner: parent -->
- [ ] Verify `go test -short ./internal/...` passes. <!-- sdd-owner: parent -->
- [ ] Approve and merge PR 6 into `feature/skeleton-reboot`. <!-- sdd-owner: parent -->

---

## Post-Merge Verification (all PRs merged into feature branch)

### Task Final — Feature branch merges to main

| Field | Value |
| ------- | ------- |
| Action | Merge `feature/skeleton-reboot` → `main` |
| Dependencies | All PRs approved and merged |
| Ownership | `parent` |

**Pre-merge checklist:**

- [x] `go build ./cmd/music-dl/` succeeds. <!-- sdd-owner: implementation -->
- [x] `go vet ./...` passes. <!-- sdd-owner: implementation -->
- [x] `go test -short ./internal/...` passes. <!-- sdd-owner: implementation -->
- [ ] All PRs reviewed and approved. <!-- sdd-owner: parent -->
- [ ] Feature branch is up to date with `main`. <!-- sdd-owner: parent -->
- [ ] Squash-merge with message: `feat(music-dl): implement hexagonal architecture skeleton`. <!-- sdd-owner: parent -->

---

## Cross-Cutting Constraints (Apply to All Tasks)

### Import Rules (compile-time enforced)

| Package | Imports from | Must NOT import |
| --------- | -------------- | ----------------- |
| `core/domain` | stdlib only | anything else |
| `core/ports` | `context`, `core/domain` | adapters, TUI, service |
| `core/service` | `core/ports`, `core/domain` | adapters, TUI |
| `adapters/*` | `core/ports`, `core/domain` | TUI, service, other adapters |
| `tui/` | `core/service`, `core/domain` | adapters, `os/exec` |
| `cmd/music-dl` | all layers | — |

### TDD Ordering

Write test file first (RED), verify expected failure, then implement to pass (GREEN), then refactor if needed.

### Build Gates

Every PR branch must pass before merging:

```bash
go build ./...
go vet ./...
go test -short ./internal/...
```

### Integration Test Skipping

All yt-dlp tests MUST use:

```go
if testing.Short() {
    t.Skip("Skipping integration test")
}
```

### Domain Error Protocol

All adapter errors that cross port boundaries MUST be `domain.Error` type (implements `error` interface via value receiver):

```go
return ports.SearchResult{}, domain.Error{
    Code:    domain.ErrorBinaryNotFound,
    Message: "yt-dlp: " + err.Error(),
}
```

### Feature Branch Chain Workflow

```
main
  └── feature/skeleton-reboot (long-lived feature branch)
       ├── PR #1  → domain + ports            (merge to feature/skeleton-reboot)
       ├── PR #2  → orchestrator               (merge to feature/skeleton-reboot)
       ├── PR #3  → pure adapters              (merge to feature/skeleton-reboot)
       ├── PR #4  → integration adapters       (merge to feature/skeleton-reboot)
       ├── PR #5  → TUI types + tests          (merge to feature/skeleton-reboot)
       └── PR #6  → TUI update + view + main   (merge to feature/skeleton-reboot)
```

Each PR's review diff is scoped to its own changes. Only the final merge of `feature/skeleton-reboot` into `main` closes the change.
