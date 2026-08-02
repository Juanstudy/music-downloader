# Proposal: Configurable Audio Quality (MP3 128k / 192k / 320k)

**Change:** `audio-quality`
**Status:** Proposed
**Date:** 2026-08-01

---

## 1. Problem / Opportunity

music-dl downloads every track as MP3 with a hardcoded `yt-dlp` invocation: `-x --audio-format mp3` and **no bitrate flag** (`internal/adapters/downloader/ytdlp.go:33-41`). What bitrate the user gets is whatever yt-dlp's default happens to be, and it is not visible or controllable anywhere in the product.

That is a real product gap:

- A **data-conscious user** on a metered connection or with a large library wants small files (128k) and currently has no way to get them.
- A **quality-focused user** wants the best MP3 the source allows (320k) and cannot request it.
- There is **no settings surface at all** — the app only has input/resolving/playlist/downloading/done screens, so the product cannot grow user preferences without one.

The README roadmap already promises "Calidad configurable (128k, 320k, lossless)". This change delivers the lossy half of that promise with a persisted, user-visible setting, and explicitly defers lossless (see Non-Goals).

## 2. Target Users & Situations

| User | Situation |
| ------ | ----------- |
| Casual listener | Downloads a handful of songs; wants the best quality by default without touching anything |
| Data-conscious user | Downloads large playlists on a metered connection; wants 128k to save space/bandwidth |
| Quality-conscious user | Rips for archival/offline listening; wants 320k explicitly and to keep that choice between sessions |
| Current user | Never asked for a bitrate and never saw one; the app now documents and defaults it (320k) |

## 3. Current-State Gap

- `internal/adapters/downloader/ytdlp.go:33-41` hardcodes `-x --audio-format mp3 --embed-metadata --embed-thumbnail --add-metadata -o <tmpl> --no-warnings <url>` — no `--audio-bitrate`.
- `NewDownloader()` (`ytdlp.go:22-26`) takes no options and hardcodes the binary; there is no seam to inject configuration.
- No configuration screen exists. The TUI has 5 screens (`ScreenInput`, `ScreenResolving`, `ScreenPlaylist`, `ScreenDownloading`, `ScreenDone` — `internal/tui/model.go:16-24`) and a `PrevScreen` field already used for back navigation.
- Config handling lives in `internal/adapters/spotify/config.go`: `ConfigPath()` (XDG-aware → `~/.config/music-dl/config.toml`), a `Config` struct with **only** a `[spotify]` section, and `LoadConfig` returning `(nil, nil)` when the file is missing. `main.go:55-63` reads only the Spotify section. There is no write path and no `[quality]` section.
- Keybindings: global `ctrl+c` / `q` / `?` (`update.go:41-46`); per-screen handlers own everything else. `c` is free on every screen (verified: no `case "c"` anywhere in `update.go`).
- `helpContent` is a static global list (`keys.go:17-33`) — any new key appears in the help overlay on every screen (existing accepted behavior).
- The port signature `ports.Downloader.Download(ctx, media, outputDir)` (`internal/core/ports/downloader.go:16-18`) is consumed by `Orchestrator.DownloadTrack` and must not change.

## 4. Proposed Solution

### 4.1 New shared config package: `internal/config`

A new package owning the config file path, the `[quality]` section, and a safe write path:

- `Quality` constants: `Quality128 = "128k"`, `Quality192 = "192k"`, `Quality320 = "320k"`; `DefaultQuality = Quality320`.
- `ConfigPath()` — XDG-aware path to `~/.config/music-dl/config.toml` (same logic as `spotify.ConfigPath()`, now the canonical copy).
- `Config` struct with `Quality` and `Spotify` subsections mirroring the existing `[spotify]` tags.
- `LoadConfig(path)` — returns `DefaultQuality` when the file is missing, the `[quality]` section is missing, or the value is not one of the three valid bitrates (invalid values: log warning + fallback to 320k). Malformed TOML still returns an error (same contract as `spotify.LoadConfig`).
- `SaveConfig(path, cfg)` — **load-merge-save**: reads the existing file first so a write from the TUI never drops the `[spotify]` section, creates the `music-dl` directory if needed, and writes both sections back.

`internal/adapters/spotify/config.go` is **left untouched** in this change (no behavior change, no risk to the Spotify flow). The duplicated path logic is accepted for this slice; making `spotify` delegate to `internal/config` is a follow-up cleanup, not part of this change.

### 4.2 Downloader: constructor injection, port untouched

- `NewDownloader(opts ...Option)` with a functional option `WithAudioBitrate(q string)` (defaults to today's behavior when no option is passed, keeping existing tests green).
- Add `SetAudioBitrate(q string)` so a quality change made in the TUI takes effect for subsequent downloads **in the same session** without a restart.
- Extract the argument list into a pure helper `buildArgs(media, outputDir, bitrate string) []string` that inserts `--audio-bitrate <bitrate>` immediately after `--audio-format mp3`. This makes the flag testable with a table-driven unit test and **no real yt-dlp**.
- The `Downloader` port interface and `Orchestrator.DownloadTrack` signature stay byte-for-byte identical.

### 4.3 Orchestrator passthrough

- `Orchestrator` gains `SetAudioQuality(q string)` delegating to the injected downloader's `SetAudioBitrate`. No port change, no new dependency.

### 4.4 TUI: new Config screen

- New screen constant `ScreenConfig` (`model.go`), plus state fields `audioQuality string` and `qualityCursor int`.
- **Opening:** pressing `c` opens Config from the Resolving, Playlist, Downloading, and Done screens (`openConfig()` sets `Screen = ScreenConfig`, `PrevScreen` = current screen, resets the cursor).
  - **Deliberate exception:** `c` is **not** intercepted on the Input screen — URLs and search queries contain the letter `c` (e.g. `music.youtube.com/watch?v=...`) and capturing it would break typing. This follows the existing per-screen interception pattern (`s` is already intercepted per-screen, not globally).
  - On the Playlist screen the existing `isFiltering` guard runs before the key switch, so typing a filter containing `c` is automatically safe.
- **Navigation:** `j`/`k` move the cursor across the three options (128k / 192k / 320k), `Enter` confirms, `Esc` cancels without changes. `q` still quits (global), `?` still toggles help.
- **Confirm flow:** update `m.audioQuality`, call `m.orchestrator.SetAudioQuality(q)`, persist via `internal/config.SaveConfig`, then return to `PrevScreen`. Persistence failures are non-fatal: show a warning line in the config view, keep the in-session value.
- **Render:** `renderConfigView()` lists the three options with a cursor/selection indicator (following the `renderInputView` Source/Search indicator style), the current effective quality, and a footer hint (`j/k move · Enter confirm · Esc back`). Add `case ScreenConfig:` to the `View()` dispatch (`view.go:24-35`) and update the per-screen footer.
- `keys.go`: add `{"c", "Configure quality"}` to `helpContent` (global help overlay, consistent with existing behavior).

### 4.5 Wiring in `main.go`

- Load the quality config via `internal/config` (default 320k when absent).
- Pass it to the downloader: `downloader.NewDownloader(downloader.WithAudioBitrate(q))`.
- Pass the initial quality to the TUI so the Config screen renders the persisted value. `NewModel` is the only constructor used by `main.go` (TUI tests build `Model{}` literals), so adding one parameter there is contained; alternatively a post-construction `SetAudioQuality` — decided in design.

### 4.6 yt-dlp behavior

- With a quality set, `Download` invokes: `yt-dlp -x --audio-format mp3 --audio-bitrate 320k --embed-metadata ...` (bitrate omitted only if no option was injected).
- `--audio-bitrate` applies to lossy formats only — correct for our MP3-only scope.
- yt-dlp **never up-samples**: a source encoded at a lower bitrate will not be magically upgraded to 320k. Documented in the config screen copy ("lower when source is lower").

## 5. Scope / First Slice

| In scope | Out of scope |
| ---------- | ------------ |
| `internal/config` package (path, load, save, `[quality]`, defaults) | Lossless / FLAC / `--audio-format flac` |
| `[quality]` persisted in `~/.config/music-dl/config.toml` | "best" quality option |
| `ScreenConfig` TUI screen + `c` keybinding (Resolving/Playlist/Downloading/Done) | Per-track quality override |
| `--audio-bitrate` in the downloader args (constructor injection + same-session setter) | Any other config settings (theme, output dir, binary path, …) |
| `Orchestrator.SetAudioQuality` passthrough | Growing the Config screen beyond the three bitrates |
| Default 320k applied to users who never touch settings | Making `c` open Config from the Input screen while typing (deliberate exception) |
| Update `openspec/specs/adapters-downloader/spec.md` (adds bitrate requirement, keeps `--audio-format mp3`) | `spotify/config.go` refactor to delegate to `internal/config` (follow-up) |
| README controls + roadmap update | AltScreen drift fix (`tea.WithAltScreen`) |
| Tests: config round-trip, args builder, TUI transitions | |

## 6. Non-Goals (Explicit)

- **NO FLAC / lossless.** MP3 is a lossy format; lossless would require `--audio-format flac` and a different bitrate model. The roadmap's "lossless" item stays open for a separate change.
- **NO "best" quality option.** Exactly three choices: 128k, 192k, 320k.
- **NO per-track quality.** Quality is a global preference.
- **NO config-screen expansion** — no dark/light theme, no output-dir setting, no binary path, nothing else.
- Does NOT change the `Downloader` port signature or `Orchestrator.DownloadTrack`.
- Does NOT touch the Spotify adapter's config loading or resolution flow.
- Does NOT change the global `q`/`?` handling and does not add `c` to the Input screen (typing letters must keep working there).

## 7. Product Constraints

| Constraint | Decision |
| ------------ | ---------- |
| Requires yt-dlp? | Yes — already a hard requirement; `--audio-bitrate` is supported by yt-dlp for lossy formats |
| Config file location | `~/.config/music-dl/config.toml` (XDG-aware), same file Spotify already reads |
| Default when file/section missing | 320k — every user who never touches settings gets 320k |
| Persistence | Choice survives restarts via `[quality]` section |
| Effect of changing quality mid-session | Applies to the next download (setter on the downloader), no restart needed |
| Effect of deleting the config file | App falls back to 320k; no crash |
| Upsampling | Not performed by yt-dlp — requested 320k may yield lower if the source is lower |

## 8. Business Trade-offs

| Trade-off | Implication |
| ----------- | ------------- |
| 320k default vs. smaller defaults | Best-quality-by-default matches the "downloader for personal music" positioning; slight file-size cost for the majority who never change it |
| `c` not intercepted on the Input screen | Config is one key away from every screen except the input field; URLs and search queries with the letter `c` keep working — worth the small inconsistency |
| Persisting via load-merge-save | Slightly more code than a plain write, but guarantees the Spotify section is never dropped when a user changes quality |
| Constructor injection + setter instead of changing the port | Port stays stable (orchestrator + adapters + compliance tests untouched); the downloader carries a tiny mutable bit of config state |
| Duplicated path logic in `spotify` and `internal/config` | Accepted duplication for this slice; a follow-up cleanup can make `spotify` delegate to the shared package |

## 9. Edge Cases

| Edge Case | Behavior |
| ----------- | ---------- |
| Config file does not exist | Quality = 320k; first save creates `~/.config/music-dl/` + `config.toml` |
| `[quality]` missing from an existing file | 320k; Spotify section preserved on next save |
| Invalid value in `[quality]` (e.g. `"999k"` or `"flac"`) | Warning + fallback to 320k; no crash |
| Malformed TOML | Load error surfaced (same contract as Spotify); app continues with 320k |
| User changes quality mid-download | Applies to the next track in the queue (downloader setter); current track unaffected |
| `Esc` from Config | No change, no write, returns to previous screen |
| `Enter` when save fails (permissions/disk) | In-session quality applied; warning shown; not fatal |
| `c` while typing a URL or search query on Input | Types the letter (no intercept) |
| `c` while typing a filter on Playlist | Types the letter (filter guard runs first) |
| Source track encoded below the requested bitrate | Download completes at the source's bitrate; UI copy notes this |
| Config screen + help overlay | `c` listed in global help; consistent with existing global `helpContent` behavior |

## 10. Risks

| Risk | Mitigation |
| ------ | ------------ |
| `q` is a **global quit** (`update.go:41-46`) | Do not reuse `q`; `c` is verified free on every screen |
| Capturing `c` globally would break URL/search typing (every YouTube URL contains `c`) | Intercept per-screen, never on ScreenInput; Playlist filter guard already precedes the key switch |
| `NewModel` signature change breaks `main.go` | `main.go` is the only caller and is updated in the same change; TUI tests use `Model{}` literals and are unaffected; design decides param-vs-setter |
| `SaveConfig` could drop the `[spotify]` section | Load-merge-save is a stated requirement of `internal/config`; covered by a round-trip test in `t.TempDir()` |
| Quality changes mid-session not applied | Setter path through `Orchestrator.SetAudioQuality`; covered by TUI confirm-flow test |
| Spinner only ticks on ScreenResolving | Config screen has no spinner — no `spinner.TickMsg` concern; downloads keep running while Config is open |
| `helpContent` is global | Accepted existing behavior; single `c` entry added |
| yt-dlp `--audio-bitrate` flag drift | Flag is standard yt-dlp; args builder is unit-tested without invoking yt-dlp; integration tests remain `testing.Short()`-gated |
| Diff size grows past a single-PR budget | Target < 400 lines (~350-400 estimated, see Impact); keep config and args tests table-driven and TUI tests scenario-based |

## 11. Rollback

The change is additive and reversible:

1. Revert TUI changes (model, update, view, keys) — screens, `c` binding, config view.
2. Remove `internal/config`; restore `NewDownloader()` to its no-options form (the option pattern is backward compatible, so even keeping it compiles).
3. Revert `main.go` wiring and the orchestrator passthrough.
4. Revert the spec/README updates.

Config files already written with a `[quality]` section are **harmless after rollback**: BurntSushi TOML decoding ignores unknown sections, and the current `spotify.Config` struct does not read `[quality]`. No data migration needed.

## 12. Success Criteria

| Criterion | Measurement |
| ----------- | ------------- |
| Default quality is 320k | Fresh run with no config file downloads with `--audio-bitrate 320k` in the args |
| All three options work | 128k / 192k / 320k each produce the corresponding `--audio-bitrate` flag |
| Config screen navigation | `c` opens Config from Resolving/Playlist/Downloading/Done; `j`/`k` move; `Enter` confirms; `Esc` cancels without writing |
| `c` never hijacks typing | URL/search input and playlist filter still accept the letter `c` |
| Persistence | Choosing 192k, quitting, and relaunching shows 192k selected and downloads at 192k |
| Spotify section survives | After a config save, the `[spotify]` section in `config.toml` is unchanged (round-trip test) |
| No port breakage | `Downloader` port and `Orchestrator.DownloadTrack` signatures unchanged; existing adapter/orchestrator tests pass |
| No regressions | `go test ./...` passes; existing URL-mode resolution and download flow unaffected |

## 13. Impact Summary

**Files affected:**

| File | Change |
| ------ | ------ |
| `internal/config/config.go` (new) | Path, `[quality]` load/save, defaults (~75 lines) |
| `internal/config/config_test.go` (new) | Table-driven round-trip, defaults, invalid values, XDG path (~65 lines) |
| `internal/adapters/downloader/ytdlp.go` | Options + setter + `buildArgs` extraction (~35 lines) |
| `internal/adapters/downloader/ytdlp_test.go` | `buildArgs` table-driven tests (~35 lines) |
| `internal/core/service/orchestrator.go` (+ test) | `SetAudioQuality` passthrough (~12 lines) |
| `internal/tui/model.go` | `ScreenConfig`, `audioQuality`, `qualityCursor` (~8 lines) |
| `internal/tui/update.go` | `openConfig`, `handleConfigKeys`, `c` cases (~50 lines) |
| `internal/tui/view.go` | `renderConfigView`, dispatch case, footer (~30 lines) |
| `internal/tui/keys.go` | `c` help entry (~2 lines) |
| `internal/tui/*_test.go` | Config screen transition/confirm/cancel scenarios (~60 lines) |
| `cmd/music-dl/main.go` | Load quality, inject into downloader + TUI (~8 lines) |
| `openspec/specs/adapters-downloader/spec.md` | Add `--audio-bitrate` requirement (~4 lines) |
| `README.md` | Controls + roadmap (~8 lines) |

**Review workload forecast:** estimated **~350–400 changed lines** across 12-13 files — within single-PR budget by design. Medium risk of crossing 400 if tests expand; mitigation is table-driven config/args tests and scenario-based TUI tests. If the applied diff crosses 400, the review workload guard flags it and delivery strategy (`ask-on-risk`) applies before PR.

## 14. Proposal Question Round (assumptions for review)

Product decisions 1-4 are locked (levels, Config screen, 320k default, TOML persistence). The following micro-decisions were resolved by assumption and are worth a quick user check:

1. **`c` is not intercepted on the Input screen** — it types normally so URLs/search queries with the letter `c` keep working (every YouTube URL contains `c`). Config stays one key away from all other screens. OK, or must Config be reachable from Input too?
2. **Mid-session effect:** confirming a quality change in Config applies to the *next* download immediately, without restarting. OK?
3. **Invalid values in `[quality]`** (e.g. `"999k"`) fall back to 320k with a warning, instead of erroring. OK?
4. **Save must preserve `[spotify]`** — load-merge-save round-trip so changing quality never wipes Spotify credentials. OK?
5. **`c` appears in the global help overlay** on every screen (existing `helpContent` behavior). OK?

---
