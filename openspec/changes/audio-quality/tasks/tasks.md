# Tasks: Configurable Audio Quality (MP3 128k / 192k / 320k)

**Change:** `audio-quality`
**Date:** 2026-08-01
**Estimated total:** ~695 lines (15 files: 4 new, 11 modified) — split into 2 chained PRs
**Status:** Draft
**Test runner:** `go test ./...` (STRICT TDD: RED tests first, then GREEN implementation)

---

## Review Workload Forecast

| Field | Value |
| ------- | ------- |
| Estimated changed lines | ~695 total (PR#1 ~342 / PR#2 ~365) |
| 400-line budget risk | High (total); Low per PR after the already-decided split |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 (feature-branch-chain) |
| Delivery strategy | ask-on-risk (already resolved → 2 chained PRs) |
| Chain strategy | feature-branch-chain |

```text
Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High
```

> **Why chained:** the change totals ~695 lines (> 400). The split was decided in advance (not reopened here). Each PR lands under the 400-line budget: **PR#1 ~342**, **PR#2 ~365**. PR#2 depends on PR#1 (`internal/tui` imports `internal/config`; `main.go` uses `config.LoadConfig`; `confirmQuality` calls `orchestrator.SetAudioQuality`).

---

## Chain Context (feature-branch-chain)

```
main ──► feat/audio-quality (tracker, draft, no-merge until PR#2 merges)
                ▲
   PR #1 ────────┘   (targets tracker branch — backend slice)
        ▲
   PR #2 ─┘          (targets PR #1 branch — UI + entry + docs)
```

- Tracker PR `feat/audio-quality`: created first, kept draft/no-merge; only it merges to main at the end.
- PR #1 targets the tracker; PR #2 targets PR #1's branch. Review diffs stay focused on one work unit.
- Each PR must pass `go build ./... && go vet ./... && go test ./...` independently and be independently revertible (see Rollback Notes).
- Do not mix chain strategies; retarget/rebase if a diff gets polluted by the base.

---

## Task Overview

```
PR#1 (backend, ~342 lines):
  T1.1 RED   config tests (config_test.go) ............ ~135 lines  (no deps)
  T1.2 GREEN config package (config.go) ............... ~90 lines   (depends: T1.1)
  T1.3 RED   downloader buildArgs tests (ytdlp_test.go)  ~55 lines  (no deps)
  T1.4 GREEN downloader options/setter/buildArgs ....... ~30 lines   (depends: T1.3)
  T1.5 RED   orchestrator passthrough test ............ ~18 lines   (no deps)
  T1.6 GREEN orchestrator SetAudioQuality ............. ~10 lines   (depends: T1.5)
  T1.7 GATE  PR#1 build/test + canonical spec commit ..  —          (depends: T1.1-T1.6)

PR#2 (UI+entry, ~365 lines):
  T2.1 Model fields + ScreenConfig constant ........... ~12 lines   (depends: PR#1)
  T2.2 RED   config open/navigation tests ............. ~90 lines   (depends: T2.1)
  T2.3 GREEN update.go open/nav handlers .............. ~40 lines   (depends: T2.2)
  T2.4 RED   confirm/persist/failure tests ............ ~75 lines   (depends: T2.1)
  T2.5 GREEN update.go confirmQuality ................. ~20 lines   (depends: T2.4)
  T2.6 Wiring: NewModel param + main.go ............... ~26 lines   (depends: PR#1, T2.1)
  T2.7 RED   view tests (view_test.go, NEW) ........... ~55 lines   (depends: T2.1)
  T2.8 GREEN view.go + keys.go ........................ ~39 lines   (depends: T2.7)
  T2.9 README controls + roadmap ...................... ~8 lines    (no deps)
  T2.10 GATE PR#2 build/test + no-regression checks ...  —          (depends: T2.1-T2.9)
```

---

# PR 1 — Backend: `internal/config` + downloader + orchestrator (~342 lines)

📍 Current PR. Targets `feat/audio-quality` (tracker).

- **Start:** new `internal/config` package with zero impact on existing code.
- **End:** downloader supports `--audio-bitrate` (constructor option + mid-session setter), orchestrator exposes the passthrough, port signatures byte-for-byte unchanged.
- **Prior deps:** none.
- **Follow-up (next PR):** TUI Config screen + entry wiring + README (PR#2).
- **Out of scope:** `internal/adapters/spotify/config.go` (AQ-019, untouched), TUI, README, lossless/FLAC.
- **Requirements:** AQ-001, AQ-002, AQ-003, AQ-004, AQ-005, AQ-006, AQ-007, AQ-018.
- **Verification:** `go build ./... && go vet ./... && go test ./...` (integration tests stay `testing.Short()`-gated).

## Task T1.1 — RED: config package tests (AQ-001…AQ-004)

**Files:** `internal/config/config_test.go` (NEW)
**TDD phase:** RED — write the tests first; the package does not exist yet, so the test file must fail to compile until T1.2.

**Steps:**

1. Create `internal/config/config_test.go` (package `config`, tests in `t.TempDir()`; use `t.Setenv("XDG_CONFIG_HOME", ...)`).
2. Cover AQ-001: `TestValidQualities` (exactly `[128k, 192k, 320k]`), `TestIsValidQuality_RejectsNonLevels` (`64k`, `best`, `lossless`, `""` → invalid), `TestDefaultQuality` (`320k`).
3. Cover AQ-002: `TestConfigPath_XDGSet` (`/custom/config/music-dl/config.toml`), `TestConfigPath_XDGUnset` (`filepath.Join(home, ".config", "music-dl", "config.toml")`).
4. Cover AQ-003: `TestLoadConfig_MissingFile` (no file → `320k`, nil error), `TestLoadConfig_MissingQualitySection` (file with only `[spotify]` → `320k`), `TestLoadConfig_InvalidValueFallsBack` (`value = "999k"` and `"flac"` → warning + `320k`, nil error), `TestLoadConfig_MalformedTOML` (garbage file → non-nil error).
5. Cover AQ-004: `TestSaveConfig_RoundTripPreservesSpotify` (seed the file with `toml.Encoder` output so the `[spotify]` block is canonically encoded; save `192k`; assert `[quality] value = "192k"` and the `[spotify]` block byte-identical), `TestSaveConfig_FirstSaveCreatesDirAndFile` (empty temp dir → dir + file created with `[quality]`), `TestSaveConfig_MalformedExistingFile` (garbage existing file → non-nil error, file not clobbered).

**Verification:** `go test ./internal/config/...` fails (package not found) — the RED state.

**Acceptance:**

- [x] All AQ-001..AQ-004 cases from the spec's config test table are present, table-driven, using `t.TempDir()`/`t.Setenv` only (no user config touched). <!-- sdd-owner: implementation -->
- [x] `go test ./internal/config/...` is red at this point (compile failure proves the tests exist before the implementation). <!-- sdd-owner: implementation -->

**Estimated lines:** ~135. **Dependencies:** none. **Risk:** Low.

## Task T1.2 — GREEN: implement `internal/config` (AQ-001…AQ-004)

**Files:** `internal/config/config.go` (NEW)
**TDD phase:** GREEN — implement to make T1.1 pass; then REFACTOR (doc comments, minimal imports).

**Steps:**

1. Constants `Quality128 = "128k"`, `Quality192 = "192k"`, `Quality320 = "320k"`, `DefaultQuality = Quality320`; `Quality{Value string \`toml:"value"\`}`;`Config{Quality Quality \`toml:"quality"\`; Spotify struct{ClientID, ClientSecret} \`toml:"spotify"\`}` (exact shapes in design §2.1).
2. `ConfigPath()`: copy of `spotify.ConfigPath()` logic (XDG-aware, fallback `~/.config`) — `internal/adapters/spotify/config.go` stays untouched (AQ-019).
3. `ValidQualities()` returns `[128k, 192k, 320k]` in display order; `IsValidQuality(q)`.
4. `LoadConfig(path) (Config, error)`: missing file / missing `[quality]` → `320k`, nil; invalid value → `log.Printf("warning: invalid audio quality %q, using %s", ...)` + `320k`, nil; malformed TOML → non-nil error, zero `Config`.
5. `SaveConfig(path, cfg)`: `os.MkdirAll(filepath.Dir(path), 0o755)` → decode existing file (load-merge-save; overwrite only `merged.Quality`) → encode via `toml.NewEncoder(&buf).Encode(merged)` → `os.WriteFile(path, buf.Bytes(), 0o644)`. Malformed existing file → error, nothing written.

**Verification:** `go test ./internal/config/...` passes.

**Acceptance:**

- [x] `internal/config/config.go` implements the design §2.1 API exactly (names, structs, constants). <!-- sdd-owner: implementation -->
- [x] `go test ./internal/config/...` green; load-merge-save round-trip preserves the `[spotify]` section byte-for-byte. <!-- sdd-owner: implementation -->

**Estimated lines:** ~90. **Dependencies:** T1.1. **Risk:** Low.

## Task T1.3 — RED: downloader `buildArgs` tests (AQ-005, AQ-006)

**Files:** `internal/adapters/downloader/ytdlp_test.go` (MODIFIED — add new tests; existing integration tests unchanged)
**TDD phase:** RED — add the tests before the implementation; `buildArgs` does not exist yet.

**Steps:**

1. Table-driven `TestBuildArgs_*` calling the pure function `buildArgs(media, outputDir, bitrate)` (no yt-dlp invoked; use a `domain.Media{URL, Title, Artist}` fixture and `t.TempDir()`).
2. AQ-005: `TestBuildArgs_NoBitrate` (`""` → no `--audio-bitrate`; `-x`, `--audio-format mp3`, `--embed-metadata`, `-o` template present and prefixed with `outputDir`); `TestBuildArgs_BitratePosition` (`"192k"` → `--audio-bitrate 192k` immediately after `--audio-format mp3`); `TestBuildArgs_ExistingFlagsIntact`.
3. AQ-006: `TestWithAudioBitrate_EachLevel` (128k/192k/320k → respective flags via `d.audioBitrate`), `TestNewDownloader_NoOption` (field `""` → no flag, identical to pre-change invocation), `TestSetAudioBitrate_MidSession` (set 128k→320k, next args use 320k).
4. Add a compile-time check `var _ ports.Downloader = (*Downloader)(nil)` in the test package if the implementation guard is not yet present (AQ-018 evidence).

**Verification:** `go test ./internal/adapters/downloader/...` fails to compile (no `buildArgs`, no options) — RED.

**Acceptance:**

- [x] All AQ-005/AQ-006 args cases are covered table-driven with no real yt-dlp. <!-- sdd-owner: implementation -->
- [x] Existing integration tests untouched; `go test -short ./internal/adapters/downloader/...` red only on compile. <!-- sdd-owner: implementation -->

**Estimated lines:** ~55. **Dependencies:** none. **Risk:** Low.

## Task T1.4 — GREEN: downloader options, setter, `buildArgs` (AQ-005, AQ-006, AQ-018)

**Files:** `internal/adapters/downloader/ytdlp.go` (MODIFIED)
**TDD phase:** GREEN — implement to make T1.3 pass.

**Steps:**

1. Add `audioBitrate string` field to `Downloader`; change `NewDownloader()` → `NewDownloader(opts ...Option)` applying options in order (variadic keeps existing call sites compiling — AQ-006 "no option keeps today's behavior").
2. Add `type Option func(*Downloader)`, `WithAudioBitrate(q string) Option`, `SetAudioBitrate(q string)`.
3. Extract the args literal into pure `buildArgs(media domain.Media, outputDir, bitrate string) []string` per design §2.2 (insert `--audio-bitrate <bitrate>` immediately after `--audio-format mp3` only when `bitrate != ""`); `Download` becomes `args := buildArgs(media, outputDir, d.audioBitrate)` — everything else in `Download` unchanged.
4. Add `var _ ports.Downloader = (*Downloader)(nil)` compile-time guard (AQ-018).

**Verification:** `go test ./internal/adapters/downloader/...` passes (new unit tests + existing ones).

**Acceptance:**

- [x] `go test ./internal/adapters/downloader/...` green; port guard compiles. <!-- sdd-owner: implementation -->
- [x] `NewDownloader()` with no options produces byte-for-byte identical args to the pre-change invocation. <!-- sdd-owner: implementation -->

**Estimated lines:** ~30. **Dependencies:** T1.3. **Risk:** Low.

## Task T1.5 — RED: orchestrator passthrough test (AQ-007)

**Files:** `internal/core/service/orchestrator_test.go` (MODIFIED — extend mock, add test)
**TDD phase:** RED — `SetAudioQuality` does not exist yet.

**Steps:**

1. Extend `mockDownloader` with `audioBitrate string` field + `SetAudioBitrate(q string)` method that records the value (existing tests unaffected).
2. Add `TestSetAudioQualityForwardsToDownloader`: `NewOrchestrator(&mockSearcher{}, mock)` → `orch.SetAudioQuality("192k")` → assert `mock.audioBitrate == "192k"`.
3. Port-stability evidence: existing compile-time assignments in this package stay unchanged (AQ-018).

**Verification:** `go test ./internal/core/service/...` fails to compile — RED.

**Acceptance:**

- [x] `TestSetAudioQualityForwardsToDownloader` present; mock records the setter value. <!-- sdd-owner: implementation -->
- [x] No existing orchestrator test modified in behavior. <!-- sdd-owner: implementation -->

**Estimated lines:** ~18. **Dependencies:** none. **Risk:** Low.

## Task T1.6 — GREEN: orchestrator `SetAudioQuality` (AQ-007, AQ-018)

**Files:** `internal/core/service/orchestrator.go` (MODIFIED)
**TDD phase:** GREEN — implement to make T1.5 pass.

**Steps:**

1. Add the local optional-capability interface `qualitySetter interface { SetAudioBitrate(string) }` (core must not import the adapter package).
2. Add `SetAudioQuality(q string)` with a guarded type assertion on `o.downloader`; no-op when the injected downloader lacks the setter.
3. `ports.Downloader` and `Orchestrator.DownloadTrack` stay byte-for-byte unchanged.

**Verification:** `go test ./internal/core/service/...` passes.

**Acceptance:**

- [x] `SetAudioQuality` forwards via the `qualitySetter` assertion; port + `DownloadTrack` signatures untouched (AQ-018). <!-- sdd-owner: implementation -->
- [x] `go test ./internal/core/service/...` green. <!-- sdd-owner: implementation -->

**Estimated lines:** ~10. **Dependencies:** T1.5. **Risk:** Low.

## Task T1.7 — GATE: PR#1 verification + canonical spec commit (AQ-018, AQ-020)

**Files:** `openspec/specs/adapters-downloader/spec.md` (verify; already updated by the spec phase — ensure it ships in PR#1), verification only otherwise
**TDD phase:** REFACTOR / gate — no new behavior.

**Steps:**

1. Confirm `openspec/specs/adapters-downloader/spec.md` reflects AQ-005/AQ-006 (bitrate requirement, `NewDownloader(opts ...Option)` signature, updated test table, change-history note); fix any drift; include the file in PR#1's commit.
2. Run `go build ./... && go vet ./... && go test ./...` — full suite green (AQ-020), Spotify package untouched (AQ-019).
3. Confirm no changes to `internal/adapters/spotify/config.go`, `internal/core/ports/`, or the five pre-existing TUI screens (AQ-018/AQ-019).

**Acceptance:**

- [x] PR#1 boundary complete: `internal/config/` (new), `internal/adapters/downloader/ytdlp.go` + `ytdlp_test.go`, `internal/core/service/orchestrator.go` + `orchestrator_test.go`, canonical downloader spec. <!-- sdd-owner: implementation -->
- [x] `go build ./... && go vet ./... && go test ./...` green; Spotify adapter byte-identical to pre-change. <!-- sdd-owner: implementation -->

**Estimated lines:** ~4 (canonical spec). **Dependencies:** T1.1-T1.6. **Risk:** Low.

**Parent actions (after PR#1 implementation lands):**

- [ ] Start or reuse bounded review for PR#1 (backend slice). <!-- sdd-owner: parent -->

---

# PR 2 — UI + entry: TUI Config screen + `main.go` + README (~365 lines)

📍 Current PR. Targets PR #1's branch (`feature-branch-chain`); `internal/tui` imports `internal/config` (PR#1 dependency).

- **Start:** model surface for the sixth screen (constant + fields) — additive, five existing screens untouched.
- **End:** `c` opens Config from Resolving/Playlist/Downloading/Done, `j/k/Enter/Esc` navigate, confirm persists + applies mid-session, wiring in `main.go`, README updated.
- **Prior deps:** PR#1 (T1.2 `internal/config`, T1.6 `orchestrator.SetAudioQuality`).
- **Follow-up:** none for this change (lossless remains a separate future change).
- **Out of scope:** `internal/adapters/spotify/config.go` delegation cleanup, Input-screen `c` intercept (deliberate exception, AQ-010), "best" quality, FLAC.
- **Requirements:** AQ-008, AQ-009, AQ-010, AQ-011, AQ-012, AQ-013, AQ-014, AQ-015, AQ-016, AQ-017, AQ-020.
- **Verification:** `go build ./cmd/music-dl/ && go vet ./... && go test ./...`.

## Task T2.1 — Model surface: `ScreenConfig` constant + config fields (AQ-008)

**Files:** `internal/tui/model.go` (MODIFIED)
**TDD phase:** compile scaffolding — adds the surface the RED tests need (fields/constant), no behavior yet. `NewModel` signature change is deferred to T2.6.

**Steps:**

1. Add `ScreenConfig Screen = iota` after `ScreenDone` (value 5).
2. Add fields to `Model`: `audioQuality string`, `qualityCursor int`, `configWarn string`, `configPath string`, `saveConfig func(path string, cfg config.Config) error`.
3. In `NewModel`, initialize `configPath: config.ConfigPath()`, `saveConfig: config.SaveConfig` (keep existing signature; `audioQuality` stays `""` until T2.6 wires the real value — tests set it via `Model{}` literals).
4. Add import `github.com/Juanstudy/music-downloader/internal/config` (no import cycle: `internal/config` imports stdlib + BurntSushi only).

**Verification:** `go build ./internal/tui/...` compiles; existing tests still pass (`go test ./internal/tui/...`).

**Acceptance:**

- [ ] `ScreenConfig` constant + the five config fields exist on `Model`; `NewModel` sets the `configPath`/`saveConfig` seams. <!-- sdd-owner: implementation -->
- [ ] `go test ./internal/tui/...` green with zero existing-test changes. <!-- sdd-owner: implementation -->

**Estimated lines:** ~12. **Dependencies:** PR#1. **Risk:** Low.

## Task T2.2 — RED: config open/navigation tests (AQ-008…AQ-011)

**Files:** `internal/tui/update_test.go` (MODIFIED — add tests + `newFilterInput()` helper)
**TDD phase:** RED — behavior does not exist yet (handlers come in T2.3).

**Steps:**

1. Helper `newFilterInput()` mirroring `newInput()` (focused `textinput.Model`).
2. AQ-009: `TestConfig_CFromScreens` table — `c` from `ScreenResolving` / `ScreenPlaylist` / `ScreenDownloading` / `ScreenDone` → `ScreenConfig` + correct `PrevScreen`.
3. AQ-008: `TestOpenConfig_CursorAtCurrentQuality` — `Model{Screen: ScreenPlaylist, audioQuality: "192k"}` + `c` → `qualityCursor == 1`.
4. AQ-010: `TestConfig_COnInputTypes` (Input focused, value `"music.youtube.com/watch"` + `c` → stays `ScreenInput`, letter appended) and `TestConfig_CInFilterTypes` (`isFiltering: true` + `c` → filter text gains `c`, filter stays open).
5. AQ-011: `TestConfig_JKMovesBounded` (clamps at 0 and 2), `TestConfig_EscCancelsWithoutWrite` (from Playlist with a `saveConfig` spy → back to Playlist, `audioQuality` unchanged, spy not called), `TestConfig_QStillQuits` (`q` → `tea.Quit`), `TestConfig_HelpStillToggles` (`?` toggles `showHelp`).

**Verification:** `go test ./internal/tui/...` fails on these new tests (RED); existing tests stay green.

**Acceptance:**

- [ ] AQ-008/AQ-009/AQ-010/AQ-011 scenarios covered via `Model{}` literals; `saveConfig` spy used for the no-write assertion. <!-- sdd-owner: implementation -->
- [ ] New tests red before T2.3; all pre-existing tests unaffected. <!-- sdd-owner: implementation -->

**Estimated lines:** ~90. **Dependencies:** T2.1. **Risk:** Medium (key-routing edge cases: Input and filter typing).

## Task T2.3 — GREEN: config open/navigation handlers (AQ-008…AQ-011)

**Files:** `internal/tui/update.go` (MODIFIED)
**TDD phase:** GREEN — implement to make T2.2 pass.

**Steps:**

1. Add `case ScreenConfig:` to `Update()`'s screen routing (KeyMsg → `handleConfigKeys`, else `m, nil`). Global `q`/`?`/`ctrl+c` and message handlers already run before routing, so they keep working on Config.
2. `openConfig()`: set `PrevScreen = m.Screen`, `Screen = ScreenConfig`, clear `configWarn`, `qualityCursor = indexOfQuality(m.audioQuality)` (design §3.8.2).
3. `indexOfQuality(q)` helper (index into `config.ValidQualities()`, 0 fallback).
4. `handleConfigKeys()`: `j`/`down` and `k`/`up` bounded moves, `esc` → back to `PrevScreen` + reset cursor/warn (no write), `enter` → `confirmQuality()` (T2.5 stub may return `m, nil` for now to keep compile green — replaced in T2.5).
5. Add `case "c": return m.openConfig()` to `handleResolvingKeys`, `handlePlaylistKeys` (after the `isFiltering` guard, so filter typing is untouched — AQ-010), `handleDownloadingKeys`, `handleDoneKeys`. `handleInputKeys` gets NO `c` case (types normally — AQ-010).

**Verification:** `go test ./internal/tui/...` passes T2.2 tests.

**Acceptance:**

- [ ] `c` opens Config from the four screens; never intercepted on Input or while filtering. <!-- sdd-owner: implementation -->
- [ ] `j/k` bounded, `Esc` returns without writes; `q`/`?` still handled globally on Config. <!-- sdd-owner: implementation -->

**Estimated lines:** ~40. **Dependencies:** T2.2. **Risk:** Medium.

## Task T2.4 — RED: confirm/persist/failure tests (AQ-012, AQ-013)

**Files:** `internal/tui/update_test.go` (MODIFIED — add tests; local recording downloader helper)
**TDD phase:** RED — `confirmQuality` behavior does not exist yet.

**Steps:**

1. Local helper implementing `ports.Downloader` + `SetAudioBitrate(q)` recording the value, wrapped in a real `service.NewOrchestrator(&stubSearcher{}, rec)`.
2. AQ-012: `TestConfig_EnterConfirmsAndPersists` — `Model{Screen: ScreenConfig, PrevScreen: ScreenPlaylist, audioQuality: "320k", qualityCursor: 1, orchestrator: orch, configPath: filepath.Join(t.TempDir(), "config.toml"), saveConfig: config.SaveConfig}` + `Enter` → `audioQuality == "192k"`, recorder bitrate `"192k"`, file decodes to `[quality] value = "192k"`, back to `ScreenPlaylist`.
3. AQ-013: `TestConfig_EnterSaveFailureNonFatal` — `saveConfig: func(...) error { return errors.New("disk full") }` + `Enter` → `audioQuality` = confirmed value, recorder bitrate set, `configWarn != ""`, stays on `ScreenConfig`, no `tea.Quit` cmd.

**Verification:** `go test ./internal/tui/...` red on the new tests; existing green.

**Acceptance:**

- [ ] AQ-012 asserts in-session value + downloader update + real file write (in `t.TempDir()`) + return to `PrevScreen`. <!-- sdd-owner: implementation -->
- [ ] AQ-013 asserts non-fatal path: value applied, warning set, no quit. <!-- sdd-owner: implementation -->

**Estimated lines:** ~75. **Dependencies:** T2.1, PR#1 (T1.6). **Risk:** Medium.

## Task T2.5 — GREEN: `confirmQuality` (AQ-012, AQ-013)

**Files:** `internal/tui/update.go` (MODIFIED)
**TDD phase:** GREEN — implement to make T2.4 pass.

**Steps:**

1. `confirmQuality()`: `q := config.ValidQualities()[m.qualityCursor]`; set `m.audioQuality = q`; `if m.orchestrator != nil { m.orchestrator.SetAudioQuality(q) }`; call `m.saveConfig(m.configPath, config.Config{Quality: config.Quality{Value: q}})`.
2. On save error: `m.configWarn = fmt.Sprintf("Could not save config (%v). Applied for this session only.", err)`; stay on `ScreenConfig` (warning visible — design §6.7). On success: clear warn, `Screen = PrevScreen`, cursor reset to 0.
3. Wire `handleConfigKeys`'s `enter` case to `confirmQuality()` (replacing the T2.3 stub).

**Verification:** `go test ./internal/tui/...` green.

**Acceptance:**

- [ ] Confirm applies mid-session (model + downloader), persists, returns to `PrevScreen` on success. <!-- sdd-owner: implementation -->
- [ ] Save failure keeps the value, shows `configWarn`, never crashes or quits. <!-- sdd-owner: implementation -->

**Estimated lines:** ~20. **Dependencies:** T2.4. **Risk:** Low.

## Task T2.6 — Wiring: `NewModel` param + `main.go` (AQ-016)

**Files:** `internal/tui/model.go` (MODIFIED), `cmd/music-dl/main.go` (MODIFIED)
**TDD phase:** GREEN (wiring; compilation is the acceptance test).

**Steps:**

1. `model.go`: extend `NewModel` with trailing `audioQuality string`; initialize `audioQuality: audioQuality` (main.go is the only caller; TUI tests use `Model{}` literals — invisible to tests).
2. `main.go`: add `internal/config` import; after `outputDir := defaultOutputDir()` and before searcher wiring, load quality — `cfg, err := config.LoadConfig(config.ConfigPath())`; on error log a warning and keep `config.DefaultQuality` (non-fatal, app continues — AQ-003); else `quality = cfg.Quality.Value`.
3. Downloader construction: `downloader.NewDownloader(downloader.WithAudioBitrate(quality))`.
4. Model construction: `tui.NewModel(orch, searcherImpl, spotifySearcher, querySearcherImpl, outputDir, quality)`.
5. Spotify wiring block unchanged (still uses `spotify.ConfigPath()` — AQ-019).

**Verification:** `go build ./cmd/music-dl/` succeeds; `go test ./internal/tui/...` green.

**Acceptance:**

- [ ] `main()` loads quality (default 320k when absent/malformed), injects it into the downloader and the TUI. <!-- sdd-owner: implementation -->
- [ ] `go build ./cmd/music-dl/` exits 0; Spotify wiring block untouched. <!-- sdd-owner: implementation -->

**Estimated lines:** ~26. **Dependencies:** PR#1, T2.1. **Risk:** Low.

## Task T2.7 — RED: view tests (AQ-014, AQ-015)

**Files:** `internal/tui/view_test.go` (NEW)
**TDD phase:** RED — `renderConfigView` does not exist yet.

**Steps:**

1. Builder `configModel()` — `Model{Screen: ScreenConfig, Ready: true, Width: 80, Height: 24, audioQuality: ..., qualityCursor: ...}`.
2. AQ-014: `TestConfigView_RendersOptionsAndCursor` (cursor on 320k → output contains `128k`, `192k`, `320k` and the 320k line marked with cursor/selection indicator); `TestConfigView_RendersCurrentQualityAndFooter` (`audioQuality: "192k"` → output indicates current quality is `192k`; footer mentions `j/k`, `Enter`, `Esc`).
3. AQ-015: `TestHelpShowsCKey` — `helpView(width)` output contains `c` and `Configure quality`.

**Verification:** `go test ./internal/tui/...` red on the new tests (missing symbols/behavior).

**Acceptance:**

- [ ] AQ-014 rendering cases assert options + cursor marker + current quality + footer hints. <!-- sdd-owner: implementation -->
- [ ] AQ-015 help test asserts the `c` entry in the global overlay. <!-- sdd-owner: implementation -->

**Estimated lines:** ~55. **Dependencies:** T2.1. **Risk:** Low.

## Task T2.8 — GREEN: config view + help entry (AQ-014, AQ-015)

**Files:** `internal/tui/view.go` (MODIFIED), `internal/tui/keys.go` (MODIFIED)
**TDD phase:** GREEN — implement to make T2.7 pass.

**Steps:**

1. `view.go`: add `case ScreenConfig: content = m.renderConfigView()` to `View()` dispatch.
2. `renderConfigView()` per design §3.9: header `♪ music-dl — Configure Quality`, optional `configWarn` line (`warningStyle`, "⚠ " prefix), muted hint ("yt-dlp never up-samples..."), three options with `▸` cursor + `●` marker (selected line via `selectedStyle`), `Current: <q>` via `emphStyle`, then `renderFooter()`.
3. `renderFooter()`: when `m.Screen == ScreenConfig`, append `j/k move`, `Enter confirm`, `Esc back` rows (design §3.9).
4. `keys.go`: append `{"c", "Configure quality"}` to `helpContent`.

**Verification:** `go test ./internal/tui/...` green.

**Acceptance:**

- [ ] Config view renders options, cursor marker, current quality, footer; `View()` dispatches to it. <!-- sdd-owner: implementation -->
- [ ] Global help overlay lists `c` on every screen. <!-- sdd-owner: implementation -->

**Estimated lines:** ~39. **Dependencies:** T2.7. **Risk:** Low.

## Task T2.9 — README: controls + roadmap (AQ-017)

**Files:** `README.md` (MODIFIED)
**TDD phase:** docs (no test; verify by reading rendered section).

**Steps:**

1. Controles/keybindings table: add `c` row (Spanish, matching existing table style), e.g. `| c | Configurar calidad de audio (128k / 192k / 320k) |`.
2. Roadmap: replace `Calidad configurable (128k, 320k, lossless)` with a delivered item `Calidad configurable (128k / 192k / 320k) ✔`; keep lossless listed as open.

**Acceptance:**

- [ ] README lists the `c` → Config screen binding; roadmap shows lossy delivered and lossless still open. <!-- sdd-owner: implementation -->

**Estimated lines:** ~8. **Dependencies:** none. **Risk:** Low.

## Task T2.10 — GATE: PR#2 verification + no-regression (AQ-016…AQ-020)

**Files:** verification only
**TDD phase:** REFACTOR / gate.

**Steps:**

1. `go build ./cmd/music-dl/` (AQ-016) and `go vet ./...`.
2. `go test ./...` — full suite green (AQ-020).
3. AQ-019: `go test ./internal/adapters/spotify/...` green; confirm zero diff in `internal/adapters/spotify/`.
4. AQ-018: confirm `ports.Downloader` and `Orchestrator.DownloadTrack` unchanged; existing adapter/orchestrator/compliance tests pass unmodified.
5. Manual sanity (optional): default run downloads with `--audio-bitrate 320k` (URL mode unchanged except the default flag).

**Acceptance:**

- [ ] `go build ./cmd/music-dl/ && go vet ./... && go test ./...` green. <!-- sdd-owner: implementation -->
- [ ] Spotify package untouched (AQ-019); port + `DownloadTrack` signatures unchanged (AQ-018); URL-mode flow regression-free (AQ-020). <!-- sdd-owner: implementation -->

**Estimated lines:** —. **Dependencies:** T2.1-T2.9. **Risk:** Low.

**Parent actions (after PR#2 implementation lands):**

- [ ] Start or reuse bounded review for PR#2 (UI + entry slice). <!-- sdd-owner: parent -->

---

## Risk Assessment

| Task | Risk | Mitigation |
| ------ | ------ | ------------ |
| T1.1/T1.2 (config package) | Low | Table-driven `t.TempDir()` tests; load-merge-save pinned by round-trip test |
| T1.3/T1.4 (downloader) | Low | Pure `buildArgs` unit-tested without yt-dlp; variadic options keep old call sites compiling |
| T1.5/T1.6 (orchestrator) | Low | Optional-capability assertion; port frozen (AQ-018) |
| T2.2/T2.3 (`c` routing) | Medium | Per-screen intercept only; `isFiltering` guard precedes the Playlist switch; Input untouched (AQ-010 pinned by tests) |
| T2.4/T2.5 (confirm/save) | Medium | `saveConfig` seam makes AQ-012 (real write) and AQ-013 (forced failure) deterministic without touching the user's real config |
| T2.6 (wiring) | Low | `main.go` is the only `NewModel` caller; config load non-fatal |
| T2.7/T2.8 (view) | Low | Pure rendering driven by model state |
| Gates (T1.7/T2.10) | Low | `go build`/`go vet`/`go test ./...` full pass; Spotify package diff check |

**Overall risk:** Low-Medium — additive and cleanly split; the only real hazard is `c` key routing, pinned by T2.2's Input/filter tests.

---

## Rollback Notes

Each PR is independently revertible:

- **PR#1:** delete `internal/config/`; revert `ytdlp.go`/`ytdlp_test.go` to the no-options form; revert `orchestrator.go`/`orchestrator_test.go`; revert `openspec/specs/adapters-downloader/spec.md`. Zero impact on the TUI or Spotify flow — the options pattern is backward compatible, so even keeping it compiles.
- **PR#2:** revert `model.go`, `update.go`, `view.go`, `keys.go`, `update_test.go`, `view_test.go`, `main.go`, `README.md`. The five pre-existing screens and URL-mode flow are untouched during the change.
- Config files already written with a `[quality]` section are **harmless after rollback**: BurntSushi ignores unknown sections and `spotify.Config` never reads `[quality]`. No data migration.
