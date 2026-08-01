# Apply Progress — `audio-quality` (PR#1 + PR#2 — COMPLETE)

**Change:** `audio-quality`
**Phase:** sdd-apply — BOTH SLICES DONE (PR#1 backend, PR#2 TUI)
**Date:** 2026-08-01
**Test runner:** `go test ./...` — **STRICT TDD active**
**Delivery:** feature-branch-chain — tracker `feat/audio-quality` (PR #17 draft → main), child `feat/audio-quality-pr2` (PR #18 → tracker).

---

## Structured Status

```yaml
schemaName: spec-driven
changeName: audio-quality
artifactStore: hybrid          # openspec + engram (obs 188 = apply-progress merged)
changeRoot: openspec/changes/audio-quality
artifactStatus:
  proposal: done
  specs: done
  design: done
  tasks: done
  applyProgress: done (this file)
taskProgress:
  total: 33            # T1.1..T1.7 (14) + T2.1..T2.10 (19)
  complete: 33
  remaining: 0
parentOwned:
  review PR#1: done   # lineage review-85de9463aacffcaa — APPROVED, committed c3872e9
  review PR#2: done   # lineage review-63ae02872f1980c3 — APPROVED, committed fdd31cf
applyState: all_done
dependencies:
  apply: all_done
  verify: ready
  sync: not_applicable
  archive: ready
nextRecommended: verify → archive
```

---

## PR#1 — Backend (committed `c3872e9`)

- `internal/config` NEW: `ConfigPath` (XDG-aware), `ValidQualities` [128k,192k,320k], `DefaultQuality=320k`, `LoadConfig` (missing/invalid → 320k nil; malformed → error+zero), `SaveConfig` (load-merge-save, never clobbers malformed). 12 tests.
- Downloader `ytdlp.go`: `WithAudioBitrate` option + `SetAudioBitrate`, pure `buildArgs` (`--audio-bitrate` after `--audio-format mp3`), mutex-guarded field. 6 tests + port guard.
- Orchestrator `SetAudioQuality` guarded passthrough (qualitySetter interface). Port + `DownloadTrack` byte-for-byte unchanged.
- Canonical `openspec/specs/adapters-downloader/spec.md` updated in this slice (no re-apply in archive).
- Review `review-85de9463aacffcaa`: 4R, 1 CRITICAL (R3-001 data race) fixed via mutex with strict TDD; **approved**; pre-commit/pre-push `allow`.

## PR#2 — TUI Config screen (committed `fdd31cf`)

- `internal/tui/model.go`: `ScreenConfig` + `qualityCursor`/`configWarn`/`configPath` + `saveConfig`/`configPath` seams.
- `internal/tui/update.go`: `openConfig`/`handleConfigKeys`/`confirmQuality`/`indexOfQuality`; `c` on Resolving/Playlist/Downloading/Done, never Input/filter (AQ-010). Esc cancels without write; Enter confirms + persists; save failure non-fatal (AQ-013).
- `internal/tui/update_test.go` +9 tests (routing, bounds, cancel, persist round-trip with t.TempDir, save-failure non-fatal, q/? on Config).
- `internal/tui/view.go`: `renderConfigView` + footer; `internal/tui/view_test.go` NEW (options/cursor/current/footer/help).
- `internal/tui/keys.go`: help entry `c → Configure quality`.
- `cmd/music-dl/main.go`: non-fatal config load (DefaultQuality fallback), `WithAudioBitrate` + `NewModel(..., quality)` wiring.
- `README.md`: controls table `c` row; roadmap lossy delivered ✔ / lossless (FLAC) open.
- Review `review-63ae02872f1980c3`: 4R, **approved** (3 WARNING + 12 SUGGESTION → follow-ups, no blockers); refuter 6/6 corroborated; evidence `passed`; pre-commit/pre-push/pre-pr `allow`.

## Gates (both PRs)

- `go build ./cmd/music-dl/` ✅ · `go vet ./...` ✅ · `gofmt -l` ✅
- `go test ./... -count=1` ✅ (incl. network integration) · `go test -race` tui/config/core ✅
- Spotify adapters zero-diff · `ports.Downloader`/`DownloadTrack` byte-identical

## Follow-ups (post-merge, non-blocking)

- R1-001/R4-003: atomic SaveConfig (temp+rename) + 0o600 — shared file with Spotify secrets.
- R4-001/R1-004: async messages (resolve/download done) can hijack ScreenConfig; `esc` on Done quits.
- R1-002: visible warning for malformed config; R1-003: help hint for `c` on Input.
- R2-001: artifact line-count forecast stale (~365 claimed vs 541 real).
- R3-003: use t.TempDir() in two tests (hardcoded /tmp/config.toml).
