# Tasks: Adapter Routing Fix (Auto URL Detection + Spotify URI Support + Minimal CI)

**Change:** `adapter-routing-fix`
**Date:** 2026-08-01
**Estimated total:** ~200–220 lines (7 files: 1 new, 6 modified) — single PR, no chain
**Status:** Draft
**Test runner:** `go test -short ./...` (STRICT TDD: RED tests first, then GREEN implementation)

---

## Review Workload Forecast

| Field | Value |
| ------- | ------- |
| Estimated changed lines | ~200–220 (url.go ~20, url_test.go ~45, update.go ~12, update_test.go ~85, ci.yml ~20, searcher/ytdlp.go ~10, downloader/ytdlp.go ~3) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | ask-on-risk (no risk trigger → single PR, no chain) |
| Chain strategy | pending (inert — no chain is recommended for this change) |
| Review tier | Standard (no auth/update/security/payments paths; no >400-line code diff) |
| Recommended lens | one `review-reliability` lens (dominant risk: behavior/state/tests/regressions per the review-lens risk table) |

```text
Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low
```

> **Why single PR:** the change totals ~180–200 changed lines across 5 files, well under the 400-line budget. `ask-on-risk` finds no risk trigger, so no chain is required and the chain-strategy field is inert. No 4R fan-out: exactly one standard lens (`review-reliability`) runs inside the post-apply bounded review.

---

## Task Overview

```
T1. RED   IsSpotifyURL table (url_test.go) ............ ~45 lines   (no deps)
T2. GREEN IsSpotifyURL host helper (url.go) ........... ~20 lines   (depends: T1)
T3. RED   routing table + gate tests (update_test.go) . ~85 lines   (depends: T2 — strict §5.1 sequence)
T4. GREEN update.go: signature, Auto branch, gate ..... ~12 lines   (depends: T3)
T5. GATE  go vet / build / test -short + no-regression  —          (depends: T1–T4)
T6. NEW   .github/workflows/ci.yml .................... ~20 lines   (depends: T5 — Go green first, design §7.6)
T7. RED+GREEN yt-dlp `--` option terminator .......... ~15 lines   (depends: T5 — scope expansion ARF-010, design §3.6)
```

Requirements coverage: ARF-001 (T3/T4), ARF-002 (T3/T4), ARF-003 (T1/T2), ARF-004 (T6), ARF-005 (T6), ARF-006 (T4/T5, amended), ARF-007 (T2/T5), ARF-008 (T3/T4), ARF-009 (T5), ARF-010 (T7). All spec Test Specifications are covered: TUI searcher routing table (T3), TUI URL-mode input gate (T3 + existing tests pinned in T5), IsSpotifyURL table (T1), repository gates (T5), CI workflow file inspection (T6 self-check; formal inspection in the verify phase per design §5.1 step 7), yt-dlp option terminator (T7).

---

# Single PR — Auto URL routing + Spotify URI gate + CI (~180–200 lines)

- **Start:** `TestIsSpotifyURL` (red) in the `spotify` package.
- **End:** `.github/workflows/ci.yml` committed after the Go work is green.
- **Prior deps:** none.
- **Follow-up:** verify phase (design §5.1 steps 7–9) — CI file inspection, `config.yaml` byte-identity, `ports.Searcher` unchanged, `TestParseSpotifyURL` no-regression rows.
- **Out of scope:** `parseSpotifyURL`/`validateTrack` (ARF-007), `ports.Searcher`/orchestrator (ARF-006), `openspec/config.yaml`, the `audio-quality` change, explicit source-mode semantics. The downloader and yt-dlp searcher adapters receive ONLY the scoped `--` terminator fix (ARF-010, design §2.6/§3.6).
- **Requirements:** ARF-001 … ARF-010.
- **Verification:** `go vet ./... && go build ./... && go test -short ./...` (network-gated integration tests skip under `-short`).

## Task T1 — RED: `TestIsSpotifyURL` table (ARF-003)

**Files:** `internal/adapters/spotify/url_test.go` (MODIFIED — add `TestIsSpotifyURL`; existing `TestParseSpotifyURL` untouched)
**TDD phase:** RED — `IsSpotifyURL` is not defined yet, so the test file must fail to compile.

**Steps:**

1. Add `TestIsSpotifyURL`, a table-driven test mirroring the existing `TestParseSpotifyURL` convention (struct slice + `t.Run(tt.name, ...)` subtests).
2. Include the 15 rows from the spec's Test Specifications (§ "Test: IsSpotifyURL table"): the 9 `true` cases (`https://open.spotify.com/track/{id}` / `playlist` / `album` / `artist`, `https://www.spotify.com/...`, and `spotify:track:{id}` / `playlist` / `album` / `artist`) and the 6 `false` cases (`https://music.youtube.com/watch?v=...`, `https://youtube.com/watch?v=...`, `https://evilspotify.com/track/x`, `https://spotify.com.evil.example/track/x`, `""`, `"   "`).
3. Add a 16th row `https://spotify.com/track/x` → `true` (design §2.1 decision table): it exercises the `host == "spotify.com"` exact-match branch, which the spec's 15 rows never hit. (Design §3.1 counts "sixteen rows" for this reason; the spec table enumerates 15 — both are satisfied.)
4. Optionally add a `"  spotify:track:x  "` → `true` row to pin design §7.4 (TrimSpace inside the helper).

**Watch for (design §7):** keep every row lowercase — the `spotify:` prefix check is case-sensitive (§7.1) and NO case-folding row belongs in this test; do NOT add port rows like `open.spotify.com:443` — explicit ports are deliberately not handled (§7.2).

**Verification:** `go test ./internal/adapters/spotify/` fails to compile (`IsSpotifyURL` undefined) — the RED state.

**Acceptance:**

- [x] All spec table rows (15) present, table-driven, `t.Run` subtests; existing `TestParseSpotifyURL` rows untouched. <!-- sdd-owner: implementation -->
- [x] `go test ./internal/adapters/spotify/` is red at this point (compile failure proves the test exists before the implementation). <!-- sdd-owner: implementation -->

**Estimated lines:** ~45. **Dependencies:** none. **Risk:** Low.

## Task T2 — GREEN: `IsSpotifyURL` host helper (ARF-003, ARF-007)

**Files:** `internal/adapters/spotify/url.go` (MODIFIED — add `IsSpotifyURL` directly above `parseSpotifyURL`; nothing else)
**TDD phase:** GREEN — implement to make T1 pass; then REFACTOR (doc comment per design §2.1).

**Steps:**

1. Add the exported function exactly per design §2.1: `TrimSpace` first (empty → `false`), then `strings.HasPrefix(rawURL, "spotify:")` → `true`, then `url.Parse` (error → `false`), then host rule `host == "spotify.com" || strings.HasSuffix(host, ".spotify.com")`.
2. `net/url` and `strings` are already imported by `url.go` — **no new imports** (verified: current imports are `errors`, `net/url`, `regexp`, `strings`).
3. Do NOT modify `parseSpotifyURL` or `validateTrack` (ARF-007); do NOT align `parseSpotifyURL`'s looser host check (deliberate, safe-direction divergence — design §6.4).

**Watch for (design §7):** case-sensitive prefix, keep `HasPrefix` exact (§7.1); do NOT add `net.SplitHostPort` port stripping — not required by any spec case, keep minimal (§7.2); scheme-less `spotify.com` → `url.Parse` yields empty Host → `false`, correct (§7.3); `TrimSpace` runs first so `" spotify:track:x "` is recognized (§7.4).

**Verification:** `go test ./internal/adapters/spotify/` passes (new table + existing `TestParseSpotifyURL`).

**Acceptance:**

- [x] `IsSpotifyURL(url string) bool` exported from package `spotify` per design §2.1 (decision table rows all pass); no new imports in `url.go`. <!-- sdd-owner: implementation -->
- [x] `go test ./internal/adapters/spotify/` green; `parseSpotifyURL`/`validateTrack` byte-identical to pre-change. <!-- sdd-owner: implementation -->

**Estimated lines:** ~20. **Dependencies:** T1. **Risk:** Low.

## Task T3 — RED: routing table + URL-mode gate tests (ARF-001, ARF-002, ARF-008)

**Files:** `internal/tui/update_test.go` (MODIFIED — add 3 tests; existing tests untouched)
**TDD phase:** RED — `selectedSearcher()` takes no argument yet, so the new tests fail to compile; once the signature is widened with the old body, case 1 is behaviorally red (Auto returns the Spotify searcher for a YouTube URL — the spec's stated red condition for issue #19).

**Steps:**

1. Add `TestSelectedSearcherRouting`: two distinct sentinels `yt := &stubSearcher{}` and `sp := &stubSearcher{}`; `Model{sourceMode: tt.mode, searcher: yt}` literals with `spotifySearcher: sp` only when `tt.configured` (never `NewModel`); assert **identity** (`got != tt.want`) so a wrong-but-non-nil searcher still fails. Cover the spec's 8 routing cases; use the design §3.3a 9-row form (case 6 "SourceYouTube ignores the URL" split into two rows: Spotify URL and `spotify:` URI — both must return `yt`).
2. Add `TestURLMode_SpotifyURIAccepted` mirroring `TestURLMode_ValidURLStillResolves`: `Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}`, `m.Input.SetValue("spotify:track:4iV5W9uYEdYUVa79Axb7Rh")`, `Update(tea.KeyMsg{Type: tea.KeyEnter})` → `Screen == ScreenResolving`, `cmd != nil`, `InputError` does NOT contain "That doesn't look like a URL".
3. Add `TestURLMode_OtherSchemeStillBlocked` mirroring `TestURLMode_NonURLSuggestion`: input `"itunes:track:123"` + Enter → stays `ScreenInput`, `InputError` contains "That doesn't look like a URL" (ARF-008).
4. Do NOT modify the existing gate tests that pin no-regression: `TestEnterEmptyURLShowsError`, `TestInputWhitespaceURL`, `TestURLMode_NonURLSuggestion`, `TestURLMode_ValidURLStillResolves` — they must stay green and unchanged (design §5.1).

**Watch for (design §7):** do NOT add a case where `SPOTIFY:track:x` is accepted — the gate is case-sensitive (§7.1); do NOT add a rejection expectation for bare `spotify:track` (no ID) — it is admitted by gate + router and rejected downstream by `parseSpotifyURL` with "invalid Spotify URI format" (§7.7); `" spotify:track:x "` (surrounding whitespace) is admitted — TrimSpace runs before the gate (§7.4).

**Verification:** `go test ./internal/tui/` fails on the new tests (compile — `selectedSearcher` arity); pre-existing tests stay green.

**Acceptance:**

- [x] Routing table covers spec cases 1–5 (Auto), 6 (SourceYouTube, both URL and URI forms), 7a/7b (SourceSpotify), with sentinel identity assertions. <!-- sdd-owner: implementation -->
- [x] The 2 new gate tests assert the `spotify:` URI is accepted (ARF-002) and `itunes:` stays blocked (ARF-008); new tests red before T4. <!-- sdd-owner: implementation -->

**Estimated lines:** ~85. **Dependencies:** T2 (strict §5.1 sequence — spotify package green before moving on). **Risk:** Low.

## Task T4 — GREEN: `update.go` — signature, Auto branch, gate, call site (ARF-001, ARF-002, ARF-008, ARF-006)

**Files:** `internal/tui/update.go` (MODIFIED)
**TDD phase:** GREEN — implement to make T3 pass; then REFACTOR (doc comment on `selectedSearcher` per design §2.2).

**Steps:**

1. Add import `"github.com/Juanstudy/music-downloader/internal/adapters/spotify"` with the other `internal/...` imports, after the stdlib block (design §7.8 — gofmt-stable). No cycle: `internal/adapters/spotify` imports only stdlib + `internal/core/domain` + `internal/core/ports` (design §1.3, ARF-006).
2. Relax the URL-mode gate in `handleInputKeys`: `if !strings.Contains(val, "://") && !strings.HasPrefix(val, "spotify:") { ... "That doesn't look like a URL ..." }`. The trimmed-empty check ("Please enter a URL") stays first and unchanged (design §2.4, ARF-002).
3. Update the call site in `startResolve` (currently `update.go:158`): `return m, resolveCmd(m.selectedSearcher(url), url)` (design §2.3).
4. Change `selectedSearcher` to `func (m Model) selectedSearcher(url string) ports.Searcher` (currently `update.go:162`) with the design §2.2 body: `SourceSpotify` → `spotifySearcher` when non-nil else `searcher`; `SourceYouTube` → `searcher`; `SourceAuto` → `spotifySearcher` only when `m.spotifySearcher != nil && spotify.IsSpotifyURL(url)` else `searcher`; `default` → `searcher`. `SourceSpotify`, `SourceYouTube`, and `default` are byte-for-byte identical to today.

**Watch for (design §7):** the gate and the `IsSpotifyURL` prefix check stay case-sensitive — `SPOTIFY:track:x` is rejected at the gate and routes to yt (§7.1); the Auto branch uses the imported `spotify.IsSpotifyURL` — do not duplicate host logic in `tui` (design §6.2).

**Verification:** `go test ./internal/tui/` green; `go vet ./internal/tui/` exit 0.

**Acceptance:**

- [x] `selectedSearcher(url)` is URL-aware only in `SourceAuto`; explicit-mode branches byte-identical (ARF-001 cases 6–7). <!-- sdd-owner: implementation -->
- [x] `go test ./internal/tui/` green including the 4 pre-existing gate tests; `internal/adapters/spotify` has no new imports (no `internal/tui` — ARF-006). <!-- sdd-owner: implementation -->

**Estimated lines:** ~12 changed. **Dependencies:** T3. **Risk:** Low.

## Task T5 — GATE: repository gates + no-regression (ARF-009, ARF-006, ARF-007)

**Files:** verification only (no source edits)
**TDD phase:** REFACTOR / gate — no new behavior.

**Steps:**

1. `go vet ./...` — exit 0.
2. `go build ./...` — exit 0.
3. `go test -short ./...` — exit 0 and hermetic: network-gated yt-dlp/Spotify integration tests MUST skip under `-short`, not run (ARF-009).
4. No-regression (ARF-006): `openspec/config.yaml` byte-identical to pre-change (`git diff --exit-code openspec/config.yaml`); `ports.Searcher`/orchestrator/downloader untouched; `internal/adapters/spotify` does not import `internal/tui`; the `audio-quality` change's files show zero diff.
5. ARF-007: `parseSpotifyURL`/`validateTrack` unchanged; the existing `TestParseSpotifyURL` playlist/album/artist rows still pass with the "only track URLs are supported in this version" message (formal re-verification in the verify phase, design §5.1 step 9).

**Acceptance:**

- [x] `go vet ./... && go build ./... && go test -short ./...` all exit 0; integration tests skipped under `-short`. <!-- sdd-owner: implementation -->
- [x] `config.yaml` byte-identical; spotify package import set unchanged; no diff in `audio-quality` files (ARF-006/ARF-007). <!-- sdd-owner: implementation -->

**Estimated lines:** —. **Dependencies:** T1–T4. **Risk:** Low.

## Task T6 — NEW: `.github/workflows/ci.yml` (ARF-004, ARF-005)

**Files:** `.github/workflows/ci.yml` (NEW — the repo currently has no `.github/workflows/` directory)
**TDD phase:** config artifact — written only after the Go work is green (design §7.6); strict TDD applies to executable Go, this file is verified by inspection.

**Steps:**

1. Create `.github/workflows/ci.yml` byte-for-byte per design §3.5:
   - `on:` with `push:` and `pull_request:` as **empty map entries** (all branches, no path filters — no `paths:`/`paths-ignore:` keys anywhere).
   - One job `ci` on `runs-on: ubuntu-latest` with steps in order: `actions/checkout@v4` → `actions/setup-go@v5` with `with: { go-version-file: go.mod }` → `run: go vet ./...` → `run: go build ./...` → `run: go test -short ./...`.
   - `-short` MUST be present (hermetic CI — mirrors `openspec/config.yaml` runner, design §6.7, ARF-004).
2. Self-check the spec's "CI workflow (file inspection)" test before finishing: file exists; triggers include `push` + `pull_request` with no path filters; `ubuntu-latest`; `checkout@v4`; `setup-go@v5` with `go-version-file: go.mod`; the three commands in order; NO linter, coverage gate, caching, `matrix` key, or path filters (ARF-005). (The formal inspection test re-runs in the verify phase — design §5.1 step 7.)
3. If a YAML parser is available (`python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml'))"`), run it to confirm the file parses; otherwise rely on the §3.5 shape match.

**Watch for (design §7):** use the **map form** for `on:` (empty `push:`/`pull_request:` entries), not the list form — both satisfy ARF-004/005, the design pins the map form (§7.5); do NOT add caching, matrix, path filters, or a linter step even though GitHub would tolerate them (ARF-005).

**Verification:** file present at `.github/workflows/ci.yml`; content matches design §3.5; YAML parses.

**Acceptance:**

- [x] `.github/workflows/ci.yml` exists with map-form `on: {push, pull_request}`, no path filters, single `ubuntu-latest` job, steps in order checkout@v4 → setup-go@v5 (`go-version-file: go.mod`) → vet → build → `test -short`. <!-- sdd-owner: implementation -->
- [x] No linter/coverage/caching/matrix/path-filter references anywhere in the file (ARF-005); file parses as YAML. <!-- sdd-owner: implementation -->

**Estimated lines:** ~20. **Dependencies:** T5 (Go green first — design §7.6). **Risk:** Low.

**Parent actions (after implementation lands):**

- [ ] Start or reuse bounded review for the single PR: standard tier, exactly one `review-reliability` lens (dominant risk: behavior, state, tests, determinism, regressions). <!-- sdd-owner: parent -->
- [ ] Verify-phase launch: CI file inspection (design §5.1 step 7), ARF-006 repo-wide checks (step 8), ARF-007 `TestParseSpotifyURL` no-regression rows (step 9). <!-- sdd-owner: parent -->

---

## Risk Assessment

| Task | Risk | Mitigation |
| ---- | ---- | ---------- |
| T1/T2 (IsSpotifyURL) | Low | Table pins all 4 hostile host cases (`evilspotify.com`, `spotify.com.evil.example`) + empty/whitespace; exact + `.spotify.com`-suffix rule is the security boundary (design §6.4) |
| T3/T4 (routing + gate) | Low | Sentinel-identity assertions catch wrong-but-non-nil searchers; explicit-mode branches byte-identical; only `spotify:` admitted (ARF-008) |
| T4 (import direction) | None | `spotify` imports only stdlib + core — no cycle possible (design §1.3); pinned in T5 |
| T5 (no-regression) | Low | `config.yaml` byte-check + zero-diff checks + hermetic `-short` suite |
| T6 (CI) | Low | Shape pinned to design §3.5; mirrors the local `openspec/config.yaml` runner exactly (design §6.7) |

**Overall risk:** Low — additive, 5 files, well under the 400-line budget; the only real hazard (Auto-mode routing) is pinned by the 9-row identity table and the 2 new gate tests.

---

## Rollback Notes

Single PR, independently revertible (design §8.2): revert `internal/tui/update.go` (signature, Auto branch, gate, call site, import), revert `internal/tui/update_test.go` and `internal/adapters/spotify/url_test.go` to pre-change, remove or keep the additive `IsSpotifyURL` export (removing it is safe — it is only called from `update.go`), delete `.github/workflows/ci.yml`. No data, config, or persisted state is touched; zero migration cost. `openspec/config.yaml`, `ports.Searcher`, `parseSpotifyURL`/`validateTrack`, and the `audio-quality` change are untouched by the implementation, so no cascading revert is possible.

## Task T7 — RED+GREEN: yt-dlp `--` option terminator (ARF-010, design §3.6)

**Files:** `internal/adapters/searcher/ytdlp.go` + `ytdlp_test.go`, `internal/adapters/downloader/ytdlp.go` + `ytdlp_test.go`

**TDD phase:** RED → GREEN (scope expansion authorized by the user after review finding R1-01).

1. **RED** — `TestSearchArgs_OptionTerminatorBeforeURL` in `internal/adapters/searcher/ytdlp_test.go`: table-driven over `searchArgs(url)` asserting `args[len-2] == "--"` and `args[len-1] == url` (rows: regular URL, `--config-location=http://evil.example/x`, `--output`). Fails to compile — `searchArgs` undefined.
2. **GREEN** — extract pure `searchArgs(url string) []string` in `internal/adapters/searcher/ytdlp.go` returning `["--flat-playlist", "--dump-json", "--ignore-errors", "--no-warnings", "--", url]`; `Search` calls it. Test passes.
3. **RED** — `TestBuildArgs_OptionTerminatorBeforeURL` in `internal/adapters/downloader/ytdlp_test.go`: `buildArgs(media, outputDir, "")` asserting `args[len-2] == "--"` and `args[len-1] == media.URL`. Fails.
4. **GREEN** — insert `"--"` immediately before `media.URL` in `buildArgs` (`internal/adapters/downloader/ytdlp.go`). Test passes; existing `TestBuildArgs_*` stay green.

**Why:** review finding R1-01 (risk lens) — without `--`, a pasted string like `--config-location=http://evil.example/x` (passes the `://` gate, not a Spotify URL) is parsed by yt-dlp as an OPTION, enabling option injection (remote config fetch + `--exec`). The routing change (T4) makes this reachable in Auto+Spotify-configured mode.

**Acceptance:**

- [x] `searchArgs` places `--` immediately before the URL; pinned by unit test (no yt-dlp needed)
- [x] `buildArgs` places `--` immediately before `media.URL`; pinned by unit test
- [x] `go test -short -count=1 ./...` green (11 packages ok)
- [x] gofmt clean

**Estimated lines:** ~15. **Dependencies:** T5. **Risk:** Low (pure argv change, `--` is a no-op for legitimate URLs).
