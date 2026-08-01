# Design: Configurable Audio Quality (MP3 128k / 192k / 320k)

**Status:** Draft
**Date:** 2026-08-01
**Change:** `audio-quality`
**Applies to:** Go 1.26 + Bubble Tea TUI music-downloader (`github.com/Juanstudy/music-downloader`)

---

## 1. Architecture Overview

### 1.1 High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        cmd/music-dl/main.go                              │
│                                                                          │
│  config.LoadConfig(config.ConfigPath())  ──►  quality (default 320k)     │
│        │                                        │                        │
│        │                                        ▼                        │
│        │                    downloader.NewDownloader(                    │
│        │                        downloader.WithAudioBitrate(quality))    │
│        │                                        │                        │
│        └───────────────┐                        ▼                        │
│                        │      service.NewOrchestrator(searcher, dl)     │
│                        │                        │                        │
│                        ▼                        ▼                        │
│   tui.NewModel(orch, ..., outputDir, quality)                            │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │                     internal/tui/ (Bubble Tea)                   │    │
│  │                                                                  │    │
│  │  model.go        ScreenConfig, audioQuality, qualityCursor,      │    │
│  │                  configWarn, configPath, saveConfig              │    │
│  │  update.go       c on 4 screens → openConfig(), handleConfigKeys,│    │
│  │                  confirmQuality()                                 │    │
│  │  view.go         renderConfigView(), View() dispatch, footer     │    │
│  │  keys.go         "c" → helpContent                               │    │
│  └───────────────┬──────────────────────────────────────────────────┘    │
│                  │ confirmQuality()                                      │
│                  │  ├─ orchestrator.SetAudioQuality(q)                   │
│                  │  └─ config.SaveConfig(configPath, cfg)               │
│                  ▼                                                       │
│  ┌─────────────────────────┐         ┌──────────────────────────────┐   │
│  │  internal/config (NEW)  │         │  internal/core/service/      │   │
│  │  ConfigPath, LoadConfig,│         │  orchestrator.go             │   │
│  │  SaveConfig,            │         │  SetAudioQuality passthrough │   │
│  │  ValidQualities         │         └──────────────┬───────────────┘   │
│  └─────────────────────────┘                        │ qualitySetter     │
│                                                     ▼                   │
│  ┌──────────────────────────────────────────────────────────────┐       │
│  │  internal/adapters/downloader/ytdlp.go                       │       │
│  │  Option, WithAudioBitrate, SetAudioBitrate, buildArgs()      │       │
│  │  (port ports.Downloader UNCHANGED)                           │       │
│  └──────────────────────────────────────────────────────────────┘       │
└──────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Component Responsibilities

| Component | Responsibility |
| --------- | --------------- |
| `internal/config` (new package) | Canonical XDG-aware config path, `[quality]` load with 320k normalization, load-merge-save writer, the three quality constants |
| `internal/adapters/downloader` | Constructor option + mid-session setter + pure `buildArgs(media, outputDir, bitrate)` |
| `internal/core/service.Orchestrator` | `SetAudioQuality(q)` passthrough via guarded type assertion — port untouched |
| `internal/tui` | `ScreenConfig`, `c` binding on 4 screens, `openConfig`/`handleConfigKeys`/`confirmQuality`, `renderConfigView` |
| `cmd/music-dl/main.go` | Load quality first, inject into downloader + TUI; non-fatal on load error |
| `README.md` | `c` control row + roadmap update |

### 1.3 Config File Shape (after this change)

```toml
[quality]
value = "320k"

[spotify]
client_id = "..."
client_secret = "..."
```

---

## 2. Interface / Type Definitions

### 2.1 NEW package `internal/config`

**File:** `internal/config/config.go`

```go
package config

// Quality levels. Exactly three; DefaultQuality is the fallback everywhere.
const (
    Quality128     = "128k"
    Quality192     = "192k"
    Quality320     = "320k"
    DefaultQuality = Quality320
)

// Quality holds the [quality] TOML section.
type Quality struct {
    Value string `toml:"value"`
}

// Config mirrors the config file: [quality] plus the existing [spotify] section
// (so a save never drops Spotify credentials).
type Config struct {
    Quality Quality `toml:"quality"`
    Spotify struct {
        ClientID     string `toml:"client_id"`
        ClientSecret string `toml:"client_secret"`
    } `toml:"spotify"`
}

// ConfigPath returns $XDG_CONFIG_HOME/music-dl/config.toml, falling back to
// ~/.config/music-dl/config.toml. Same file the Spotify adapter reads.
func ConfigPath() string

// ValidQualities returns the three levels in display order: 128k, 192k, 320k.
func ValidQualities() []string

// IsValidQuality reports whether q is one of the three valid levels.
func IsValidQuality(q string) bool

// LoadConfig returns the effective configuration. Missing file, missing
// [quality] section, and invalid values are normalized to DefaultQuality with
// a warning (invalid) and no error. Malformed TOML returns a non-nil error and
// a zero Config (caller falls back to DefaultQuality).
func LoadConfig(path string) (Config, error)

// SaveConfig persists cfg with load-merge-save: the existing file is decoded
// first so the [spotify] section survives, then the [quality] section is
// replaced by cfg.Quality. Creates the music-dl directory and file on first
// save. If the existing file is malformed TOML, SaveConfig returns an error
// (the broken file is never clobbered).
func SaveConfig(path string, cfg Config) error
```

Contract details the verify phase must pin:

- **LoadConfig missing file / missing section** → `Config{Quality: Quality{Value: DefaultQuality}}`, `nil` error.
- **LoadConfig invalid value** (e.g. `"999k"`, `"flac"`, `"best"`) → `log.Printf("warning: invalid audio quality %q, using %s", v, DefaultQuality)` + effective value `DefaultQuality`, `nil` error.
- **LoadConfig malformed TOML** → non-nil error, zero `Config`. `main.go` absorbs it: logs a warning and uses `DefaultQuality` (app continues, no crash — AQ-003).
- **SaveConfig merge rule:** `merged` starts as the decoded existing file; `merged.Quality = cfg.Quality` is the only overwrite. `cfg.Spotify` is ignored by design (this slice only writes quality). Other keys (comments) are not preserved by BurntSushi re-encode — see Open Decisions #5.
- **SaveConfig write path:** `os.MkdirAll(filepath.Dir(path), 0o755)` → encode to `bytes.Buffer` via `toml.NewEncoder(&buf).Encode(merged)` → `os.WriteFile(path, buf.Bytes(), 0o644)`. Buffer-then-write avoids leaving a truncated file on encode failure.
- **SaveConfig malformed existing file** → `fmt.Errorf("save config: existing config is malformed: %w", err)`; nothing is written.

### 2.2 Downloader (`internal/adapters/downloader/ytdlp.go`)

```go
type Downloader struct {
    binary       string
    audioBitrate string // "" = no --audio-bitrate flag (pre-change behavior)
}

// Option configures a Downloader at construction time.
type Option func(*Downloader)

// WithAudioBitrate sets the MP3 bitrate used for subsequent downloads.
func WithAudioBitrate(q string) Option

func NewDownloader(opts ...Option) *Downloader

// SetAudioBitrate changes the bitrate for subsequent downloads mid-session.
func (d *Downloader) SetAudioBitrate(q string)

// buildArgs returns the yt-dlp invocation arguments. When bitrate is non-empty,
// --audio-bitrate <bitrate> is inserted immediately after --audio-format mp3.
// When bitrate is empty the args are byte-for-byte identical to the pre-change
// invocation. Pure function (no receiver, no I/O) — unit-testable without yt-dlp.
func buildArgs(media domain.Media, outputDir, bitrate string) []string
```

`buildArgs` exact shape:

```go
func buildArgs(media domain.Media, outputDir, bitrate string) []string {
    outputTemplate := filepath.Join(outputDir, "%(artist)s - %(title)s.%(ext)s")
    args := []string{"-x", "--audio-format", "mp3"}
    if bitrate != "" {
        args = append(args, "--audio-bitrate", bitrate)
    }
    args = append(args,
        "--embed-metadata",
        "--embed-thumbnail",
        "--add-metadata",
        "-o", outputTemplate,
        "--no-warnings",
        media.URL,
    )
    return args
}
```

`Download` changes: replace the inline `args := []string{...}` block with

```go
args := buildArgs(media, outputDir, d.audioBitrate)
cmd := exec.CommandContext(ctx, d.binary, args...)
```

Everything else in `Download` (MkdirAll, stdout discard, stderr capture, output path resolution, `sanitizeFilename`) is unchanged. Add a compile-time guard `var _ ports.Downloader = (*Downloader)(nil)` (AQ-018 evidence).

### 2.3 Orchestrator (`internal/core/service/orchestrator.go`)

```go
// qualitySetter is the optional capability the downloader may expose. Kept
// local so core does not import the adapter package and the port stays frozen.
type qualitySetter interface {
    SetAudioBitrate(string)
}

// SetAudioQuality forwards the audio quality to the downloader so subsequent
// downloads use it. No-op when the injected downloader has no setter.
func (o *Orchestrator) SetAudioQuality(q string) {
    if s, ok := o.downloader.(qualitySetter); ok {
        s.SetAudioBitrate(q)
    }
}
```

`ports.Downloader` and `Orchestrator.DownloadTrack` are **byte-for-byte unchanged** (AQ-018).

### 2.4 TUI types (`internal/tui/model.go`)

```go
const (
    ScreenInput Screen = iota
    ScreenResolving
    ScreenPlaylist
    ScreenDownloading
    ScreenDone
    ScreenConfig // NEW — sixth screen
)
```

New `Model` fields:

```go
// Config screen
audioQuality  string                              // current effective quality
qualityCursor int                                 // cursor into config.ValidQualities()
configWarn    string                              // non-fatal save-failure warning (config view)
configPath    string                              // file passed to saveConfig (tests override)
saveConfig    func(path string, cfg config.Config) error // test seam; NewModel wires config.SaveConfig
```

New `NewModel` signature (add trailing param):

```go
// Before:
func NewModel(orch *service.Orchestrator, youtubeSearcher, spotifySearcher ports.Searcher, querySearcher ports.QuerySearcher, outputDir string) Model

// After:
func NewModel(orch *service.Orchestrator, youtubeSearcher, spotifySearcher ports.Searcher, querySearcher ports.QuerySearcher, outputDir, audioQuality string) Model
```

`NewModel` body additionally initializes:

```go
audioQuality:  audioQuality,
qualityCursor: 0,
configWarn:    "",
configPath:    config.ConfigPath(), // derived, not a param (see §6.2)
saveConfig:    config.SaveConfig,
```

`internal/tui` now imports `github.com/Juanstudy/music-downloader/internal/config`.

### 2.5 Function signature changes

| Symbol | Before | After |
| ------ | ------ | ----- |
| `downloader.NewDownloader` | `NewDownloader() *Downloader` | `NewDownloader(opts ...Option) *Downloader` |
| `downloader` | — | `WithAudioBitrate(q string) Option`, `SetAudioBitrate(q string)`, `buildArgs(media domain.Media, outputDir, bitrate string) []string` |
| `service.Orchestrator` | — | `SetAudioQuality(q string)` |
| `tui.NewModel` | `(orch, youtubeSearcher, spotifySearcher, querySearcher, outputDir)` | `(orch, youtubeSearcher, spotifySearcher, querySearcher, outputDir, audioQuality)` |
| `ports.Downloader` | unchanged | **unchanged** (AQ-018) |

---

## 3. File-by-File Change Plan

### 3.1 NEW: `internal/config/config.go`

Full package as specified in §2.1 (~90 lines). Imports: `bytes`, `log`, `os`, `path/filepath`, `github.com/BurntSushi/toml`. Path logic is a copy of `spotify.ConfigPath()` (canonical copy — AQ-019 keeps `spotify/config.go` untouched).

### 3.2 NEW: `internal/config/config_test.go`

See §5 Testing Strategy.

### 3.3 MODIFIED: `internal/adapters/downloader/ytdlp.go`

1. Add `audioBitrate string` field to `Downloader`.
2. Add `Option` type + `WithAudioBitrate`.
3. Change `NewDownloader()` → `NewDownloader(opts ...Option)`, applying opts in order.
4. Add `SetAudioBitrate`.
5. Extract the args literal into `buildArgs` (pure) and call it from `Download`.
6. Add `var _ ports.Downloader = (*Downloader)(nil)`.

### 3.4 MODIFIED: `internal/adapters/downloader/ytdlp_test.go`

Add table-driven `buildArgs` tests (no yt-dlp invocation). Existing integration tests keep calling `NewDownloader()` — they compile unchanged because the options are variadic (AQ-006 "no option keeps today's behavior").

### 3.5 MODIFIED: `internal/core/service/orchestrator.go`

Add the `qualitySetter` interface and `SetAudioQuality` method (§2.3). No other change.

### 3.6 MODIFIED: `internal/core/service/orchestrator_test.go`

1. Add `audioBitrate string` field + `SetAudioBitrate(q string)` method to `mockDownloader` (records the value; existing tests unaffected).
2. Add `TestSetAudioQualityForwardsToDownloader`.

### 3.7 MODIFIED: `internal/tui/model.go`

1. Add `ScreenConfig` constant (after `ScreenDone`, value 5).
2. Add the four config fields (§2.4).
3. Extend `NewModel` signature + initialization.

### 3.8 MODIFIED: `internal/tui/update.go`

#### 3.8.1 `Update()` — screen routing

Add before `default`:

```go
case ScreenConfig:
    if km, ok := msg.(tea.KeyMsg); ok {
        return m.handleConfigKeys(km)
    }
    return m, nil
```

Global `q`/`?`/`ctrl+c` and the message handlers (`resolveFinishedMsg`, `trackDownloadedMsg`, `spinner.TickMsg`) run before screen routing, so they keep working on `ScreenConfig` (AQ-011). Note: if the queue finishes while Config is open, `handleTrackDone` still transitions to `ScreenDone` — accepted (see Open Decisions #6).

#### 3.8.2 New: `openConfig()`

```go
// openConfig switches to the Config screen, remembering the origin screen and
// placing the cursor on the current effective quality.
func (m Model) openConfig() (tea.Model, tea.Cmd) {
    m.PrevScreen = m.Screen
    m.Screen = ScreenConfig
    m.configWarn = ""
    m.qualityCursor = indexOfQuality(m.audioQuality)
    return m, nil
}

// indexOfQuality returns the index of q in config.ValidQualities(), or 0 when
// q is not a valid level (never happens in practice: LoadConfig normalizes).
func indexOfQuality(q string) int {
    for i, v := range config.ValidQualities() {
        if v == q {
            return i
        }
    }
    return 0
}
```

#### 3.8.3 New: `handleConfigKeys()`

```go
func (m Model) handleConfigKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "j", "down":
        if m.qualityCursor < len(config.ValidQualities())-1 {
            m.qualityCursor++
        }
        return m, nil
    case "k", "up":
        if m.qualityCursor > 0 {
            m.qualityCursor--
        }
        return m, nil
    case "enter":
        return m.confirmQuality()
    case "esc":
        m.Screen = m.PrevScreen
        m.qualityCursor = 0
        m.configWarn = ""
        return m, nil
    }
    return m, nil
}
```

(`q` and `?` never reach here — the global switch in `Update()` handles them first.)

#### 3.8.4 New: `confirmQuality()`

```go
// confirmQuality applies the selected quality in-session, persists it, and
// returns to the origin screen. Persistence failures are non-fatal: the
// in-session value and the downloader stay updated, a warning is shown in the
// config view, and the app keeps running (AQ-013).
func (m Model) confirmQuality() (tea.Model, tea.Cmd) {
    q := config.ValidQualities()[m.qualityCursor]
    m.audioQuality = q
    if m.orchestrator != nil {
        m.orchestrator.SetAudioQuality(q)
    }

    if err := m.saveConfig(m.configPath, config.Config{Quality: config.Quality{Value: q}}); err != nil {
        m.configWarn = fmt.Sprintf("Could not save config (%v). Applied for this session only.", err)
        return m, nil // stay on ScreenConfig so the warning is visible
    }

    m.configWarn = ""
    m.Screen = m.PrevScreen
    m.qualityCursor = 0
    return m, nil
}
```

#### 3.8.5 `c` interception — one `case "c"` per screen handler

- `handleResolvingKeys`: add `case "c": return m.openConfig()`.
- `handlePlaylistKeys`: add `case "c": return m.openConfig()` **before** `default`. The `if m.isFiltering { return m.handlePlaylistFilterInput(msg) }` guard already sits above the switch, so a `c` typed into the filter reaches the filter input (AQ-010).
- `handleDownloadingKeys`: add `case "c": return m.openConfig()`.
- `handleDoneKeys`: add `case "c": return m.openConfig()`.
- `handleInputKeys`: **NO change** — `c` falls through to `m.Input.Update(msg)` and types normally (AQ-010). The existing `s` intercept is the only letter intercept there.

### 3.9 MODIFIED: `internal/tui/view.go`

1. `View()` dispatch — add `case ScreenConfig: content = m.renderConfigView()`.
2. New `renderConfigView()`:

```go
// renderConfigView lists the three quality options with a cursor/selection
// indicator, the current effective quality, a save-failure warning when
// present, and the footer hint.
func (m Model) renderConfigView() string {
    var b strings.Builder

    b.WriteString(m.renderHeader("♪ music-dl — Configure Quality"))
    b.WriteString("\n\n")

    if m.configWarn != "" {
        b.WriteString(warningStyle.Render("⚠ " + m.configWarn))
        b.WriteString("\n\n")
    }

    b.WriteString(mutedStyle.Render("Audio quality (yt-dlp never up-samples: a lower source stays lower)"))
    b.WriteString("\n\n")

    for i, q := range config.ValidQualities() {
        marker := " "
        cursor := "  "
        if i == m.qualityCursor {
            cursor = "▸"
            marker = "●"
        }
        line := fmt.Sprintf("%s %s %s", cursor, marker, q)
        if i == m.qualityCursor {
            b.WriteString(selectedStyle.Render(line))
        } else {
            b.WriteString(line)
        }
        b.WriteString("\n")
    }

    b.WriteString("\n")
    b.WriteString(mutedStyle.Render("Current: ") + emphStyle.Render(m.audioQuality))
    b.WriteString("\n\n")
    b.WriteString(m.renderFooter())

    return b.String()
}
```

1. `renderFooter()` — add a Config block:

```go
if m.Screen == ScreenConfig {
    keys = append(keys,
        keyStyle.Render("j/k")+" "+keyDescStyle.Render("move"),
        keyStyle.Render("Enter")+" "+keyDescStyle.Render("confirm"),
        keyStyle.Render("Esc")+" "+keyDescStyle.Render("back"),
    )
}
```

(`q` quit is global and needs no footer row on Config.)

### 3.10 MODIFIED: `internal/tui/keys.go`

Append to `helpContent`:

```go
{"c", "Configure quality"},
```

(AQ-015: shows on every screen via the existing global help overlay.)

### 3.11 MODIFIED: `cmd/music-dl/main.go`

Order of construction: **preflight → outputDir → quality config → searchers → downloader → orchestrator → spotify wiring → model** (config load stays after the preflight exit gate).

1. Add import `"github.com/Juanstudy/music-downloader/internal/config"`.
2. After `outputDir := defaultOutputDir()` and before the searcher wiring:

```go
// Load the audio quality setting (defaults to 320k). A malformed config file
// is non-fatal: warn and keep the default.
quality := config.DefaultQuality
cfg, err := config.LoadConfig(config.ConfigPath())
if err != nil {
    log.Printf("warning: failed to load config: %v", err)
} else {
    quality = cfg.Quality.Value
}
```

1. Change the downloader construction:

```go
downloaderImpl := downloader.NewDownloader(downloader.WithAudioBitrate(quality))
```

1. Change the model construction:

```go
m := tui.NewModel(orch, searcherImpl, spotifySearcher, querySearcherImpl, outputDir, quality)
```

The Spotify wiring block is **unchanged** (it keeps using `spotify.ConfigPath()`; both paths resolve to the same file — AQ-019).

### 3.12 MODIFIED: `internal/tui/update_test.go` and NEW `internal/tui/view_test.go`

See §5. `update_test.go` gains ~9 test functions; `view_test.go` is a new file (none exists today).

### 3.13 MODIFIED: `README.md`

1. Controles table (~line 116): add a `c` row, e.g. `|`c`| Configurar calidad de audio (128k / 192k / 320k) |` (Spanish, matching the existing table).
2. Roadmap (~line 18): replace `Calidad configurable (128k, 320k, lossless)` with a delivered item (e.g. `Calidad configurable (128k / 192k / 320k) ✔`) and keep lossless listed as open.

---

## 4. Data Flow Details

### 4.1 Startup wiring

```
main() (after preflight)
  │
  ├─ quality = config.LoadConfig(config.ConfigPath())
  │     ├─ file missing / [quality] missing / invalid  → 320k (invalid: warning log)
  │     ├─ malformed TOML                              → error → log warning → 320k
  │     └─ valid (e.g. "192k")                         → "192k"
  │
  ├─ downloader.NewDownloader(WithAudioBitrate(quality))
  ├─ service.NewOrchestrator(searcherImpl, downloaderImpl)
  └─ tui.NewModel(orch, ..., outputDir, quality)
```

### 4.2 Opening Config (`c` on Resolving / Playlist / Downloading / Done)

```
User presses 'c' (e.g. on ScreenPlaylist, not filtering)
        │
        ▼
  handlePlaylistKeys → case "c" → openConfig()
        │
        ├─ PrevScreen = ScreenPlaylist
        ├─ Screen = ScreenConfig
        ├─ configWarn = ""
        └─ qualityCursor = indexOfQuality(m.audioQuality)
              │
              ├─ audioQuality "192k" → cursor at index 1 (192k option marked)
              └─ audioQuality "320k" → cursor at index 2 (320k option marked)
```

### 4.3 Navigating and confirming

```
j/k → cursor moves, bounded to [0, 2]
Esc → Screen = PrevScreen, qualityCursor = 0, configWarn = "" (NO write — AQ-011)
Enter → confirmQuality()
        │
        ├─ q = ValidQualities()[cursor]
        ├─ m.audioQuality = q
        ├─ orchestrator.SetAudioQuality(q)          (in-session, next download)
        ├─ saveConfig(configPath, Config{Quality{Value: q}})
        │     ├─ SUCCESS → configWarn = "", Screen = PrevScreen, cursor = 0
        │     └─ FAIL    → configWarn = "Could not save config (...)...",
        │                  stay on ScreenConfig (warning visible), no crash
```

### 4.4 Downloader flag insertion

```
quality = ""       → yt-dlp -x --audio-format mp3 --embed-metadata ...
quality = "192k"   → yt-dlp -x --audio-format mp3 --audio-bitrate 192k --embed-metadata ...
```

A change mid-session only affects `buildArgs` calls after the setter: the in-flight track already captured its args (AQ-006 "change mid-download leaves in-flight track unaffected").

---

## 5. Testing Strategy

Conventions followed: TUI scenario tests use `Model{}` literals (never `NewModel`); config tests are table-driven over `t.TempDir()`; downloader args tests are table-driven with **no real yt-dlp**; `testing.Short()` guards integration.

### 5.1 Requirement → Test Mapping (AQ-001 … AQ-020)

| Req | Test | File | Scenario |
| --- | ---- | ---- | -------- |
| AQ-001 | `TestValidQualities` / `TestIsValidQuality_RejectsNonLevels` / `TestDefaultQuality` | `config_test.go` | exactly `[128k,192k,320k]`; `"64k"`, `"best"`, `"lossless"`, `""` invalid; default `"320k"` |
| AQ-002 | `TestConfigPath_XDGSet` / `TestConfigPath_XDGUnset` | `config_test.go` | `t.Setenv("XDG_CONFIG_HOME", "/custom/config")` → `/custom/config/music-dl/config.toml`; unset → `filepath.Join(home, ".config", "music-dl", "config.toml")` |
| AQ-003 | `TestLoadConfig_MissingFile` / `TestLoadConfig_MissingQualitySection` / `TestLoadConfig_InvalidValueFallsBack` / `TestLoadConfig_MalformedTOML` | `config_test.go` | temp dir without file → `320k`, nil error; file with only `[spotify]` → `320k`; `[quality] value = "999k"` (also `"flac"`) → `320k`, nil error; garbage file → non-nil error |
| AQ-004 | `TestSaveConfig_RoundTripPreservesSpotify` / `TestSaveConfig_FirstSaveCreatesDirAndFile` / `TestSaveConfig_MalformedExistingFile` | `config_test.go` | canonical `[spotify]` file + save `192k` → file has `[quality] value = "192k"` and the `[spotify]` block is byte-identical; empty temp dir + save → dir+file created; malformed existing file → non-nil error, file unchanged |
| AQ-005 | `TestBuildArgs_NoBitrate` / `TestBuildArgs_BitratePosition` / `TestBuildArgs_ExistingFlagsIntact` | `ytdlp_test.go` | empty bitrate → no `--audio-bitrate`, all pre-change flags present, `-o` template prefixed with outputDir; `"192k"` → `--audio-bitrate 192k` immediately after `--audio-format mp3` |
| AQ-006 | `TestWithAudioBitrate_EachLevel` / `TestNewDownloader_NoOption` / `TestSetAudioBitrate_MidSession` | `ytdlp_test.go` | each level → its flag (via `buildArgs(media, out, d.audioBitrate)`); `NewDownloader()` → field `""` → no flag; set `128k`→`320k` → next args use `320k` |
| AQ-007 | `TestSetAudioQualityForwardsToDownloader` | `orchestrator_test.go` | `mockDownloader` records `"192k"` after `SetAudioQuality("192k")`; port/DownloadTrack compile unchanged (existing tests) |
| AQ-008 | `TestOpenConfig_CursorAtCurrentQuality` (+ model literal usage compiles) | `update_test.go` | `Model{Screen: ScreenPlaylist, audioQuality: "192k"}` + `c` → `ScreenConfig`, `qualityCursor == 1` |
| AQ-009 | `TestConfig_CFromScreens` (table) | `update_test.go` | 4 rows: Resolving / Playlist / Downloading / Done → `ScreenConfig`, `PrevScreen` = origin |
| AQ-010 | `TestConfig_COnInputTypes` / `TestConfig_CInFilterTypes` | `update_test.go` | Input focused with `"music.youtube.com/watch"` + `c` → stays Input, value ends `c`; Playlist `isFiltering: true` + `c` → filter text gains `c`, filter not closed |
| AQ-011 | `TestConfig_JKMovesBounded` / `TestConfig_EscCancelsWithoutWrite` / `TestConfig_QStillQuits` / `TestConfig_HelpStillToggles` | `update_test.go` | cursor clamps at 0 and 2; Esc from Config (opened from Playlist, `saveConfig` spy) → back to Playlist, `audioQuality` unchanged, spy not called; `q` → `tea.Quit`; `?` toggles `showHelp` |
| AQ-012 | `TestConfig_EnterConfirmsAndPersists` | `update_test.go` | `configPath` in `t.TempDir()`, `saveConfig: config.SaveConfig`, cursor on 192k, real orchestrator with mock downloader → `audioQuality == "192k"`, mock bitrate `"192k"`, file decodes to `[quality] value = "192k"`, back to `PrevScreen`. Restart scenario = `LoadConfig` round-trip (config tests) + cursor-on-open (AQ-008 test) + `go build` wiring (AQ-016) |
| AQ-013 | `TestConfig_EnterSaveFailureNonFatal` | `update_test.go` | `saveConfig: func(...) error { return errors.New("disk full") }` → `audioQuality` = confirmed value, mock bitrate set, `configWarn != ""`, stays on `ScreenConfig`, no quit cmd |
| AQ-014 | `TestConfigView_RendersOptionsAndCursor` / `TestConfigView_RendersCurrentQualityAndFooter` | `view_test.go` (NEW) | `View()` on Config cursor at 320k → output contains `128k`, `192k`, `320k`, 320k line marked (selected/cursor); `audioQuality "192k"` → output indicates current quality + footer mentions `j/k`, `Enter`, `Esc` |
| AQ-015 | `TestHelpShowsCKey` | `view_test.go` (NEW) | `helpView(width)` output contains `c` and `Configure quality` |
| AQ-016 | `go build ./cmd/music-dl/` | (verify phase) | wiring compiles; downloader constructed with `WithAudioBitrate(quality)`; `NewModel` receives `quality` |
| AQ-017 | README review | (verify phase) | Controles table has `c` row; roadmap: lossy delivered, lossless still open |
| AQ-018 | existing tests + `var _ ports.Downloader = (*Downloader)(nil)` | `ytdlp.go` / all | everything compiles with port unchanged |
| AQ-019 | `go test ./internal/adapters/spotify/...` | (verify phase) | spotify package untouched, tests pass |
| AQ-020 | `go test ./...` | (verify phase) | full suite green |

### 5.2 Test helper additions

- `update_test.go`: `newFilterInput()` (mirrors `newInput()`, returns a focused `textinput.Model`), and a `saveConfig` spy helper if needed (`func(path string, cfg config.Config) error` recording calls).
- `view_test.go`: model builder `configModel()` — `Model{Screen: ScreenConfig, Ready: true, Width: 80, Height: 24, audioQuality: ..., qualityCursor: ...}`.
- `orchestrator_test.go`: extend `mockDownloader` with `audioBitrate string` + `SetAudioBitrate`.

### 5.3 Wiring compilation

```bash
go build ./cmd/music-dl/   # must succeed (AQ-016)
go test ./...              # must pass (AQ-020)
```

---

## 6. Key Design Decisions

### 6.1 `NewModel` param vs. post-construction setter → **param**

`NewModel(..., outputDir, audioQuality string)`. Justification:

- `main.go` is the **only** caller of `NewModel`; TUI tests build `Model{}` literals (verified in `update_test.go`), so the new param is invisible to every existing test. The proposal's risk analysis explicitly calls this contained.
- `audioQuality` is initial configuration state — the same category as `outputDir` (also a constructor param). A post-construction `SetAudioQuality` on the Model would be a single-call dead API in `main.go` and would force an exported field or bespoke method for no benefit.
- Consistency: every injected value in this codebase enters through `NewModel`.

### 6.2 Config path is derived, not passed

`NewModel` sets `configPath: config.ConfigPath()` internally; tests override the field. `config.ConfigPath()` is a deterministic function of the environment; threading a 7th param through `main.go` adds noise with no testability gain (the field seam already exists).

### 6.3 `saveConfig` function field as test seam

`confirmQuality` calls `m.saveConfig(m.configPath, ...)`. `NewModel` wires it to `config.SaveConfig`; tests wire a spy or a failing stub to prove AQ-012 (file written) and AQ-013 (failure non-fatal) **without touching the user's real `~/.config/music-dl/config.toml`**. This is the only way to force a write failure deterministically (permission-based failure is root-user dependent and flaky).

### 6.4 Orchestrator passthrough via local `qualitySetter` interface

`SetAudioQuality` type-asserts `o.downloader.(qualitySetter)`. Alternatives rejected: changing the port (breaks AQ-018) and typing the field as `*downloader.Downloader` (inverts the dependency — core must not import adapters). The local one-method interface is the standard Go "optional capability" pattern; a downloader without the setter degrades to a no-op.

### 6.5 `buildArgs` is a pure package-level function

No receiver, no I/O, deterministic from `(media, outputDir, bitrate)`. This is what makes the whole flag contract unit-testable without yt-dlp (AQ-005/AQ-006), matching the proposal's "no real yt-dlp" constraint. `Download` is now a thin shell: `args := buildArgs(media, outputDir, d.audioBitrate)`.

### 6.6 Cursor position on open → current quality (not first option)

`openConfig` positions `qualityCursor` at the index of `m.audioQuality` (AQ-008 scenario: "cursor MUST point at the 192k option"). AQ-009's "reset the cursor to the first option" is read as "reset to a fresh position each open" — its scenarios only assert `Screen`/`PrevScreen`, so both AQ-008 and AQ-009 tests pass. See Open Decisions #2.

### 6.7 Save failure keeps the user on the Config screen

On `saveConfig` error, `confirmQuality` stays on `ScreenConfig` with `configWarn` rendered inline — this makes AQ-013's "a warning MUST be displayed in the config view" literally true at the moment of failure. The happy path (AQ-012) still returns to `PrevScreen`.

### 6.8 `c` intercepted per-screen, never in the input widget

Follows the existing `s` pattern exactly: one `case "c"` per non-Input screen handler. The Playlist filter guard (`isFiltering`) already routes typing ahead of the key switch (AQ-010).

---

## 7. Open Decisions (assumed)

Decisions the spec left ambiguous; the verify phase should pin these as written here:

1. **`[quality]` value key.** The spec's shorthand `[quality] = "192k"` is not valid TOML (a table cannot be a string). Assumed: `[quality]` is a section containing `value = "192k"`, mirroring the `[spotify]` nested-section pattern in `spotify.Config`. File: `[quality]` → `value` key.
2. **Cursor on open (AQ-008 vs AQ-009).** Assumed: cursor lands on the current effective quality's index, not the first option (see §6.6). AQ-009's "first option" language is treated as "reset the cursor each open".
3. **Save-failure screen behavior.** Assumed: stay on `ScreenConfig` to display the warning (AQ-013); only the successful path returns to `PrevScreen` (AQ-012). The AQ-013 scenario does not assert the resulting screen.
4. **`SaveConfig` with a malformed existing file.** Assumed: returns an error and writes nothing (never clobber a file whose `[spotify]` content cannot be read). AQ-004 only specifies the happy round-trip.
5. **"Byte-identical `[spotify]`"** holds for canonically-encoded files (as produced by BurntSushi). Re-encoding drops comments and normalizes formatting; the round-trip test seeds the file with `toml.Encoder` output so the section comparison is byte-identical. Hand-written files with comments will be reformatted on first save — accepted for this slice.
6. **Queue completion while Config is open.** Global message handlers preempt screen routing, so `trackDownloadedMsg` chains the next download or moves to `ScreenDone` even if the user is on `ScreenConfig`. Downloads keep running (per proposal); the app lands on Done if the queue finishes while configuring. No special handling.
7. **`LoadConfig` returns a value type** (`(Config, error)`), not the `spotify` pointer style. Different contract by design: defaults are applied inside the loader, so callers never handle nil. Malformed TOML returns a zero `Config` + error.
8. **`tui` imports `internal/config`.** The TUI depends on `config.ValidQualities()` (single source of the option list) and the `config.Config` type in the `saveConfig` seam. No cycle: `internal/config` imports only stdlib + BurntSushi.

---

## 8. Migration / Rollback

### 8.1 Migration (forward)

Additive, deploy order:

1. `internal/config` package (new, zero impact on existing code).
2. Downloader options/setter/`buildArgs` (backward compatible — `NewDownloader()` still compiles and behaves identically).
3. Orchestrator `SetAudioQuality` (additive).
4. TUI: `ScreenConfig`, handlers, view, keys (additive; the five existing screens untouched).
5. `main.go` wiring.
6. README + tests.
7. `go test ./...` + `go build ./cmd/music-dl/`.

### 8.2 Rollback (backward)

Per proposal §11: revert TUI, remove `internal/config`, restore `NewDownloader()`, revert wiring/spec/README. Config files already containing `[quality]` are harmless: BurntSushi ignores unknown sections and `spotify.Config` does not read `[quality]`. No data migration.

### 8.3 Compatibility guarantees

| Artifact | Affected? |
| -------- | --------- |
| `ports.Downloader` | NO — unchanged (AQ-018) |
| `Orchestrator.DownloadTrack` | NO — unchanged |
| `spotify.Config` / `spotify.LoadConfig` / Spotify flow | NO — untouched (AQ-019) |
| Five existing TUI screens + transitions | NO — unchanged |
| Global `q` / `?` / `ctrl+c` | NO — unchanged |
| Existing downloader integration tests | NO — `NewDownloader()` keeps compiling (variadic options) |
| URL-mode resolve/download flow | NO — only change is the default `--audio-bitrate 320k` flag (AQ-020) |

---

## 9. Risks and Mitigations

| Risk | Severity | Mitigation |
| ---- | -------- | ---------- |
| Diff crosses the 400-line single-PR budget | **HIGH** (see forecast below) | Table-driven config/args tests; scenario-based TUI tests; if the guard fires, `ask-on-risk` delivery applies — natural split is PR #1 = `internal/config` + downloader + orchestrator (+ canonical spec), PR #2 = TUI + entrypoint + README |
| `SaveConfig` re-encode reformats a hand-edited `config.toml` (comments lost) | Low | Accepted (Open Decisions #5); Spotify credentials are preserved by load-merge-save, which is the stated requirement |
| `c` intercept could capture typed letters | Low | Per-screen intercept only; Playlist filter guard runs first; Input untouched (AQ-010 tests pin it) |
| `SetAudioQuality` silently no-ops with a non-downloader adapter | Low | Only the real `downloader.Downloader` is wired in `main.go`; orchestrator test pins the passthrough |
| yt-dlp `--audio-bitrate` flag drift | Low | `buildArgs` unit-tested without yt-dlp; integration tests stay `testing.Short()`-gated |
| Spinner/tick interference on Config | None | `spinner.TickMsg` only ticks on `ScreenResolving`; Config has no spinner |
| LoadConfig warning spam on every start with an invalid value | Low | Single `log.Printf` warning per load — matches `main.go`'s existing warning style |

### 9.1 Review workload forecast (per file, added/changed lines)

| File | Est. lines | Status |
| ---- | ---------- | ------ |
| `internal/config/config.go` | ~90 | NEW |
| `internal/config/config_test.go` | ~135 | NEW |
| `internal/adapters/downloader/ytdlp.go` | ~30 | MODIFIED |
| `internal/adapters/downloader/ytdlp_test.go` | ~55 | MODIFIED |
| `internal/core/service/orchestrator.go` | ~10 | MODIFIED |
| `internal/core/service/orchestrator_test.go` | ~18 | MODIFIED |
| `internal/tui/model.go` | ~14 | MODIFIED |
| `internal/tui/update.go` | ~60 | MODIFIED |
| `internal/tui/view.go` | ~38 | MODIFIED |
| `internal/tui/keys.go` | ~1 | MODIFIED |
| `internal/tui/update_test.go` | ~165 | MODIFIED |
| `internal/tui/view_test.go` | ~55 | NEW |
| `cmd/music-dl/main.go` | ~12 | MODIFIED |
| `README.md` | ~8 | MODIFIED |
| `openspec/specs/adapters-downloader/spec.md` | ~4 | already updated in this change |
| **Total** | **~625–660** | **crosses the 400-line guard — HIGH risk** |

The proposal's 350–400 estimate was optimistic about TUI test expansion (the spec's own test table lists 10 TUI cases plus the 9 AQ-008…015 scenarios). **Recommendation for the orchestrator:** run the workload guard with `ask-on-risk`; if a single PR is required, use the PR #1 / PR #2 split above.

---

## Summary of Changed Files

| File | Status | Change Summary |
| ---- | ------ | -------------- |
| `internal/config/config.go` | **NEW** | Constants, `Config`/`Quality` structs, `ConfigPath`, `ValidQualities`, `IsValidQuality`, `LoadConfig` (defaults + warning), `SaveConfig` (load-merge-save) |
| `internal/config/config_test.go` | **NEW** | AQ-001…AQ-004 table-driven tests in `t.TempDir()` |
| `internal/adapters/downloader/ytdlp.go` | **MODIFIED** | `Option`/`WithAudioBitrate`, `SetAudioBitrate`, pure `buildArgs`, port guard |
| `internal/adapters/downloader/ytdlp_test.go` | **MODIFIED** | `buildArgs` table tests (AQ-005/AQ-006), no yt-dlp |
| `internal/core/service/orchestrator.go` | **MODIFIED** | `SetAudioQuality` passthrough via local `qualitySetter` |
| `internal/core/service/orchestrator_test.go` | **MODIFIED** | mock setter + `TestSetAudioQualityForwardsToDownloader` |
| `internal/tui/model.go` | **MODIFIED** | `ScreenConfig`, 4 config fields, `NewModel` param |
| `internal/tui/update.go` | **MODIFIED** | `openConfig`, `handleConfigKeys`, `confirmQuality`, `c` cases ×4, routing case |
| `internal/tui/view.go` | **MODIFIED** | `renderConfigView`, dispatch, Config footer |
| `internal/tui/keys.go` | **MODIFIED** | `{"c", "Configure quality"}` |
| `internal/tui/update_test.go` | **MODIFIED** | AQ-008…AQ-013 scenario tests |
| `internal/tui/view_test.go` | **NEW** | AQ-014/AQ-015 rendering + help tests |
| `cmd/music-dl/main.go` | **MODIFIED** | quality load (non-fatal), `WithAudioBitrate`, `NewModel` param |
| `README.md` | **MODIFIED** | `c` control row + roadmap update |
