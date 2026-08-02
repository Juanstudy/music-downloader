# GitHub Workflows Specification

> **Created by:** `adapter-routing-fix` (2026-08-01) — minimal hermetic CI gate (ARF-004, ARF-005), hermetic full-suite green (ARF-009).

## Purpose

Provide a minimal, hermetic CI gate (vet + build + `go test -short ./...` on every push and pull request) so regressions like issue #19 fail the build instead of shipping silently. Deliberately minimal per the locked scope decision: no linter, no coverage gate, no caching, no matrix, no path filters.

## Requirements

### Requirement: ARF-004 — CI workflow runs vet, build, and hermetic tests on push and pull_request

The repository MUST include `.github/workflows/ci.yml` defining a workflow that triggers on `push` and `pull_request` (no path filters) and runs one job on `ubuntu-latest` which checks out the repository with `actions/checkout@v4`, installs Go with `actions/setup-go@v5` using `go-version-file: go.mod`, and then runs, in order: `go vet ./...`, `go build ./...`, `go test -short ./...`. The `-short` flag MUST be present so network-gated yt-dlp/Spotify integration tests skip and the workflow stays hermetic.

#### Scenario: workflow file exists at the expected path

- GIVEN the repository root
- WHEN checking for `.github/workflows/ci.yml`
- THEN the file MUST exist

#### Scenario: triggers cover push and pull_request

- GIVEN the workflow's `on:` key
- WHEN inspecting it
- THEN it MUST include `push`
- AND it MUST include `pull_request`
- AND it MUST NOT restrict either trigger with path filters

#### Scenario: job runs the three commands in order

- GIVEN the workflow's job steps
- WHEN inspecting them in order
- THEN step 1 MUST be `actions/checkout@v4`
- AND step 2 MUST be `actions/setup-go@v5` with `go-version-file: go.mod`
- AND the following steps MUST run `go vet ./...`, then `go build ./...`, then `go test -short ./...`

#### Scenario: tests are hermetic under -short

- GIVEN a machine with no network access (e.g. CI sandbox)
- WHEN the workflow's test step runs `go test -short ./...`
- THEN the network-gated integration tests MUST skip
- AND the step MUST pass without live network calls

### Requirement: ARF-005 — CI stays minimal

The workflow MUST NOT include a linter step, a coverage gate, caching, a build matrix, path filters, or any step beyond checkout, Go setup, vet, build, and the short test run.

#### Scenario: no extras in the workflow

- GIVEN the workflow file content
- WHEN inspecting it for linters, coverage, caching, matrix, and path filters
- THEN it MUST NOT reference golangci-lint (or any linter), coverage commands or thresholds, cache actions, `matrix` keys, or `paths:`/`paths-ignore:` filters

### Requirement: ARF-009 — Hermetic full-suite green

`go test -short ./...` MUST pass (exit 0) with the change applied, on a machine without network access; network-gated integration tests MUST skip under `-short`. This is the local test runner declared in `openspec/config.yaml` and the exact command CI runs.

#### Scenario: short test suite passes locally

- GIVEN the change applied and no network access
- WHEN running `go test -short ./...`
- THEN it MUST exit 0
- AND network-gated integration tests (e.g. the yt-dlp integration tests) MUST be skipped, not run

---

## Test Specifications

### Test: CI workflow (file inspection)

| Case | Check | Expected |
| ------ | ----- | --------- |
| File exists | `.github/workflows/ci.yml` | present |
| Triggers | `on:` key | `push` and `pull_request`, no path filters |
| Runner | job `runs-on` | `ubuntu-latest` |
| Setup | steps | `actions/checkout@v4`; `actions/setup-go@v5` with `go-version-file: go.mod` |
| Commands | steps in order | `go vet ./...`, `go build ./...`, `go test -short ./...` |
| Minimality | workflow content | no linter, no coverage gate, no caching, no matrix, no path filters |
