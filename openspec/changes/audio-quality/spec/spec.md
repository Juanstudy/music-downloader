# Specification: Configurable Audio Quality (MP3 128k / 192k / 320k)

**Change:** `audio-quality`
**Date:** 2026-08-01

## Purpose

Give users control over the MP3 bitrate that yt-dlp produces, backed by a persisted, user-visible setting. Every download gets an explicit bitrate: the default is 320k, so users who never touch settings get the best MP3 the source allows. A new Config screen lets data-conscious and quality-conscious users pick between exactly three levels (128k / 192k / 320k). The choice survives restarts through a `[quality]` section in the existing `~/.config/music-dl/config.toml`, applies to the next download mid-session without a restart, and never interferes with typing the letter `c` in the URL/search input or the playlist filter.

This change delivers the lossy half of the README roadmap item "Calidad configurable (128k, 320k, lossless)". Lossless / FLAC and a "best" option are explicitly out of scope.

## Business Rules

- Exactly three quality levels: `128k`, `192k`, `320k`. No "best" option, no per-track override — quality is a global preference.
- Default quality is `320k`, applied whenever the config file, the `[quality]` section, or a valid value is absent.
- Quality persists in `~/.config/music-dl/config.toml` under `[quality]` — the same XDG-aware file the Spotify adapter already reads.
- Saving quality MUST NOT drop the existing `[spotify]` section (load-merge-save).
- A quality change applies to the next download; a download already in flight is not affected.
- yt-dlp never up-samples: a source encoded below the requested bitrate is downloaded at the source's bitrate.

---

## DOMAIN: internal-config — FULL SPEC (New Domain)

No canonical spec exists at `openspec/specs/internal-config/spec.md`. This domain is a new package.

### Purpose

Own the config file path, the `[quality]` section, and a safe write path so the TUI can read and persist the audio quality without touching the Spotify adapter's config handling.

### ADDED Requirement: AQ-001 — Exactly three quality levels with 320k default

The system MUST define exactly three valid audio quality levels — `128k`, `192k`, `320k` — and MUST use `320k` as the default quality. Any other value MUST NOT be a valid level.

#### Scenario: the three levels are valid and distinct

- GIVEN the set of quality levels
- WHEN checking which values are accepted
- THEN exactly `128k`, `192k`, and `320k` MUST be accepted
- AND no other value (e.g. `64k`, `best`, `lossless`) MUST be a valid level

#### Scenario: default quality is 320k

- GIVEN a configuration with no quality value
- WHEN the effective quality is requested
- THEN it MUST be `320k`

### ADDED Requirement: AQ-002 — Config file location is XDG-aware

The system MUST resolve the config file path to `$XDG_CONFIG_HOME/music-dl/config.toml`, falling back to `~/.config/music-dl/config.toml` when `XDG_CONFIG_HOME` is not set. This MUST be the same file the Spotify adapter reads.

#### Scenario: XDG_CONFIG_HOME is set

- GIVEN an environment with `XDG_CONFIG_HOME=/custom/config`
- WHEN the config path is resolved
- THEN it MUST be `/custom/config/music-dl/config.toml`

#### Scenario: XDG_CONFIG_HOME is unset

- GIVEN an environment without `XDG_CONFIG_HOME`
- WHEN the config path is resolved
- THEN it MUST be `~/.config/music-dl/config.toml` (user home expanded)

### ADDED Requirement: AQ-003 — Missing or invalid quality falls back to 320k

The system MUST return the default quality (`320k`) when the config file does not exist, when the `[quality]` section is absent, and when the stored value is not one of the three valid levels (invalid values MUST produce a warning and MUST NOT crash). Malformed TOML MUST surface an error to the caller, and the application MUST continue running with `320k`.

#### Scenario: config file does not exist

- GIVEN no config file at the resolved path
- WHEN quality is loaded
- THEN the effective quality MUST be `320k`
- AND no error MUST be returned

#### Scenario: [quality] section is missing from an existing file

- GIVEN an existing config file with a `[spotify]` section but no `[quality]` section
- WHEN quality is loaded
- THEN the effective quality MUST be `320k`

#### Scenario: invalid quality value falls back with a warning

- GIVEN a config file with `[quality]` set to `"999k"` (and likewise for `"flac"` or any non-level value)
- WHEN quality is loaded
- THEN a warning MUST be logged
- AND the effective quality MUST be `320k`
- AND the load MUST NOT fail

#### Scenario: malformed TOML surfaces an error, app continues at 320k

- GIVEN a config file that is not valid TOML
- WHEN quality is loaded
- THEN the loader MUST return a non-nil error (same contract as the Spotify config loader)
- AND the application MUST NOT crash
- AND the effective quality used by the app MUST be `320k`

#### Scenario: deleting the config file recovers gracefully

- GIVEN a user deletes the config file between sessions
- WHEN the app starts
- THEN the effective quality MUST be `320k`
- AND the app MUST NOT crash

### ADDED Requirement: AQ-004 — Saving quality preserves the Spotify section

The system MUST persist a quality change by reading the existing config file first and writing back both the `[quality]` and `[spotify]` sections (load-merge-save), so a save from the TUI never drops the Spotify section. The first save MUST create the `music-dl` directory and the `config.toml` file when they do not exist.

#### Scenario: round-trip preserves the [spotify] section

- GIVEN a config file containing a `[spotify]` section with credentials
- WHEN a quality change is saved
- THEN the `[quality]` section MUST contain the new value
- AND the `[spotify]` section MUST be byte-identical to its pre-save content

#### Scenario: first save creates directory and file

- GIVEN no `music-dl` directory and no config file
- WHEN a quality change is saved
- THEN the `music-dl` directory MUST be created
- AND the `config.toml` file MUST be created with the `[quality]` section set to the saved value

---

## DOMAIN: adapters-downloader — MODIFIED Requirements

Canonical at `openspec/specs/adapters-downloader/spec.md` (updated in this change, see that file). The `Downloader` port signature MUST NOT change.

### MODIFIED Requirement: AQ-005 — Downloader invokes yt-dlp with audio-extraction flags and bitrate

The system MUST provide a `Downloader` implementation that shells out to `yt-dlp` for audio extraction. The downloader MUST accept an optional audio bitrate; when set, the invocation MUST include `--audio-bitrate <bitrate>` inserted immediately after `--audio-format mp3`; when unset, the invocation MUST be identical to the pre-change behavior with no bitrate flag.

```go
type Downloader struct{}

func NewDownloader(opts ...Option) *Downloader
func (d *Downloader) Download(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error)
```

(Previously: the downloader hardcoded `-x --audio-format mp3 --embed-metadata ...` with no bitrate flag and no seam to set one.)

#### Scenario: Download calls yt-dlp with correct audio flags

- GIVEN a `Downloader` instance and a resolved `domain.Media`
- WHEN `Download(ctx, media, outputDir)` is called
- THEN the executable `yt-dlp` MUST be invoked
- AND the arguments MUST include `-x` (extract audio)
- AND the arguments MUST include `--audio-format mp3`
- AND the arguments MUST include `--embed-metadata`
- AND the arguments MUST include `-o` with a template like `{artist} - {title}.mp3`
- AND the output template path MUST be prefixed with `outputDir`

#### Scenario: bitrate flag is inserted after --audio-format mp3 when set

- GIVEN a `Downloader` configured with quality `192k`
- WHEN `Download(ctx, media, outputDir)` is called
- THEN the arguments MUST include `--audio-bitrate 192k`
- AND `--audio-bitrate 192k` MUST appear immediately after `--audio-format mp3`

#### Scenario: no bitrate flag when no quality is configured

- GIVEN a `Downloader` constructed without a quality option
- WHEN `Download(ctx, media, outputDir)` is called
- THEN the arguments MUST NOT include `--audio-bitrate`

#### Scenario: source below requested bitrate is not up-sampled

- GIVEN a `Downloader` configured with quality `320k`
- WHEN downloading a track whose source is encoded at `128k`
- THEN the download MUST complete successfully at the source's bitrate
- AND yt-dlp MUST NOT be instructed to up-sample beyond the source

### ADDED Requirement: AQ-006 — Quality is configurable at construction and mid-session

The system MUST allow the downloader to be constructed with a target bitrate, defaulting to the pre-change no-flag behavior when no option is passed, and MUST provide a setter so a quality change takes effect for subsequent downloads in the same session without a restart.

#### Scenario: each quality level produces the corresponding flag

- GIVEN a `Downloader` constructed with `128k`, then one with `192k`, then one with `320k`
- WHEN `Download` is called for each
- THEN the arguments MUST include `--audio-bitrate 128k`, `--audio-bitrate 192k`, and `--audio-bitrate 320k` respectively

#### Scenario: no option keeps today's behavior

- GIVEN a `Downloader` constructed with no options
- WHEN `Download` is called
- THEN the invocation MUST be identical to the pre-change invocation
- AND existing downloader tests MUST pass unchanged

#### Scenario: mid-session setter affects the next download

- GIVEN a `Downloader` running with quality `128k`
- WHEN the setter changes quality to `320k`
- THEN the next `Download` call MUST use `--audio-bitrate 320k`

#### Scenario: change mid-download leaves the in-flight track unaffected

- GIVEN a download in progress for track A at `128k`
- WHEN the quality is changed to `320k` while track A is downloading
- THEN track A MUST complete with `--audio-bitrate 128k`
- AND track B (next in the queue) MUST use `--audio-bitrate 320k`

---

## DOMAIN: core-service — ADDED Requirements

Canonical at `openspec/specs/core-service/spec.md`.

### ADDED Requirement: AQ-007 — Orchestrator exposes SetAudioQuality passthrough

The system MUST expose an `Orchestrator.SetAudioQuality(q string)` method that forwards the value to the downloader's quality setter. The `Downloader` port interface and the `Orchestrator.DownloadTrack` signature MUST remain byte-for-byte unchanged.

#### Scenario: SetAudioQuality forwards to the downloader

- GIVEN an `Orchestrator` with an injected downloader
- WHEN `SetAudioQuality("192k")` is called
- THEN the downloader's quality MUST be `192k`
- AND the next `DownloadTracks` call MUST download at `192k`

#### Scenario: port signatures are unchanged

- GIVEN the `ports.Downloader` interface and `Orchestrator.DownloadTrack`
- WHEN compared against the pre-change signatures
- THEN they MUST be identical (no added, removed, or renamed methods or parameters)

---

## DOMAIN: internal-tui — ADDED Requirements

Canonical at `openspec/specs/internal-tui/spec.md`. The TUI gains a sixth screen (`ScreenConfig`); the other five screens keep their behavior.

### ADDED Requirement: AQ-008 — Config screen constant and state

The system MUST add a `ScreenConfig` screen constant and `audioQuality` / `qualityCursor` state fields to the TUI model.

#### Scenario: model carries the new state

- GIVEN the `Model` struct
- WHEN inspecting its fields
- THEN it MUST include a `ScreenConfig` value in the `Screen` enum
- AND an `audioQuality` field holding the current effective quality
- AND a `qualityCursor` field for the config option cursor

#### Scenario: initial quality reflects the persisted value

- GIVEN the TUI is started with a persisted quality of `192k`
- WHEN the Config screen is first opened
- THEN the effective quality MUST be `192k`
- AND the cursor MUST point at the `192k` option

### ADDED Requirement: AQ-009 — `c` opens Config from Resolving, Playlist, Downloading, and Done

Pressing `c` MUST open the Config screen from `ScreenResolving`, `ScreenPlaylist`, `ScreenDownloading`, and `ScreenDone`, recording the current screen as `PrevScreen` and resetting the cursor to the first option.

#### Scenario: `c` on Resolving opens Config

- GIVEN a `Model` on `ScreenResolving`
- WHEN `Update()` receives the key `c`
- THEN `m.Screen` MUST be `ScreenConfig`
- AND `m.PrevScreen` MUST be `ScreenResolving`

#### Scenario: `c` on Playlist opens Config

- GIVEN a `Model` on `ScreenPlaylist`
- WHEN `Update()` receives the key `c`
- THEN `m.Screen` MUST be `ScreenConfig`
- AND `m.PrevScreen` MUST be `ScreenPlaylist`

#### Scenario: `c` on Downloading opens Config

- GIVEN a `Model` on `ScreenDownloading`
- WHEN `Update()` receives the key `c`
- THEN `m.Screen` MUST be `ScreenConfig`
- AND `m.PrevScreen` MUST be `ScreenDownloading`

#### Scenario: `c` on Done opens Config

- GIVEN a `Model` on `ScreenDone`
- WHEN `Update()` receives the key `c`
- THEN `m.Screen` MUST be `ScreenConfig`
- AND `m.PrevScreen` MUST be `ScreenDone`

### ADDED Requirement: AQ-010 — `c` is never intercepted while typing

The system MUST NOT intercept `c` on the Input screen and MUST NOT intercept `c` while a playlist filter is being typed; in both cases the letter MUST be inserted as normal text. (Deliberate exception: URLs and search queries contain the letter `c`, e.g. `music.youtube.com/watch?v=...`.)

#### Scenario: typing a URL with `c` inserts the letter

- GIVEN a `Model` on `ScreenInput` with `InputText == "music.youtube.com/watch"` and the input focused
- WHEN `Update()` receives the key `c`
- THEN `m.Screen` MUST remain `ScreenInput`
- AND the character `c` MUST be appended to `InputText`

#### Scenario: typing a filter with `c` inserts the letter

- GIVEN a `Model` on `ScreenPlaylist` with `isFiltering == true` and an active filter input
- WHEN `Update()` receives the key `c`
- THEN the filter MUST NOT be closed or changed into a navigation action
- AND the character `c` MUST be appended to the filter text

#### Scenario: `c` on non-typing screens opens Config

- GIVEN a `Model` on `ScreenResolving`, `ScreenPlaylist` (not filtering), `ScreenDownloading`, or `ScreenDone`
- WHEN `Update()` receives the key `c`
- THEN `m.Screen` MUST become `ScreenConfig`

### ADDED Requirement: AQ-011 — Config screen navigation (j/k, Enter, Esc)

The system MUST let `j`/`k` move the cursor across the three options (bounded, never below the first or above the last option), `Enter` confirm the selection, and `Esc` cancel without any changes or writes. Global `q` (quit) and `?` (help) MUST keep working on the Config screen.

#### Scenario: j/k move the cursor within bounds

- GIVEN a `Model` on `ScreenConfig` with `qualityCursor` at the first option
- WHEN `Update()` receives `k`
- THEN the cursor MUST NOT move below the first option
- WHEN `Update()` receives `j`
- THEN the cursor MUST move to the second option
- AND at the last option, a further `j` MUST NOT move past it

#### Scenario: Esc cancels without changes

- GIVEN a `Model` on `ScreenConfig` opened from `ScreenPlaylist`
- WHEN `Update()` receives `Esc`
- THEN `m.Screen` MUST return to `ScreenPlaylist`
- AND `m.audioQuality` MUST be unchanged
- AND no config write MUST be performed

#### Scenario: q still quits and ? still toggles help

- GIVEN a `Model` on `ScreenConfig`
- WHEN `Update()` receives `q`
- THEN a `tea.Quit` command MUST be returned
- WHEN `Update()` receives `?`
- THEN the help overlay MUST toggle

### ADDED Requirement: AQ-012 — Confirming applies quality mid-session and persists

Confirming a selection with `Enter` MUST update the in-session effective quality, apply it to the downloader (so the next download uses it), persist it via the config writer, and return to `PrevScreen`.

#### Scenario: Enter confirms and persists

- GIVEN a `Model` on `ScreenConfig` opened from `ScreenPlaylist` with `qualityCursor` on `192k`
- WHEN `Update()` receives `Enter`
- THEN `m.audioQuality` MUST be `192k`
- AND the downloader's quality MUST be set to `192k` (next download uses it)
- AND the config file MUST be written with `[quality] = "192k"`
- AND `m.Screen` MUST return to `ScreenPlaylist`

#### Scenario: persisted choice survives a restart

- GIVEN a user confirms `192k` in the Config screen and quits
- WHEN the app is relaunched
- THEN the Config screen MUST show `192k` selected
- AND downloads MUST use `--audio-bitrate 192k`

### ADDED Requirement: AQ-013 — Save failure is non-fatal

If persisting the quality fails (e.g. permissions or disk error), the in-session quality MUST still apply, a warning MUST be shown to the user, and the application MUST NOT crash.

#### Scenario: failed save keeps in-session value and shows a warning

- GIVEN a `Model` on `ScreenConfig` where the config write fails
- WHEN the user confirms a quality
- THEN the in-session effective quality MUST be the confirmed value
- AND the downloader MUST be updated to the confirmed value
- AND a warning MUST be displayed in the config view
- AND the application MUST continue running (no crash, no forced quit)

### ADDED Requirement: AQ-014 — Config view renders options, current quality, and footer

The Config view MUST list the three options (`128k`, `192k`, `320k`) with a cursor/selection indicator, display the current effective quality, and show a footer hint (`j/k move · Enter confirm · Esc back`). `View()` MUST dispatch to this view when `m.Screen == ScreenConfig`.

#### Scenario: view shows all three options and the cursor

- GIVEN a `Model` on `ScreenConfig` with `qualityCursor` on `320k`
- WHEN `View()` is called
- THEN the output MUST include the options `128k`, `192k`, and `320k`
- AND the `320k` option MUST be visually marked as selected/cursor-positioned

#### Scenario: view shows the current quality and footer

- GIVEN a `Model` on `ScreenConfig` with `audioQuality == "192k"`
- WHEN `View()` is called
- THEN the output MUST indicate the current effective quality is `192k`
- AND the footer MUST include hints for `j`/`k` (move), `Enter` (confirm), and `Esc` (back)

### ADDED Requirement: AQ-015 — Help overlay lists the `c` binding

The global help overlay MUST include a `c` entry describing the Config screen (e.g. "Configure quality"), consistent with the existing global `helpContent` behavior of appearing on every screen.

#### Scenario: help overlay shows the c entry

- GIVEN a `Model` on any screen
- WHEN the help overlay is rendered
- THEN it MUST include an entry for the key `c` describing the Config screen

---

## DOMAIN: cmd-entrypoint — ADDED Requirements

Canonical at `openspec/specs/cmd-entrypoint/spec.md`.

### ADDED Requirement: AQ-016 — main wires quality into the downloader and TUI

The `main()` function MUST load the quality via the new config package (default `320k` when absent), construct the downloader with that bitrate, and pass the initial quality to the TUI so the Config screen renders the persisted value.

#### Scenario: no config file → downloader gets 320k

- GIVEN no config file exists
- WHEN `main()` executes its wiring
- THEN the downloader MUST be constructed with quality `320k`
- AND the TUI MUST receive an initial quality of `320k`

#### Scenario: persisted quality is wired through

- GIVEN a config file with `[quality] = "128k"`
- WHEN `main()` executes its wiring
- THEN the downloader MUST be constructed with quality `128k`
- AND the TUI MUST receive an initial quality of `128k`

#### Scenario: the app builds and starts

- GIVEN the wiring change
- WHEN running `go build ./cmd/music-dl/`
- THEN it MUST succeed

---

## DOMAIN: project-docs — ADDED Requirements

### ADDED Requirement: AQ-017 — README documents the quality control and roadmap

The README MUST document the `c` keybinding and Config screen controls, and MUST update the roadmap item to reflect that `128k` / `192k` / `320k` are delivered while lossless remains open.

#### Scenario: README lists the control

- GIVEN the README's controls/keybindings section
- WHEN reading it
- THEN it MUST include the `c` → Config screen binding

#### Scenario: roadmap reflects the delivered slice

- GIVEN the README's roadmap section
- WHEN reading the quality item
- THEN it MUST state that configurable lossy quality (128k/192k/320k) is delivered
- AND lossless MUST still be listed as open

---

## NON-FUNCTIONAL Requirements

### Requirement: AQ-018 — Port stability

The `Downloader` port interface (`ports.Downloader.Download(ctx, media, outputDir)`) and `Orchestrator.DownloadTrack` MUST NOT change in signature, name, or behavior. Existing adapter, orchestrator, and compliance tests MUST pass without modification.

#### Scenario: compile-time contract holds

- GIVEN the existing port and orchestrator code
- WHEN compiled with the change applied
- THEN all existing assignments and call sites MUST compile unchanged

### Requirement: AQ-019 — Spotify adapter untouched

The change MUST NOT modify `internal/adapters/spotify/config.go` or the Spotify resolution flow. The duplicated config-path logic in the Spotify adapter is accepted for this slice; delegating it to `internal/config` is a follow-up cleanup, not part of this change.

#### Scenario: spotify flow unaffected

- GIVEN the change applied
- WHEN running the Spotify adapter tests and the existing Spotify resolution flow
- THEN they MUST behave identically to the pre-change state

### Requirement: AQ-020 — No regressions

The change MUST NOT regress existing behavior: `go test ./...` MUST pass, and the URL-mode resolution and download flows MUST behave identically to the pre-change state.

#### Scenario: full test suite passes

- GIVEN the change applied
- WHEN running `go test ./...`
- THEN all tests MUST pass

#### Scenario: URL mode unchanged

- GIVEN a user who never presses `c`
- WHEN pasting a URL, resolving, and downloading
- THEN the flow MUST be identical to the pre-change behavior except for the added `--audio-bitrate 320k` flag (default quality)

---

## Test Specifications

### Test: config package (unit)

**File:** `internal/config/config_test.go`

| Case | Input | Expected |
| ------ | ------ | --------- |
| No config file | Load on temp dir | `320k`, no error |
| `[quality]` missing | File with only `[spotify]` | `320k` |
| Invalid value `"999k"` | `[quality] = "999k"` | Warning + `320k` |
| Malformed TOML | Garbage file | Non-nil error |
| Round-trip preserves `[spotify]` | File with `[spotify]`; save quality | `[quality]` new, `[spotify]` byte-identical |
| First save creates dir + file | Empty temp dir | File created with `[quality]` |
| XDG path | `XDG_CONFIG_HOME` set/unset | Path under XDG / under `~/.config` |

### Test: downloader args (unit, no real yt-dlp)

**File:** `internal/adapters/downloader/ytdlp_test.go`

| Case | Input | Expected |
| ------ | ------ | --------- |
| No option | `NewDownloader()` | No `--audio-bitrate` in args |
| `128k` / `192k` / `320k` | `WithAudioBitrate(...)` | `--audio-bitrate` immediately after `--audio-format mp3` |
| Setter mid-session | Set after construction | Next `buildArgs` call uses new bitrate |
| Existing flags intact | Any config | `-x`, `--audio-format mp3`, `--embed-metadata`, `-o` template present |

### Test: orchestrator passthrough (unit)

**File:** `internal/core/service/orchestrator_test.go`

| Case | Input | Expected |
| ------ | ------ | --------- |
| `SetAudioQuality` forwards | `SetAudioQuality("192k")` | Mock downloader records `192k` |
| Port signatures unchanged | Compile-time checks | Existing assignments compile |

### Test: TUI Config screen (unit)

**File:** `internal/tui/update_test.go`, `internal/tui/view_test.go`

| Case | Initial State | Msg | Expected |
| ------ | --------------- | ----- | --------- |
| `c` opens Config | `ScreenResolving` / `ScreenPlaylist` / `ScreenDownloading` / `ScreenDone` | Key `c` | `ScreenConfig`, `PrevScreen` set |
| `c` types on Input | `ScreenInput`, focused input | Key `c` | Letter appended, screen unchanged |
| `c` types in filter | `ScreenPlaylist`, `isFiltering` | Key `c` | Filter text gets `c` |
| `j`/`k` move cursor | `ScreenConfig` | `j`, `k` | Cursor moves, bounded |
| `Enter` confirms | `ScreenConfig`, cursor on `192k` | `Enter` | Quality set, downloader updated, config written, back to `PrevScreen` |
| `Esc` cancels | `ScreenConfig` from `ScreenPlaylist` | `Esc` | Back to `ScreenPlaylist`, no write, quality unchanged |
| Save failure non-fatal | Config write forced to fail | `Enter` | In-session quality applied, warning shown, no crash |
| `q` quits on Config | `ScreenConfig` | `q` | `tea.Quit` command |
| View renders options | `ScreenConfig`, cursor on `320k` | `View()` | Options + cursor marker + current quality + footer |
| Help shows `c` | Any screen | Help overlay | `c` entry present |

### Test: entrypoint (compilation)

| Case | Command | Expected |
| ------ | ------- | --------- |
| Build succeeds | `go build ./cmd/music-dl/` | Exit 0 |
| Full suite | `go test ./...` | Exit 0 |

---

## No-Regression Requirements

The following existing behaviors MUST remain unchanged and are covered by the requirements above:

- URL-mode resolution and download flows (AQ-020).
- Global `q` quit and `?` help behavior (AQ-011).
- The five existing screens and their transitions (AQ-008, AQ-009).
- The `Downloader` port and `Orchestrator.DownloadTrack` signatures (AQ-018).
- The Spotify adapter's config loading and resolution flow (AQ-019).
