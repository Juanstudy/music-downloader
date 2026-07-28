# Archive Report: skeleton-reboot

## Status

✅ **ARCHIVED** — All implementation tasks complete, build gates pass.

## Artifacts Read

| Artifact | Path / Topic Key | Present |
| ---------- | ------------------ | --------- |
| Proposal | `openspec/changes/skeleton-reboot/proposal.md` | ✅ |
| Spec (consolidated) | `openspec/changes/skeleton-reboot/spec.md` | ✅ |
| Design | `openspec/changes/skeleton-reboot/design.md` | ✅ |
| Tasks | `openspec/changes/skeleton-reboot/tasks.md` | ✅ |
| Apply Progress | `openspec/changes/skeleton-reboot/apply-progress.md` | ✅ |
| Verify Report | — (not persisted; user-provided evidence confirmed) | ⚠️ See notes |

## Domains Synced (New Canonical Specs)

All 9 domain specs copied to canonical `openspec/specs/{domain}/spec.md` as NEW specs (no pre-existing canonical):

| Domain | Status |
| -------- | -------- |
| `core-domain` | ✅ NEW |
| `core-ports` | ✅ NEW |
| `core-service` | ✅ NEW |
| `adapters-searcher` | ✅ NEW |
| `adapters-downloader` | ✅ NEW |
| `adapters-preflight` | ✅ NEW |
| `adapters-filesystem` | ✅ NEW |
| `cmd-entrypoint` | ✅ NEW |
| `internal-tui` | ✅ NEW |

Consolidated spec also preserved at `openspec/specs/consolidated/spec.md`.

## Implementation Task Checkboxes

All implementation-owned tasks (`<!-- sdd-owner: implementation -->` or unmarked) are **checked [x]**:

- Task 1.1 — Write domain type tests (RED) ✅
- Task 1.2 — Implement domain types (GREEN) ✅
- Task 1.3 — Define port interfaces ✅
- Task 1.4 — Write port struct tests ✅
- Task 2.1 — Write orchestrator tests with mocks (RED) ✅
- Task 2.2 — Implement Orchestrator to pass tests (GREEN) ✅
- Task 3.1 — Write JSON parse tests (RED) ✅
- Task 3.2 — Implement ParseLine to pass tests (GREEN) ✅
- Task 3.3 — Write preflight checker tests (RED) ✅
- Task 3.4 — Implement preflight checker (GREEN) ✅
- Task 3.5 — Write filesystem output tests (RED) ✅
- Task 3.6 — Implement filesystem output (GREEN) ✅
- Task 4.1 — Implement yt-dlp searcher adapter ✅
- Task 4.2 — Write yt-dlp searcher integration test ✅
- Task 4.3 — Implement yt-dlp downloader adapter ✅
- Task 4.4 — Write yt-dlp downloader integration test ✅
- Task 5.1 — Create TUI model, messages, styles, and keys ✅
- Task 5.2 — Write TUI state transition tests (RED) ✅
- Task 5.3 — Create stub update.go (compiles, tests fail) ✅
- Task 6.1 — Implement full Update() ✅
- Task 6.2 — Implement View() for all 5 screens ✅
- Task 6.3 — Implement main.go entrypoint ✅
- Task 6.4 — Run full project build verification ✅
- Task 6.5 — Verify with short tests ✅
- Post-merge `go build ./cmd/music-dl/` ✅
- Post-merge `go vet ./...` ✅
- Post-merge `go test -short ./internal/...` ✅

**Remaining unchecked tasks are all parent-owned** (`<!-- sdd-owner: parent -->`): bounded reviews, merge approvals, and squash-merge. These are lifecycle actions outside the implementation scope and do not block archive.

## Build Verification (confirmed at archive time)

| Gate | Result |
| ------ | -------- |
| `go build ./...` | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go test -short ./...` | ✅ ALL PASS (all packages) |

- **27 Go source files** (includes 11 test files)
- **~70 tests** across 8 packages
- Packages: `core/domain`, `core/ports`, `core/service`, `adapters/searcher`, `adapters/downloader`, `adapters/preflight`, `adapters/filesystem`, `internal/tui`

## Archived Path

```
openspec/changes/skeleton-reboot/  →  openspec/changes/archive/2026-07-27-skeleton-reboot/
```

## Active Same-Domain Change Warnings

None — no other active changes were detected.

## Destructive Merge Guard

Not applicable — all 9 domain specs were NEW in the canonical location (no existing specs to modify or remove). No destructive merge was performed.

## Deviations from Original Specification

The following intentional deviations were noted in apply-progress:

| Area | Deviation | Rationale |
| ------ | ----------- | ----------- |
| `DownloadTrack` | Does NOT set `StatusDownloading` before calling downloader | TUI manages visual downloading state independently |
| `isSupportedURL` / `DownloadTracks` | Not implemented in PR 2 scope | Deferred to PR 6 integration scope |
| URL validation | Empty-string only (not prefix-based YouTube validation) | Deferred to TUI layer for user feedback |
| `ParseLine` | Returns `StatusPending` (not `StatusResolved`) | Parser produces pending tracks; caller sets resolved status |
| Missing `webpage_url` | Returns error (not ID-based URL construction) | Safer to fail than guess URL structure |
| Model struct | Uses `textinput.Model` and `spinner.Model` widgets | Better UX than plain string fields per Bubble Tea patterns |
| Custom message names | `resolveFinishedMsg` / `trackDownloadedMsg` | Per delegated task naming convention |

## Engram Save Status

⚠️ Engram memory server was unavailable at archive time (`http://127.0.0.1:7437`). Topic key `sdd/skeleton-reboot/archive-report` could not be persisted to memory. All file-based artifacts are preserved under the archive path.

## Structured Status

| Field | Value |
| ------- | ------- |
| Status | ✅ ARCHIVED |
| Artifact Store | hybrid (openspec + engram) |
| Skill Resolution | `paths-injected` |
| Next Recommended | Parent-owned: review + merge actions on `feature/skeleton-reboot` branch |
