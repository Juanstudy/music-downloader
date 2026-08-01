# Proposal: Adapter Routing Fix (Auto Mode URL Detection + Spotify URI Support + Minimal CI)

**Change:** `adapter-routing-fix`
**Status:** Proposed
**Date:** 2026-08-01

---

## 1. Problem / Opportunity

GitHub issue #19 reports a real, user-visible bug: with source mode `auto` and search mode `URL`, pasting a **YouTube Music URL** routes it to the **Spotify adapter** whenever Spotify credentials are configured, producing `✗ not a Spotify URL`. Resolution fails for a URL that should work out of the box.

Root cause is a routing defect, not a resolver defect:

- `selectedSearcher()` (`internal/tui/update.go:156-170`) **does not receive the URL**. The `SourceAuto` branch returns `m.spotifySearcher` whenever it is non-nil, regardless of what was pasted.
- `spotifySearcher` is non-nil exactly when credentials are configured (`cmd/music-dl/main.go:55-63`), so the bug only manifests for users who configured Spotify — the users who get the worst experience: their YouTube links silently break while unconfigured users are fine.
- The call site (`update.go:152`) already has the URL in hand: `resolveCmd(m.selectedSearcher(), url)` — the fix is a signature change, not a plumbing project.

Two adjacent product gaps make this change worth doing end-to-end in one slice:

- **Latent URI dead-end:** `parseSpotifyURL` already supports `spotify:track:{id}` URIs (`internal/adapters/spotify/url.go:35-43`), but the TUI URL-mode input gate (`update.go:113`) rejects anything without `://` ("That doesn't look like a URL"), so URIs can never reach the resolver. The adapter supports a form the product blocks.
- **No regression safety net:** there is no `.github/workflows/` directory at all. Nothing catches a routing regression like this one today; `go test ./...` even fails locally on network access because the yt-dlp integration tests are `testing.Short()`-gated but nobody runs `-short`.

Business outcome: **Auto mode becomes what its name promises** — it auto-detects the URL's host and routes accordingly — Spotify credentials stop hijacking YouTube links, Spotify URI support works coherently end-to-end, and a minimal CI pipeline turns "green" into a machine-checked fact instead of a hope.

## 2. Target Users & Situations

| User | Situation |
| ------ | ----------- |
| User with Spotify configured | Pastes a YouTube Music link in Auto mode and gets `✗ not a Spotify URL` (issue #19). After the fix: downloads work. |
| User with Spotify configured | Pastes a `spotify:track:{id}` URI (copied from the Spotify share menu) in URL mode. Today: "That doesn't look like a URL". After the fix: resolves. |
| User without Spotify configured | Auto mode already works for them; behavior is unchanged and must stay unchanged. |
| Maintainer / contributor | Merges code with zero CI feedback today; every change is a manual trust exercise. After the fix: vet/build/test gate on every push and PR. |

## 3. Current-State Gap

- `selectedSearcher()` (`internal/tui/update.go:156-170`) has signature `func (m Model) selectedSearcher() ports.Searcher` and no URL parameter; the `SourceAuto` branch returns `m.spotifySearcher` whenever non-nil — no URL inspection at all.
- `resolveCmd` call site (`update.go:152`) passes the URL to the searcher invocation but cannot influence searcher selection.
- `spotifySearcher` is wired non-nil only when `ClientID` and `ClientSecret` are both set (`cmd/music-dl/main.go:55-63`), making the bug conditional on configuration.
- URL-mode input gate (`update.go:113`): `if !strings.Contains(val, "://")` → `"That doesn't look like a URL. Press 's' to switch to Search mode."` — blocks all `spotify:` URIs.
- `parseSpotifyURL` (`internal/adapters/spotify/url.go`) already parses both `https://open.spotify.com/track/{id}` and `spotify:track:{id}`, and rejects album/playlist/artist ("only track URLs are supported in this version"). Host check today (`url.go:44`): `strings.HasSuffix(parsed.Host, "open.spotify.com") || strings.HasSuffix(parsed.Host, "spotify.com")` — note this suffix check would also match a hostile `evilspotify.com` host.
- No unit tests exist for `selectedSearcher` routing (`internal/tui/update_test.go` covers other behavior; verified).
- No `.github/workflows/` directory exists; `internal/adapters/downloader/ytdlp_test.go` (and `internal/adapters/searcher/ytdlp_test.go`) skip on `testing.Short()` but nothing runs `-short`, so a plain `go test ./...` attempts live network calls (HTTP 403 from YouTube).
- `openspec/config.yaml` already declares the local test runner: `go test -short ./...`, `testing.runner.available: true`, `strict_tdd: true` — the CI workflow should mirror this exact command.

## 4. Proposed Solution

### 4.1 Routing fix: URL-aware `selectedSearcher`

- Change the signature to `func (m Model) selectedSearcher(url string) ports.Searcher`.
- Update the call site to `resolveCmd(m.selectedSearcher(url), url)` (`update.go:152`).
- `SourceAuto` branch becomes URL-driven: use the Spotify searcher **only when** `m.spotifySearcher != nil && IsSpotifyURL(url)`; otherwise fall back to `m.searcher` (yt-dlp), which is the general-purpose resolver.
- Explicit modes keep today's semantics exactly: `SourceSpotify` → `spotifySearcher` if non-nil else yt; `SourceYouTube` → always yt; `default` → yt.

### 4.2 Single source of truth: `spotify.IsSpotifyURL`

- Export `IsSpotifyURL(url string) bool` from `internal/adapters/spotify` (new helper in `url.go` or alongside it). No import cycle: `spotify` does not import `tui`.
- Accepts:
  - HTTP(S) URLs whose host is exactly `spotify.com` or ends with `.spotify.com` (covers `open.spotify.com`, `www.spotify.com`, regional subdomains).
  - `spotify:` scheme-prefixed URIs (any entity form: `spotify:track:...`, `spotify:album:...`, `spotify:playlist:...`, `spotify:artist:...`).
- Rejects: empty/whitespace input, `music.youtube.com`, `youtube.com`, `evilspotify.com` (deliberately stricter than the existing `HasSuffix` logic — see Trade-offs), any other host.
- The helper answers **"is this URL hosted by Spotify?"** — host-level routing only. Entity-level validation (track-only) stays in `parseSpotifyURL`, which is untouched.

### 4.3 TUI input gate: admit `spotify:` URIs

- Relax `update.go:113` so URL mode accepts either `://` (current) or the `spotify:` prefix: `if !strings.Contains(val, "://") && !strings.HasPrefix(val, "spotify:")` → error.
- This makes the adapter's existing URI support reachable end-to-end; `spotify:track:{id}` flows to `startResolve` → `selectedSearcher(url)` → Spotify searcher → `parseSpotifyURL`, which already handles the URI form.

### 4.4 Unit tests (strict TDD — red first)

- `internal/tui/update_test.go` — routing table for `selectedSearcher` using `Model{}` literals (existing TUI test convention), covering:
  1. **Auto + YouTube Music URL + Spotify configured → `m.searcher`** (the issue #19 regression case).
  2. Auto + `open.spotify.com/track/{id}` + configured → `m.spotifySearcher`.
  3. Auto + `spotify:track:{id}` + configured → `m.spotifySearcher`.
  4. Auto + Spotify URL **without** credentials → `m.searcher`.
  5. Auto + non-Spotify URL without credentials → `m.searcher`.
  6. `SourceYouTube` + any URL → `m.searcher`.
  7. `SourceSpotify` + configured → `m.spotifySearcher`; `SourceSpotify` + not configured → `m.searcher`.
- `internal/adapters/spotify/url_test.go` — table for `IsSpotifyURL`:
  - `https://open.spotify.com/track/{id}` → true; `.../playlist/...`, `.../album/...`, `.../artist/...` → true (host-level).
  - `spotify:track:{id}`, `spotify:playlist:{id}` → true.
  - `https://music.youtube.com/watch?v=...` → false; `https://youtube.com/...` → false; `https://spotify.com.evil.example/track/x` → false; `evilspotify.com/track/x` → false; `""` and whitespace-only → false.
- Gate relaxation covered by the routing table inputs (URI forms reach `selectedSearcher`) plus an existing-style input-key test asserting `spotify:track:{id}` no longer produces "That doesn't look like a URL".

### 4.5 Minimal CI

- New `.github/workflows/ci.yml` per user scope decision (minimal, no linter, no coverage gate):
  - Triggers: `push` and `pull_request` (default branches, no path filters).
  - Job on `ubuntu-latest`; `actions/checkout@v4`; `actions/setup-go@v5` with `go-version-file: go.mod` (Go 1.26.3).
  - Steps: `go vet ./...` → `go build ./...` → `go test -short ./...` (mirrors the `openspec/config.yaml` runner; `-short` skips the network-gated yt-dlp integration tests so CI stays hermetic).

## 5. Scope / First Slice

| In scope | Out of scope |
| ---------- | ------------ |
| `selectedSearcher(url string)` signature + `SourceAuto` URL-driven branch | Playlist/album/artist support inside the Spotify adapter (`validateTrack` stays track-only) |
| `spotify.IsSpotifyURL` exported helper (host + `spotify:` prefix) | Any change to `parseSpotifyURL`'s internal host check or entity validation |
| TUI URL-mode gate relaxation for the `spotify:` prefix | Accepting other URI schemes (`itunes:`, `tidal:`, …) at the gate |
| Routing + `IsSpotifyURL` unit tests (strict TDD) | Search-mode (`query`) changes of any kind |
| `.github/workflows/ci.yml` — `go vet` + `go build` + `go test -short ./...` on push/PR | Linters, coverage gates, caching, matrix builds, path filters in CI |
| Explicit-mode semantics preserved (`SourceSpotify`/`SourceYouTube` unchanged) | Audio-quality work (separate change, already filed) |
| yt-dlp `--` option terminator in searcher + downloader argv (ARF-010, added post-review from risk-lens finding R1-01) | Refactoring `main.go` credential wiring or the Tab source-mode cycle |
| Update `openspec/specs/` capability docs if they describe routing | Refactoring `main.go` credential wiring or the Tab source-mode cycle |

## 6. Non-Goals (Explicit)

- **NO Spotify playlist/album/artist downloads.** The adapter remains track-only; `IsSpotifyURL` routes by host, and `parseSpotifyURL` keeps rejecting non-track entities with the existing message. Broader Spotify support is a separate change.
- **NO behavior change for users without Spotify credentials** — Auto mode keeps resolving through yt-dlp exactly as today.
- **NO change to explicit source modes** — `SourceSpotify` and `SourceYouTube` keep their current semantics byte-for-byte.
- **NO new URI schemes** at the input gate beyond `spotify:`.
- **NO query/search-mode work** — `startQuerySearch` is untouched.
- **NO linter or coverage gate in CI** — user decision: minimal CI only. No caching/matrix/path-filtering.
- **NO audio-quality work**, no `internal/config` changes, no orchestrator/port changes.
- **NO yt-dlp behavior change beyond the `--` terminator** — the searcher and downloader adapters receive exactly one scoped argv change (ARF-010) and nothing else.
- Does NOT modify `openspec/config.yaml` (already corrected; treated as fixed input).

## 7. Product Constraints

| Constraint | Decision |
| ------------ | ---------- |
| Auto mode semantics | URL-host-driven: recognized Spotify URL → Spotify searcher; anything else → yt-dlp (conservative fallback, since yt is the general resolver) |
| Spotify credentials still required | Yes — `spotifySearcher` remains nil without them; Auto degrades to yt, same as explicit Spotify mode does today |
| Track-only restriction | Unchanged; routing and entity validation are separate concerns |
| `spotify:` URI support | Full and coherent: gate admits it, router recognizes it, resolver already parses it |
| CI scope | `go vet` + `go build` + `go test -short ./...` on push and pull_request — nothing more |
| Local test runner | `go test -short ./...` (per `openspec/config.yaml`); integration tests remain `testing.Short()`-gated |
| Delivery | Single PR (`ask-on-risk`); estimated diff is well under the 400-line budget |

## 8. Business Trade-offs

| Trade-off | Implication |
| ----------- | ------------- |
| URL-aware routing vs. credential-gated routing | Auto mode finally matches its label; the small cost is one extra parameter threaded through `selectedSearcher` |
| Host-level routing (playlist/album URLs go to Spotify, then get rejected) | Matches today's explicit-Spotify-mode behavior; the error message ("only track URLs are supported in this version") is clear and already shipped — routing stays a host question, validation stays an entity question |
| Stricter host check in `IsSpotifyURL` vs. parity with `parseSpotifyURL`'s `HasSuffix` | Rejects `evilspotify.com`; slightly stricter than the resolver's internal check. Safe direction: a dubious URL routes to yt (general resolver), never to the credentials-gated Spotify searcher. The resolver's check is left untouched this slice |
| Relaxing the gate for `spotify:` only | URI support works; other schemes still get the friendly "That doesn't look like a URL" hint — no half-baked generic-URI support |
| Minimal CI vs. fuller gates | Cheap to merge, catches the class of bug that shipped #19; linter/coverage stay out by explicit user decision |
| New helper in `spotify` package vs. duplicating host logic in `tui` | Single source of truth for "is this a Spotify URL?"; `tui` stays adapter-agnostic about host details |

## 9. Edge Cases

| Edge Case | Behavior |
| ----------- | ---------- |
| Auto + YouTube Music URL + Spotify configured | → yt searcher (issue #19 fixed) |
| Auto + `open.spotify.com/track/{id}` + configured | → Spotify searcher → resolves |
| Auto + `spotify:track:{id}` + configured | Gate admits URI → Spotify searcher → resolves (URI form already supported by `parseSpotifyURL`) |
| Auto + `open.spotify.com/playlist/{id}` or `spotify:playlist:{id}` + configured | → Spotify searcher → rejected with "only track URLs are supported in this version" (entity rule, unchanged) |
| Auto + Spotify URL, **no credentials** | → yt searcher (same fallback as explicit Spotify mode without credentials) |
| Auto + `music.youtube.com` / `youtube.com` | → yt searcher |
| `evilspotify.com/track/x`, `spotify.com.evil.example/track/x` | `IsSpotifyURL` → false → yt searcher; hostile hosts never reach the credentials-gated adapter |
| Empty / whitespace-only input | URL mode still errors "Please enter a URL" (trimmed-empty check runs first, unchanged) |
| Non-`spotify:` URI schemes (`itunes:…`) | Still blocked by the gate — no generic URI support |
| Query search mode | Untouched; `selectedSearcher` is only reached from `startResolve` (URL mode) |
| Malformed `spotify:` input (`spotify:track` without ID) | Passes the gate, routes to Spotify, rejected by `parseSpotifyURL` ("invalid Spotify URI format") |
| CI on a machine without network | `-short` skips network-gated integration tests; unit tests are hermetic |

## 10. Risks

| Risk | Mitigation |
| ------ | ------------ |
| Regression for users *without* credentials | Their path never changes: Auto always resolved via yt; routing table case 4/5 pins this |
| Regression in explicit modes | `SourceSpotify`/`SourceYouTube` branches are byte-identical in behavior; pinned by routing table cases 6–7 |
| Scope creep toward adapter entity support | Hard non-goal: `parseSpotifyURL` and `validateTrack` untouched; `IsSpotifyURL` is host-level only |
| Gate relaxation admits junk | Only the `spotify:` prefix is admitted; resolver rejects malformed URIs with clear errors; `IsSpotifyURL` table covers the boundary |
| Host-suffix spoofing (`evilspotify.com`) | Stricter host rule in `IsSpotifyURL` (exact `spotify.com` or `.spotify.com` suffix); documented divergence from the resolver's internal check |
| CI false-green / locally red | Workflow mirrors the exact `openspec/config.yaml` runner (`go test -short ./...`); setup-go pins the repo's Go version via `go-version-file` |
| Routing fix flips behavior for configured users (the point) | Intentional; the broken path (every URL → Spotify) is the bug being fixed; explicit Spotify mode remains available via Tab for users who want forced Spotify |
| Diff budget (single PR) | Estimated ~160–200 changed lines across 5 files — well under 400; tests are table-driven |
| TDD discipline | `strict_tdd: true` in `openspec/config.yaml`: routing/`IsSpotifyURL` tests are written red-first, before the implementation edits |

## 11. Rollback

The change is small, additive, and fully reversible:

1. Revert `internal/tui/update.go` — restore `selectedSearcher()` without the URL parameter, restore the credential-gated `SourceAuto` branch, restore the `://`-only input gate.
2. Remove or keep `spotify.IsSpotifyURL` — additive export; removing it restores the previous package surface exactly.
3. Revert `internal/tui/update_test.go` and `internal/adapters/spotify/url_test.go` additions.
4. Delete `.github/workflows/ci.yml` (or revert the commit) — restores the no-CI status quo.

No data, config file, or persisted state is touched by this change, so rollback has zero migration cost.

## 12. Success Criteria

| Criterion | Measurement |
| ----------- | ------------- |
| Issue #19 fixed | Routing table test: Auto + `https://music.youtube.com/watch?v=...` + Spotify configured → `m.searcher` (passes, red before the fix) |
| Auto routes Spotify URLs correctly | Auto + `open.spotify.com/track/{id}` + configured → `m.spotifySearcher`; Auto + `spotify:track:{id}` + configured → `m.spotifySearcher` |
| No-credentials path unchanged | Auto + Spotify URL without credentials → `m.searcher`; Auto + YouTube URL without credentials → `m.searcher` |
| Explicit modes preserved | `SourceYouTube` → yt always; `SourceSpotify` → Spotify when configured, yt otherwise |
| `IsSpotifyURL` correct | Table passes: `open.spotify.com/*` (all entity paths) and `spotify:*` URIs → true; `music.youtube.com`, `youtube.com`, `evilspotify.com`, `spotify.com.evil.example`, empty, whitespace → false |
| URI support end-to-end | URL-mode input accepts `spotify:track:{id}` (no "That doesn't look like a URL"); it reaches `startResolve` and routes to the Spotify searcher |
| Local tests green | `go test -short ./...` passes (hermetic; network-gated integration tests skipped) |
| CI present and correct | `.github/workflows/ci.yml` exists, triggers on `push` + `pull_request`, runs `go vet ./...`, `go build ./...`, `go test -short ./...` |
| Manual E2E (issue #19 closure check) | With Spotify credentials configured, paste a YouTube Music URL in Auto/URL mode → resolves via yt-dlp; paste `spotify:track:{id}` → resolves via Spotify |
| No port/config drift | `ports.Searcher`, downloader, orchestrator, `openspec/config.yaml`, and the `audio-quality` change untouched |

## 13. Impact Summary

**Files affected:**

| File | Change |
| ------ | ------ |
| `internal/tui/update.go` | `selectedSearcher(url string)` + call site; URL-driven `SourceAuto` branch; gate relaxation for `spotify:` (~12 lines) |
| `internal/tui/update_test.go` | Routing table for `selectedSearcher` + gate-accepts-URI input test (~75 lines) |
| `internal/adapters/spotify/url.go` | New exported `IsSpotifyURL` helper (~18 lines) |
| `internal/adapters/spotify/url_test.go` | `IsSpotifyURL` table (~45 lines) |
| `.github/workflows/ci.yml` (new) | Minimal CI: vet + build + `test -short` on push/PR (~20 lines) |

**Review workload forecast:** estimated **~160–200 changed lines** across 5 files — comfortably within the single-PR budget (delivery strategy: `ask-on-risk`, no chain needed). No hot-path risk (no auth/security/payments code changed; routing logic only), so a single standard review lens applies.

## 14. Proposal Question Round (assumptions for review)

Scope decisions 1–3 are locked (single PR; minimal CI of vet/build/`-short`; full Spotify URI support end-to-end). The following micro-decisions were resolved by assumption and are worth a quick user check:

1. **Host-level routing for non-track Spotify URLs** — `open.spotify.com/playlist/...` and `spotify:playlist:...` route *to the Spotify searcher*, which then rejects them with "only track URLs are supported in this version" (identical to today's explicit Spotify mode). Alternative: treat non-track Spotify URLs as non-Spotify at the router and send them to yt. I assumed the former — OK?
2. **`evilspotify.com` → false via a stricter host check** (`host == "spotify.com"` or `.spotify.com` suffix), which is *stricter* than `parseSpotifyURL`'s existing `HasSuffix` logic. `parseSpotifyURL` itself is left untouched this slice. OK, or should the resolver's internal check be aligned in the same change?
3. **Gate admits any `spotify:` prefix** (track, album, playlist, artist URIs all pass the gate and are filtered by the resolver's track-only rule). Alternative: gate only `spotify:track:` so non-track URIs get the "doesn't look like a URL" hint. I assumed the former — OK?
4. **CI trigger scope** — `push` + `pull_request` on all branches, `ubuntu-latest`, Go version from `go-version-file: go.mod`, no path filters, no caching. OK?
5. **Preserving the Tab-mode source cycle exactly** — with the fix, Auto+YouTube URL now behaves like YouTube mode for the URL at hand; Tab still cycles Auto → Spotify → YouTube → Auto as today. No changes to the cycle. OK?

---
