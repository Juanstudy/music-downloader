# Archive Report — Adapter Routing Fix

**Change:** `adapter-routing-fix` — Auto URL detection + Spotify URI support + minimal CI
**Date:** 2026-08-01
**Store:** openspec (file-based)
**Status:** ✅ **ARCHIVED** — VERIFY PASS (10/10 ARF), bounded review round 3 approved (lineage `review-afebd24a0eb67c30`), canonical sync merged, change closed.

---

## Summary

Issue #19 fixed: Auto mode is now URL-host-driven (`selectedSearcher(url)`), the URL-mode gate admits `spotify:` URIs end-to-end, `spotify.IsSpotifyURL` is the single source of truth for routing, both yt-dlp invocations place a `--` option terminator before the URL (ARF-010), and a minimal hermetic CI workflow exists. All spec deltas merged into the canonical tree below; no source code or tests were modified by this phase (openspec artifacts only).

## Artifacts Read

| Artifact | Path | Present |
| --- | --- | --- |
| Proposal | `openspec/changes/adapter-routing-fix/proposal.md` | ✅ |
| Spec | `openspec/changes/adapter-routing-fix/spec/spec.md` | ✅ |
| Design | `openspec/changes/adapter-routing-fix/design/design.md` | ✅ |
| Tasks | `openspec/changes/adapter-routing-fix/tasks/tasks.md` | ✅ |
| Apply progress | `openspec/changes/adapter-routing-fix/apply-progress.md` | ✅ |
| Verify report | `openspec/changes/adapter-routing-fix/verify-report.md` | ✅ PASS 10/10 |
| Sync report | — | ❌ absent → archive-time sync fallback (explicitly approved by parent prompt, which instructs the canonical merge as archive step 1) |
| Config | `openspec/config.yaml` | ✅ (no `rules.archive` section; left untouched per constraints) |

## Domains Synced (archive-time sync fallback)

| Domain | Canonical target | Operation |
| --- | --- | --- |
| `internal-tui` | `openspec/specs/internal-tui/spec.md` | MODIFIED + ADDED (existing canonical) |
| `adapters-spotify` | `openspec/specs/adapters-spotify/spec.md` | ADDED (existing canonical) |
| `github-workflows` | `openspec/specs/github-workflows/spec.md` | **NEW canonical file created** |
| `adapters-searcher` | `openspec/specs/adapters-searcher/spec.md` | ADDED (existing canonical) |
| `adapters-downloader` | `openspec/specs/adapters-downloader/spec.md` | Test-spec mirror + change-history note (no requirement block) |

## Requirement Merge Log (all 10 ARF)

| Requirement | Domain | Operation | Notes |
| --- | --- | --- | --- |
| ARF-001 — URL-aware searcher selection | internal-tui | ADDED | Verbatim delta |
| ARF-002 — Input screen behavior | internal-tui | MODIFIED | Full-block replacement of canonical "Input screen behavior"; all 7 original scenarios carried forward (transformed) + 1 new `spotify:` URI scenario; none dropped |
| ARF-003 — Exported IsSpotifyURL host-level helper | adapters-spotify | ADDED | Verbatim delta |
| ARF-004 — CI workflow runs vet, build, hermetic tests | github-workflows | ADDED | Verbatim delta (new domain) |
| ARF-005 — CI stays minimal | github-workflows | ADDED | Verbatim delta (new domain) |
| ARF-006 — Port and configuration stability | internal-tui | ADDED | Change-level stability constraint; no shared/constraints canonical section exists → primary domain (internal-tui) per parent guidance |
| ARF-007 — Entity-level validation stays track-only | adapters-spotify | ADDED | Verbatim delta |
| ARF-008 — The gate admits no URI scheme other than spotify | internal-tui | ADDED | Verbatim delta |
| ARF-009 — Hermetic full-suite green | github-workflows | ADDED | Repo-level hermetic test gate; no repo-level canonical domain → github-workflows (complements ARF-004, which mandates `-short` in CI) |
| ARF-010 — yt-dlp option terminator before the URL | adapters-searcher | ADDED | Full block (covers `searchArgs` + downloader `buildArgs`) placed in adapters-searcher (primary yt-dlp argv builder); downloader canonical carries the mirrored `TestBuildArgs_OptionTerminatorBeforeURL` test spec + change-history note |

**REMOVED requirements:** none in this change.

**Byte-coherence:** verified programmatically — all 10 ARF requirement bodies in canonical files are identical to the change-spec deltas (heading rename `### ADDED/MODIFIED Requirement:` → `### Requirement:` only; canonical `---` section separators preserved). No duplicate requirement headings in `openspec/specs/`.

## Destructive Merge Guard

- **ARF-002 MODIFIED replacement:** replaced the canonical `Input screen behavior` block (~35 lines) with the full delta block (~60 lines). Per the MODIFIED merge rule this is a full-block replacement, not a partial patch; verified no scenario silently dropped. The parent prompt explicitly instructs this merge (and names ARF-002 as a MODIFIED delta to merge), recording approval for the archive-time sync including this replacement.
- **No REMOVED requirements**, no other destructive operations. `openspec/config.yaml` untouched (documented pre-existing baseline correction, part of this change's manifest).

## Active Same-Domain Change Warnings

| Change | Domain overlap | State | Impact on this archive |
| --- | --- | --- | --- |
| `ytmusic-search` | `internal-tui` (ADDED requirements) | Active — apply complete, verify pending | Informational: future `internal-tui` syncs must merge in sequence |
| `spotify-adapters` | `adapters-spotify` | Active — apply phase | Informational: future `adapters-spotify` syncs must merge in sequence |
| `audio-quality` | `internal-tui`, `internal-config`, `core-service`, `cmd-entrypoint`, `project-docs`, `adapters-downloader` | CLOSED (archive-report exists) but folder never moved to archive; canonical merge partial (adapters-downloader only) | Pre-existing condition, out of scope: canonical `internal-tui` contains **no** AQ content (verified) — nothing to clobber; the audio-quality canonical gap is tracked as a repo-level follow-up, not fixed here |

## Task Completion

All **12 implementation-owned** checkboxes in `tasks/tasks.md` are `[x]` (re-read at the Final Task Completion Gate before the sync — no `- [ ]` implementation tasks remain).

Deferred parent-owned rows (2, both reconciled at native lifecycle boundaries, remain unchecked by design):

- "Start or reuse bounded review…" → executed: bounded review round 3 approved, lineage `review-afebd24a0eb67c30` (parent prompt).
- "Verify-phase launch…" → executed: `verify-report.md` written, PASS 10/10.

No stale-checkbox reconciliation needed; no non-critical partial archive needed.

## Structured Status Consumed

| Field | Value |
| --- | --- |
| schemaName | spec-driven |
| changeName | adapter-routing-fix |
| artifactStore | openspec (file-based; status contract consumed from `~/.pi/agent/gentle-ai/support/sdd-status-contract.md`) |
| taskProgress | 12/12 implementation complete; unchecked: [] |
| deferredParentActions | 2/2 reconciled (review round 3 approved; verify executed) |
| applyState | all_done |
| dependencies | apply all_done · verify ready → done · sync done (archive-time fallback) · archive ready → done |
| actionContext | mode: repo-local · workspaceRoot: `/home/juan-arch/Projects/music-dowloader` · edits confined to `openspec/` within the workspace · warnings: same-domain active changes above |
| nextRecommended | commit (parent-owned; staged diff + pre-commit review receipt validation) |

## Delivery Notes

- **Follow-ups pending user decision (FU-01/FU-02, recorded in apply-progress round 3):** FU-01 — pin `actions/checkout@v4`/`actions/setup-go@v5` to commit SHAs + add `permissions: contents: read`; FU-02 — refresh design §2.1 code sample to match shipped case-insensitive `IsSpotifyURL`. Both WARNING-class, non-blocking, delivered as post-PR follow-ups per user decision.
- Pre-existing repo note: `audio-quality` canonical sync gap (see warnings table) — consider a follow-up to complete its canonical merge and move its folder to archive.

## Archived Path

```
openspec/changes/adapter-routing-fix/  →  openspec/changes/archive/2026-08-01-adapter-routing-fix/
```

## Memory Observation IDs

N/A — archive ran in openspec file mode per the parent's designation; no Engram save performed, no observation IDs claimed.
