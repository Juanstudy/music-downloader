# Archive Report — `audio-quality`

**Change:** `audio-quality` — Configurable MP3 audio quality (128k / 192k / 320k)
**Date:** 2026-08-01
**Store:** hybrid (openspec + engram obs 188)
**Status:** CLOSED — all slices shipped and reviewed

---

## Summary

Delivered configurable MP3 bitrate selection (128k / 192k / 320k, default 320k)
persisted in `~/.config/music-dl/config.toml` under `[quality]`, selected via a
new TUI Config screen (`c` key). Backend (config package, downloader
`WithAudioBitrate`/`SetAudioBitrate`, orchestrator passthrough) and frontend
(TUI Config screen, entry wiring, README) shipped as a 2-PR feature-branch-chain.

## Artifacts

- `openspec/changes/audio-quality/proposal.md`
- `openspec/changes/audio-quality/spec/spec.md` (AQ-001..AQ-020)
- `openspec/changes/audio-quality/design/design.md`
- `openspec/changes/audio-quality/tasks/tasks.md` (33/33 `[x]`)
- `openspec/changes/audio-quality/apply-progress.md` (T1.x + T2.x complete)
- `openspec/specs/adapters-downloader/spec.md` (canonical, updated in spec phase — NOT re-applied in archive)
- Engram: `sdd/audio-quality/apply-progress` (obs 188, merged)

## Delivery

| Slice | Branch | Commit | PR | Review |
| --- | --- | --- | --- | --- |
| PR#1 backend | `feat/audio-quality` (tracker) | `c3872e9` | #17 (draft → main) | `review-85de9463aacffcaa` approved |
| PR#2 TUI | `feat/audio-quality-pr2` → tracker | `fdd31cf` | #18 (open) | `review-63ae02872f1980c3` approved |

Chain: `main → 📍 feat/audio-quality (tracker) → 📍 feat/audio-quality-pr2`.

## Verification (sdd-verify: PASS)

- 20/20 acceptance criteria (AQ-001..AQ-020) verified against code + tests.
- `go build ./...` / `go vet ./...` / `gofmt` clean.
- `go test ./... -count=1` fresh: all 11 packages pass (incl. network integration).
- `go test -race` on config/downloader/service/tui: clean (mutex race fix holds).
- AQ-018/019: `internal/core/ports` and `internal/adapters/spotify` git-diff EMPTY; `DownloadTrack` byte-for-byte untouched.
- 33/33 task checkboxes consistent with implementation.

## Follow-ups (post-merge, non-blocking)

1. Atomic `SaveConfig` (temp+rename) + `0o600` perms — shared file with Spotify secrets (R1-001/R4-003).
2. Async messages (resolve/download done) can hijack the Config screen; `esc` on Done quits (R4-001/R1-004).
3. Visible on-screen warning for malformed config (R1-002).
4. Help hint for `c` on Input screen (R1-003).
5. `t.TempDir()` in two tests hardcoding `/tmp/config.toml` (R3-003).
6. README architecture block: "5 pantallas" → 6 (R2-002).
7. Artifact line-count forecast stale (~365 claimed vs 541 real) (R2-001).

## Non-goals (kept)

- FLAC/lossless support — remains open in README roadmap.
- "best" quality option — explicitly rejected (fixed set only).
- Spotify adapter changes — zero-diff (AQ-019).
