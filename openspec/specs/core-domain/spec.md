# Core Domain Specification

## Purpose

Define the pure domain types — `Media`, `Status`, `Error`, `ErrorCode` — that represent the problem domain of downloading music from URLs. These types carry zero behavior and zero external dependencies; they are used by every layer in the hexagonal architecture.

## Requirements

### Requirement: Domain types are in a single package with no external imports

The `core/domain` package MUST contain all shared domain types. It MUST import only the Go standard library (`"time"`). It MUST NOT import `core/ports`, `core/service`, or any adapter or TUI package.

#### Scenario: Package compiles with only stdlib imports

- GIVEN a file `internal/core/domain/media.go`
- WHEN compiled with `go build ./internal/core/domain/`
- THEN it MUST succeed
- AND the only imports MUST be from the Go standard library

### Requirement: Status enum

The system MUST define a `Status` type representing the lifecycle state of a media item or download operation.

```go
type Status int

const (
    StatusPending      Status = iota
    StatusResolving
    StatusResolved
    StatusDownloading
    StatusDone
    StatusFailed
)
```

#### Scenario: Status constants are sequential and typed

- GIVEN the `Status` type
- WHEN each constant is evaluated
- THEN `StatusPending` MUST be 0
- AND `StatusResolving` MUST be 1
- AND `StatusResolved` MUST be 2
- AND `StatusDownloading` MUST be 3
- AND `StatusDone` MUST be 4
- AND `StatusFailed` MUST be 5
- AND each constant MUST be of type `Status`

### Requirement: Media struct

The system MUST define a `Media` struct that represents a downloadable track.

```go
type Media struct {
    URL        string
    Title      string
    Artist     string
    Duration   time.Duration
    Source     string
    Status     Status
    Error      string
    OutputPath string
}
```

#### Scenario: Media struct zero-value is usable

- GIVEN a zero-value `Media{}`
- WHEN accessing each field
- THEN `URL` MUST be `""`
- AND `Title` MUST be `""`
- AND `Artist` MUST be `""`
- AND `Duration` MUST be `0`
- AND `Source` MUST be `""`
- AND `Status` MUST be `StatusPending`
- AND `Error` MUST be `""`
- AND `OutputPath` MUST be `""`

#### Scenario: Media fields are populated by struct literal

- GIVEN a `Media` constructed with field values
- WHEN each field is accessed
- THEN it MUST return the value that was set

### Requirement: ErrorCode enum and Error struct

The system MUST define typed error codes and a domain error struct for structured error propagation across layer boundaries.

```go
type ErrorCode int

const (
    ErrorGeneric        ErrorCode = iota
    ErrorNetwork
    ErrorInvalidURL
    ErrorBinaryNotFound
    ErrorTrackUnavailable
    ErrorAgeRestricted
    ErrorDiskFull
)

type Error struct {
    Code    ErrorCode
    Message string
    Track   string
}
```

#### Scenario: ErrorCode constants are sequential and typed

- GIVEN the `ErrorCode` type
- WHEN each constant is evaluated
- THEN `ErrorGeneric` MUST be 0
- AND `ErrorNetwork` MUST be 1
- AND `ErrorInvalidURL` MUST be 2
- AND `ErrorBinaryNotFound` MUST be 3
- AND `ErrorTrackUnavailable` MUST be 4
- AND `ErrorAgeRestricted` MUST be 5
- AND `ErrorDiskFull` MUST be 6
- AND each MUST be of type `ErrorCode`

#### Scenario: domain.Error implements the error interface

- GIVEN a `domain.Error{Code: ErrorNetwork, Message: "network timeout", Track: "https://..."}`
- WHEN calling `.Error()`
- THEN it MUST return a non-empty string containing the error message

#### Scenario: domain.Error with empty Track field

- GIVEN a `domain.Error{Code: ErrorBinaryNotFound, Message: "yt-dlp not found", Track: ""}`
- WHEN used in preflight context
- THEN the `Track` field MUST be empty
- AND the error is still valid (non-preflight errors may also have empty Track)

### Requirement: No exported constructors or helper methods

The `core/domain` package MUST NOT export constructors, factory functions, or helper methods. Types are created directly via struct literals and type conversions.

#### Scenario: No exported functions exist

- GIVEN the `core/domain` package
- WHEN checking exported symbols with `go doc ./internal/core/domain/`
- THEN the only exported symbols MUST be `Status`, `ErrorCode`, `Media`, `Error`, and their constants
- AND there MUST be zero exported functions

---

## Test Specifications

### Test: Status constants

**File:** `internal/core/domain/media_test.go`

| Case | Assertion |
|------|-----------|
| `StatusPending` is 0, `StatusResolving` is 1, ..., `StatusFailed` is 5 | Each constant has correct sequential value |
| All constants are type `Status` | Compile-time type check via typed const |

### Test: Media struct

| Case | Assertion |
|------|-----------|
| Zero-value `Media{}` has expected defaults | All fields are zero/empty |
| Media with all fields set returns correct values via struct literal | Each field returns set value |

### Test: ErrorCode constants

| Case | Assertion |
|------|-----------|
| `ErrorGeneric` is 0, `ErrorNetwork` is 1, ..., `ErrorDiskFull` is 6 | Each constant has correct sequential value |
| All constants are type `ErrorCode` | Compile-time type check via typed const |

### Test: domain.Error

| Case | Assertion |
|------|-----------|
| Error with Code and Message returns Message via `Error()` | `.Error()` returns message string |
| Error with Track set includes track info | Track field is accessible and correct |
