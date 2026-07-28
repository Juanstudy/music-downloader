# Adapters — Preflight Checker Specification

## Purpose

Implement the `PreflightChecker` port by checking that external binaries (`yt-dlp`, `ffmpeg`) are available on `$PATH` before the TUI starts.

## Requirements

### Requirement: Checker validates required binaries via exec.LookPath

The system MUST provide a `PreflightChecker` implementation that uses `exec.LookPath` to validate binary availability.

```go
type Checker struct {
    Binaries []string  // configurable list, defaults to ["yt-dlp", "ffmpeg"]
}

func NewChecker() *Checker
func (c *Checker) Check(ctx context.Context) []ports.PreflightError
```

#### Scenario: Check returns empty slice when both binaries exist

- GIVEN a `Checker` with default binaries `["yt-dlp", "ffmpeg"]`
- WHEN both binaries are on `$PATH`
- THEN `Check(ctx)` MUST return an empty (nil) slice

#### Scenario: Check returns error for each missing binary

- GIVEN a `Checker` with default binaries
- WHEN neither binary is on `$PATH`
- THEN `Check(ctx)` MUST return a slice with 2 `PreflightError` items
- AND one MUST have `Binary: "yt-dlp"`
- AND the other MUST have `Binary: "ffmpeg"`
- AND both MUST have a non-nil `Err`

#### Scenario: Check operates on configurable binary list

- GIVEN a `Checker` created via struct literal with custom `Binaries`
- WHEN `Check(ctx)` is called
- THEN it MUST check only the binaries in the configured list

#### Scenario: Check is not fail-fast

- GIVEN a `Checker` with binaries `["yt-dlp", "ffmpeg"]`
- WHEN `yt-dlp` is missing but `ffmpeg` is present
- THEN `Check(ctx)` MUST return exactly 1 `PreflightError` for `yt-dlp`
- AND it MUST NOT short-circuit after finding the first missing binary

#### Scenario: Check respects context cancellation

- GIVEN a `Checker`
- WHEN the context is cancelled before `Check` completes
- THEN `Check` MUST respect context cancellation and return early

---

## Test Specifications

### Test: Checker (unit with fake PATH)

**File:** `internal/adapters/preflight/checker_test.go`

**Pattern:** Manipulate `$PATH` to point to `t.TempDir()` with expected binaries.

| Case | Setup | Expected |
| ------ | ------- | ---------- |
| Both binaries present | Create fake `yt-dlp` and `ffmpeg` scripts in TempDir, set PATH | Empty slice |
| Both binaries missing | Set PATH to empty TempDir | 2 errors (yt-dlp, ffmpeg) |
| One binary missing | Create only `yt-dlp` in TempDir | 1 error (ffmpeg) |
| Empty binary list | `Checker{Binaries: []}` | Empty slice |
| Context cancelled | Pre-cancelled context | Returns immediately (error or empty slice) |
