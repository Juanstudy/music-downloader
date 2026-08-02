# Apply Progress — Adapter Routing Fix

**Change:** `adapter-routing-fix`
**Batch:** 1 (first — no prior apply-progress existed; written fresh)
**Date:** 2026-08-01
**Executor:** sdd-apply (strict TDD mode active)
**Test runner:** `go test -short ./...`

---

## Structured Status Consumed

- Artifact store: **OpenSpec** (file-based).
- Delivery: single PR (`ask-on-risk`, no risk trigger → no chain). Review tier: standard — exactly one `review-reliability` lens in the post-apply bounded review (parent-owned).
- Decision needed before apply: **No**; Chained PRs recommended: **No**; 400-line budget risk: **Low** → no delivery blocker, implementation proceeded.
- Commit strategy (tasks rollback notes): no per-task commit instruction in the tasks artifact → per delegation rules, **all changes left uncommitted and unstaged** for the orchestrator's review step.

## Tasks Completed (T1→T6, strict TDD order)

All 12 implementation-owned checkboxes in `openspec/changes/adapter-routing-fix/tasks/tasks.md` marked `- [x]` (verified by re-reading the persisted artifact). The 2 parent-owned rows remain unchecked (deferred lifecycle actions — untouched).

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
| ------ | ----------- | ------- | ------------ | ----- | ------- | ------------- | ---------- |
| T1 | `internal/adapters/spotify/url_test.go` | Unit | ✅ spotify pkg ok (baseline) | ✅ Written + run: `undefined: IsSpotifyURL` (compile-red) | — | ✅ 17 rows (16 required + 1 optional) | — |
| T2 | `internal/adapters/spotify/url.go` | Unit | — | — | ✅ `go test ./internal/adapters/spotify/` green | ✅ all 17 rows pass | ✅ doc comment per design §2.1; no new imports |
| T3 | `internal/tui/update_test.go` | Unit | ✅ tui pkg ok (baseline) | ✅ Written + run: `too many arguments in call to m.selectedSearcher` (compile-red); then signature widened with old body → **behavioral-red**: `auto youtube url configured` FAILS (issue #19), `TestURLMode_SpotifyURIAccepted` FAILS | — | ✅ 9-row routing table + 2 gate tests | — |
| T4 | `internal/tui/update.go` | Unit | — | — | ✅ `go test ./internal/tui/` green | ✅ routing cases 1–7b + gate tests all pass | ✅ doc comment per design §2.2; gofmt clean |
| T5 | repo gates (no source edits) | — | — | — | — | — | ✅ vet / build / `-short` all exit 0 |
| T6 | `.github/workflows/ci.yml` (config artifact) | — | — | — | — | — | ✅ YAML parses (ruby psych; python3 yaml module unavailable); LSP reported YAML clean |

### Test Summary

- **Total tests written**: 21 (17 `TestIsSpotifyURL` rows + 9 `TestSelectedSearcherRouting` rows + 2 gate tests = 28 assertions-bearing subtests; 3 new test functions).
- **Total tests passing**: all — full `go test -short ./...` = 11 packages ok, 0 failures.
- **Layers used**: Unit (28).
- **Approval tests**: existing `TestParseSpotifyURL`, the 4 TUI no-regression gate tests (`TestEnterEmptyURLShowsError`, `TestInputWhitespaceURL`, `TestURLMode_NonURLSuggestion`, `TestURLMode_ValidURLStillResolves`), and the search-mode/Tab-cycle suites — all green unchanged (part of the full-suite run).
- **Pure functions created**: 1 (`IsSpotifyURL`); routing logic confined to existing method with one added parameter.

## Files Changed

| File | Status | Change |
| ------ | -------- | -------- |
| `internal/adapters/spotify/url.go` | MODIFIED | +`IsSpotifyURL` (design §2.1) above `parseSpotifyURL`; no new imports (`errors`, `net/url`, `regexp`, `strings` unchanged) |
| `internal/adapters/spotify/url_test.go` | MODIFIED | +`TestIsSpotifyURL` (17-row table); existing `TestParseSpotifyURL` untouched |
| `internal/tui/update.go` | MODIFIED | spotify import (first in internal block, gofmt-stable); gate: `&& !strings.HasPrefix(val, "spotify:")`; call site `m.selectedSearcher(url)`; `selectedSearcher(url string)` with URL-driven `SourceAuto` branch (design §2.2/2.3/2.4) |
| `internal/tui/update_test.go` | MODIFIED | +`TestSelectedSearcherRouting` (9 rows, sentinel identity), `TestURLMode_SpotifyURIAccepted`, `TestURLMode_OtherSchemeStillBlocked`; existing tests untouched |
| `.github/workflows/ci.yml` | NEW | map-form `on:` (push/pull_request empty), single `ci` job, checkout@v4 → setup-go@v5 (`go-version-file: go.mod`) → vet → build → `test -short` (design §3.5) |

## Verification Evidence (final)

```
$ go vet ./...
vet: exit 0
$ go build ./...
build: exit 0
$ go test -short -count=1 ./...
?   github.com/Juanstudy/music-downloader/cmd/music-dl  [no test files]
ok  github.com/Juanstudy/music-downloader/internal/adapters/downloader
ok  github.com/Juanstudy/music-downloader/internal/adapters/filesystem
ok  github.com/Juanstudy/music-downloader/internal/adapters/preflight
ok  github.com/Juanstudy/music-downloader/internal/adapters/querysearcher
ok  github.com/Juanstudy/music-downloader/internal/adapters/searcher
ok  github.com/Juanstudy/music-downloader/internal/adapters/spotify
ok  github.com/Juanstudy/music-downloader/internal/config
ok  github.com/Juanstudy/music-downloader/internal/core/domain
ok  github.com/Juanstudy/music-downloader/internal/core/ports
ok  github.com/Juanstudy/music-downloader/internal/core/service
ok  github.com/Juanstudy/music-downloader/internal/tui
```

- **ARF-009 hermetic**: `TestSearcher_Integration` (and the yt-dlp integration test) SKIP under `-short` — verified `--- SKIP: TestSearcher_Integration (0.00s)`, `skipping integration test (requires yt-dlp)`. Plain `go test ./...` NOT run (network 403 expected — not a regression).
- **gofmt**: all 4 modified Go files clean.

## No-Regression Evidence (ARF-006 / ARF-007)

- `git status --porcelain` shows exactly: the 4 intended modified Go files, the new `.github/workflows/ci.yml`, the pre-existing `openspec/config.yaml` modification, and the untracked `openspec/changes/adapter-routing-fix/`. No `ports.Searcher`, orchestrator, downloader, or `audio-quality` file diffs.
- **config.yaml**: NOT touched by this batch. **Pre-existing working-tree diff** (present before the apply batch, from the planning/spec phase — the proposal documents it as "already corrected; treated as fixed input"): runner `go test ./...` → `go test -short ./...`, `available: false` → `true`, `strict_tdd: false` → `true`, integration layer now declared. SHA-256 at apply end: `684dc7f803b4d917a940fa9ec35c60d59132ed1f30121f8f670c6339a6e001e9`. Byte-identity vs. the pre-batch state is preserved (no edits made to it); the verify phase should compare against its chosen baseline (commit + this pre-existing diff).
- **spotify imports** (`go list -f '{{join .Imports "\n"}}'`): stdlib + `github.com/BurntSushi/toml` + `internal/core/domain` + `internal/core/ports` — **no `internal/tui`**, no import cycle (ARF-006).
- **ARF-007**: `parseSpotifyURL`/`validateTrack` unchanged (diff shows only the new `IsSpotifyURL` doc comment mentions); `TestParseSpotifyURL` playlist/album/artist rows still pass with "only track URLs are supported in this version".

## Design §7 Micro-decisions Honored

1. Case-sensitive `spotify:` prefix — `strings.HasPrefix` exact, no case-folding anywhere. ✅
2. No explicit-port handling — no `net.SplitHostPort` added (minimal). ✅
3. Scheme-less host → `url.Parse` empty Host → `false`. ✅ (no test row: gate requires `://` or `spotify:`)
4. Whitespace tolerance — `TrimSpace` inside `IsSpotifyURL`; pinned by optional row `"  spotify:track:x  "` → true. ✅
5. YAML map-form `on:` with empty `push:`/`pull_request:` entries. ✅
6. CI written only after Go work green (T5 passed before T6). ✅
7. Bare `spotify:track` (no ID) admitted by gate + router, rejected downstream by `parseSpotifyURL` ("invalid Spotify URI format") — no special handling. ✅
8. Import ordering — spotify import first in the `internal/...` block (alphabetical: `adapters` < `config` < `core`), gofmt-confirmed stable. ✅

## Deviations / Notes

1. **Optional 17th `TestIsSpotifyURL` row** `"  spotify:track:4iV5W9uYEdYUVa79Axb7Rh  "` → `true`, explicitly permitted by tasks T1 step 4, pins design §7.4 (TrimSpace inside the helper). The required 16 rows (15 spec + exact-host `https://spotify.com/track/x`) are all present — reconciliation, not drift.
2. **config.yaml pre-existing diff** — recorded above; this batch added zero bytes to it.
3. **YAML parse check** used ruby (stdlib psych) because python3's `yaml` module is not installed; file also reported "YAML clean" by the editor LSP. `on:` shows as `true` under psych's YAML 1.1 — expected; GitHub Actions parses YAML 1.2 where `on` is the string trigger.
4. Red-state sequence for T3 followed the design §3.3 note: compile-red first (arity), then signature widened with the old body → behavioral-red for routing case 1 (issue #19) and `TestURLMode_SpotifyURIAccepted`, before the T4 green implementation.

## Remaining Tasks (exact unchecked lines — parent-owned, deferred)

- `- [ ] Start or reuse bounded review for the single PR: standard tier, exactly one \`review-reliability\` lens (dominant risk: behavior, state, tests, determinism, regressions). <!-- sdd-owner: parent -->`
- `- [ ] Verify-phase launch: CI file inspection (design §5.1 step 7), ARF-006 repo-wide checks (step 8), ARF-007 \`TestParseSpotifyURL\` no-regression rows (step 9). <!-- sdd-owner: parent -->`

## Workload / PR Boundary

~166 insertions / 12 deletions across 5 files (4 modified + 1 new) — within the ~180–200 estimate, far under the 400-line budget. Single PR, no chain. Post-apply bounded review: standard tier, one `review-reliability` lens (parent-owned).

## Commit State

**Nothing committed, nothing staged** — per delegation rules (tasks artifact specifies no commit strategy; orchestrator review step owns commit/PR lifecycle). Work-unit-commits skill loaded: the natural work-unit boundary for this PR is the whole change as one reviewable unit (tests travel with code; rollback is a single revert per design §8.2).

## Round 2: review fixes + scope expansion (ARF-010)

**Trigger:** post-approval user decision — the risk lens found R1-01 (WARNING): yt-dlp option injection via missing `--` separator, newly reachable in Auto+Spotify-configured mode by the routing change.

**Round-1 SUGGESTION fixes applied (verify/review round 2):**

1. `internal/adapters/spotify/url.go` — `strings.ToLower(parsed.Host)` before host comparisons (case-insensitive, fixes R1-01 round 1); doc comment rewritten (no HTTP(S) over-spec, no dangling ARF refs — fixes R3-1/R3-2).
2. `internal/tui/update.go:118` — gate comment reworded to "looks like a URL or a spotify: URI" (fixes R3-3).
3. `internal/adapters/spotify/url_test.go` — new row `{name: "uppercase host", url: "https://OPEN.SPOTIFY.COM/track/...", want: true}`.

**Scope expansion (ARF-010, authorized):** `--` option terminator before the URL in both yt-dlp invocations:

- `internal/adapters/searcher/ytdlp.go` — extracted pure `searchArgs(url)` with trailing `--`; new `TestSearchArgs_OptionTerminatorBeforeURL`.
- `internal/adapters/downloader/ytdlp.go` — `"--"` before `media.URL` in `buildArgs`; new `TestBuildArgs_OptionTerminatorBeforeURL`.
- spec/design/tasks updated (ARF-010, design §2.6/§3.6, T7).

**Gates re-verified (assistant + subagent):** `go vet ./...` OK · `go build ./...` OK · `go test -short -count=1 ./...` 11 packages ok · gofmt clean · working-tree tree hash == frozen candidate tree.

**Review:** successor lineage `review-d684a7d409062590ece22c39a66ab5a9f87687bcae463853d58af0c0d1dc51da` approved (round 2, before ARF-010 scope expansion); a third review round covers the final candidate with ARF-010.
