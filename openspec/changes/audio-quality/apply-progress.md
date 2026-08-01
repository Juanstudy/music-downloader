# Apply Progress — `audio-quality` (PR#1 backend slice)

**Change:** `audio-quality`
**Phase:** sdd-apply — SLICE PR#1 (backend)
**Date:** 2026-08-01
**Test runner:** `go test ./...` — **STRICT TDD active** (per parent prompt; `openspec/config.yaml` has a stale `strict_tdd: false` from before go.mod existed — deviation noted below)
**Delivery:** feature-branch-chain, PR#1 targets tracker `feat/audio-quality`. No branches/commits/pushes created (orchestrator owns the chain after the review gate).

---

## Structured Status Consumed

```yaml
schemaName: spec-driven
changeName: audio-quality
artifactStore: openspec          # artifacts under openspec/changes/audio-quality/; Engram down (see Risks)
planningHome:
  root: /home/juan-arch/Projects/music-dowloader
  changesDir: openspec/changes
changeRoot: openspec/changes/audio-quality
artifactPaths:
  proposal: [openspec/changes/audio-quality/proposal.md]
  specs:    [openspec/changes/audio-quality/spec/spec.md, openspec/specs/adapters-downloader/spec.md]
  design:   [openspec/changes/audio-quality/design/design.md]
  tasks:    [openspec/changes/audio-quality/tasks/tasks.md]
  applyProgress: [openspec/changes/audio-quality/apply-progress.md]
artifactStatus:
  proposal: done
  specs: done
  design: done
  tasks: done
  applyProgress: done (this file)
taskProgress:            # implementation-owned rows for THIS slice
  total: 14              # T1.1..T1.7 (2 checkboxes each) — PR#1
  complete: 14
  remaining: 0
deferredParentActions:   # T2.x + gate reviews are NOT this slice
  total: 2
  complete: 0
  remaining: 2           # parent-owned review gates (PR#1 and PR#2)
applyState: ready → all_done (for PR#1 slice)
dependencies:
  apply: all_done
  verify: ready (after parent bounded review)
  sync: not_applicable
  archive: not_applicable
actionContext:
  mode: repo-local
  workspaceRoot: /home/juan-arch/Projects/music-dowloader
  allowedEditRoots: [workspace root]
  warnings: []
nextRecommended: parent-lifecycle
```

---

## Completed Tasks (PR#1 — T1.1..T1.7)

All 14 implementation checkboxes for T1.1–T1.7 marked `[x]` in `openspec/changes/audio-quality/tasks/tasks.md` (persisted as each task completed).

| Task | Summary |
| ---- | ------- |
| T1.1 RED | `internal/config/config_test.go` (NEW) — AQ-001..AQ-004 table-driven tests in `t.TempDir()`/`t.Setenv`; confirmed RED (compile failure: `undefined: ValidQualities`). |
| T1.2 GREEN | `internal/config/config.go` (NEW) — constants, `Quality`/`Config` structs, `ConfigPath()` (canonical copy of spotify logic, AQ-019), `ValidQualities`, `IsValidQuality`, `LoadConfig` (missing file/section → 320k nil; invalid → `log.Printf` warning + 320k nil; malformed → error + zero Config), `SaveConfig` (load-merge-save, MkdirAll, buffer-then-write, never clobbers malformed file). |
| T1.3 RED | `ytdlp_test.go` — `TestBuildArgs_NoBitrate`, `TestBuildArgs_BitratePosition`, `TestBuildArgs_ExistingFlagsIntact`, `TestWithAudioBitrate_EachLevel`, `TestNewDownloader_NoOption`, `TestSetAudioBitrate_MidSession` + `var _ ports.Downloader = (*Downloader)(nil)`; confirmed RED (no `buildArgs`/options). |
| T1.4 GREEN | `ytdlp.go` — `audioBitrate` field, `Option`/`WithAudioBitrate`, variadic `NewDownloader(opts ...Option)` (existing call sites compile unchanged), `SetAudioBitrate`, pure `buildArgs(media, outputDir, bitrate)` (bitrate inserted immediately after `--audio-format mp3`), port guard. `Download` is now `args := buildArgs(media, outputDir, d.audioBitrate)`; rest unchanged. |
| T1.5 RED | `orchestrator_test.go` — `mockDownloader` gains `audioBitrate` + `SetAudioBitrate`; `TestSetAudioQualityForwardsToDownloader`; confirmed RED (`SetAudioQuality` undefined). |
| T1.6 GREEN | `orchestrator.go` — local `qualitySetter` interface + `SetAudioQuality` guarded type assertion; `ports.Downloader` and `DownloadTrack` byte-for-byte unchanged. |
| T1.7 GATE | `go build ./...` OK, `go vet ./...` OK, gofmt clean, `go test ./...` green (incl. real-network integration tests). Canonical `openspec/specs/adapters-downloader/spec.md` verified coherent with AQ-005/AQ-006 (change history, `NewDownloader(opts ...Option)` signature, bitrate test table) — already updated by spec phase, no drift found, ships in PR#1. Spotify/ports/TUI/main.go/README zero diff. |

## Files Changed (PR#1 boundary)

| File | Status |
| ---- | ------ |
| `internal/config/config.go` | NEW |
| `internal/config/config_test.go` | NEW |
| `internal/adapters/downloader/ytdlp.go` | MODIFIED |
| `internal/adapters/downloader/ytdlp_test.go` | MODIFIED |
| `internal/core/service/orchestrator.go` | MODIFIED |
| `internal/core/service/orchestrator_test.go` | MODIFIED |
| `openspec/specs/adapters-downloader/spec.md` | MODIFIED (pre-existing from spec phase; verified, ships in PR#1) |
| `openspec/changes/audio-quality/` | NEW (change artifacts incl. this apply-progress) |

**Untouched (zero diff, verified via git):** `internal/adapters/spotify/` (AQ-019), `internal/core/ports/` (AQ-018), `internal/tui/` (PR#2), `cmd/music-dl/main.go` (PR#2), `README.md` (PR#2).

## Tests Written (per AQ)

- **config_test.go (12 test funcs):** AQ-001 `TestValidQualities`, `TestIsValidQuality_RejectsNonLevels`, `TestDefaultQuality` · AQ-002 `TestConfigPath_XDGSet`, `TestConfigPath_XDGUnset` · AQ-003 `TestLoadConfig_MissingFile`, `TestLoadConfig_MissingQualitySection`, `TestLoadConfig_InvalidValueFallsBack` (999k/flac/best), `TestLoadConfig_MalformedTOML` · AQ-004 `TestSaveConfig_RoundTripPreservesSpotify` (byte-identical `[spotify]`), `TestSaveConfig_FirstSaveCreatesDirAndFile`, `TestSaveConfig_MalformedExistingFile`.
- **ytdlp_test.go (6 new funcs):** AQ-005 `TestBuildArgs_NoBitrate`, `TestBuildArgs_BitratePosition`, `TestBuildArgs_ExistingFlagsIntact` · AQ-006 `TestWithAudioBitrate_EachLevel`, `TestNewDownloader_NoOption`, `TestSetAudioBitrate_MidSession` · AQ-018 `var _ ports.Downloader = (*Downloader)(nil)`.
- **orchestrator_test.go (1 new func):** AQ-007 `TestSetAudioQualityForwardsToDownloader` (mock records `192k`).

## Commands Run

```bash
go test ./... -short            # baseline safety net: all green pre-change
go test ./internal/config/...   # RED (compile) → GREEN
go test ./internal/adapters/downloader/...  # RED (compile) → GREEN (-short)
go test ./internal/core/service/...         # RED (compile) → GREEN
go build ./... && go vet ./...              # OK
gofmt -l internal/                          # clean (ytdlp_test.go reformatted once)
go test ./...                               # FULL SUITE GREEN (incl. network integration)
git diff --stat -- spotify ports tui cmd README  # empty — boundaries respected
```

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
| ------ | ----------- | ------- | ------------ | ----- | ------- | ------------- | ---------- |
| 1.1 | `internal/config/config_test.go` | Unit | N/A (new) | ✅ Written (compile fail) | ✅ Passed | ✅ 3+ cases/behavior (table-driven) | ✅ Doc comments, helper extraction |
| 1.2 | `internal/config/config_test.go` | Unit | N/A (new pkg) | ✅ (from 1.1) | ✅ Passed | ✅ 999k/flac/best + missing-section | ✅ switch over if-chain |
| 1.3 | `internal/adapters/downloader/ytdlp_test.go` | Unit | ✅ 3/3 pre-existing green | ✅ Written (compile fail) | ✅ Passed | ✅ 6 funcs, each table-driven | ➖ None needed |
| 1.4 | `internal/adapters/downloader/ytdlp_test.go` | Unit | ✅ 3/3 | ✅ (from 1.3) | ✅ Passed | ✅ each level + no-option + mid-session | ✅ buildArgs extracted pure |
| 1.5 | `internal/core/service/orchestrator_test.go` | Unit | ✅ 11/11 pre-existing green | ✅ Written (compile fail) | ✅ Passed | ✅ single scenario (AQ-007) | ➖ None needed |
| 1.6 | `internal/core/service/orchestrator_test.go` | Unit | ✅ 11/11 | ✅ (from 1.5) | ✅ Passed | ✅ passthrough + no-op branch covered | ✅ local interface kept minimal |

### Test Summary

- **Total tests written:** 19 test functions (12 config + 6 downloader + 1 orchestrator; 3 config tests double as triangulation of AQ-001).
- **Total tests passing:** full suite green.
- **Layers used:** Unit (19), Integration (pre-existing, green with real network).
- **Approval tests:** None needed — no behavior changed; no-option downloader path pinned by `TestNewDownloader_NoOption`.
- **Pure functions created:** `buildArgs` (downloader), `ValidQualities`, `IsValidQuality`, `ConfigPath`, `LoadConfig`, `SaveConfig` (config).

## Deviations / Decisions on the Fly

1. **Strict TDD source:** parent prompt declares STRICT TDD active; `openspec/config.yaml` still says `strict_tdd: false` with the stale reason "No go.mod initialized yet" (go.mod exists since the skeleton-reboot). Followed the parent prompt (system-level strict-tdd-mode is enabled). Recommend the orchestrator refresh `sdd-init` capabilities so config.yaml matches reality.
2. **`LoadConfig` missing-section vs invalid-value warning:** missing `[quality]` section falls back to 320k **silently**; only a non-empty invalid value (e.g. `"999k"`, `"flac"`) logs the warning — matches AQ-003 wording ("invalid values MUST produce a warning", missing section does not).
3. **Round-trip seed:** `TestSaveConfig_RoundTripPreservesSpotify` seeds the file with a BurntSushi `toml.Encoder`-encoded spotify-only struct so the `[spotify]` block comparison is byte-identical (design Open Decisions #5 respected).
4. **gofmt:** `ytdlp_test.go` required one `gofmt -w` pass after editing (indentation inside the nested loop).
5. **Canonical spec:** no drift found in `openspec/specs/adapters-downloader/spec.md`; it already carries the AQ-005/AQ-006 bitrate requirement, `NewDownloader(opts ...Option)` signature, updated test table, and change-history note. Not rewritten.
6. **Engram persistence:** the `mem_*` server was briefly unreachable mid-session (`http://127.0.0.1:7437`), but the final `mem_save` for `topic_key: sdd/audio-quality/apply-progress` **succeeded** (observation id 188, project `music-downloader`). This apply-progress is also persisted as a file under `openspec/changes/audio-quality/` (dual source of truth).

## Remaining Tasks (NOT this slice — PR#2)

Exact unchecked lines in `openspec/changes/audio-quality/tasks/tasks.md` for PR#2 (T2.1..T2.10, 20 implementation checkboxes) and the parent-owned review gates:

- T2.1 Model surface (`ScreenConfig` + config fields)
- T2.2 RED config open/navigation tests
- T2.3 GREEN open/nav handlers
- T2.4 RED confirm/persist/failure tests
- T2.5 GREEN `confirmQuality`
- T2.6 Wiring `NewModel` param + `main.go`
- T2.7 RED view tests (`view_test.go` NEW)
- T2.8 GREEN `renderConfigView` + keys
- T2.9 README controls + roadmap
- T2.10 GATE PR#2
- Parent: `- [ ] Start or reuse bounded review for PR#1 (backend slice). <!-- sdd-owner: parent -->`
- Parent: `- [ ] Start or reuse bounded review for PR#2 (UI + entry slice). <!-- sdd-owner: parent -->`

## Workload / PR Boundary

PR#1 changed ~342 lines (`git diff --stat` + new files): 5 tracked files +292/-16 plus `internal/config/` (2 new files, ~300 lines incl. tests) — under the 400-line per-PR budget. PR#2 remains its own work unit. Feature-branch-chain: PR#1 targets the tracker; PR#2 will target PR#1's branch. Each PR independently revertible (Rollback Notes in tasks.md).

## Git Tree State (no commits made — per delivery context)

```
 M .atl/skill-registry.md                          (pre-existing, not mine)
 M internal/adapters/downloader/ytdlp.go           (PR#1)
 M internal/adapters/downloader/ytdlp_test.go      (PR#1)
 M internal/core/service/orchestrator.go           (PR#1)
 M internal/core/service/orchestrator_test.go      (PR#1)
 M openspec/specs/adapters-downloader/spec.md      (PR#1 — canonical spec)
?? internal/config/                                (PR#1 — new)
?? openspec/changes/audio-quality/                 (SDD change artifacts)
```

Zero diff: `internal/adapters/spotify/`, `internal/core/ports/`, `internal/tui/`, `cmd/music-dl/main.go`, `README.md`.
