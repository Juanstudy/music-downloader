# Adapters — Filesystem Output Specification

## Purpose

Manage the output directory where downloaded music files are stored. Provides directory creation and path resolution for the download operation.

## Requirements

### Requirement: OutputManager creates the output directory if missing

The system MUST provide an `Output` type that manages the download output directory.

```go
type Output struct {
    BasePath string
}

func NewOutput(basePath string) *Output
```

#### Scenario: NewOutput stores base path

- GIVEN `NewOutput("~/Music/music-dl")`
- WHEN accessing `Output.BasePath`
- THEN it MUST contain the provided path
- AND the path MUST be expanded (tilde expanded to user home directory)

### Requirement: Ensure method creates the directory tree

```go
func (o *Output) Ensure(ctx context.Context) error
```

#### Scenario: Ensure creates the output directory when it doesn't exist

- GIVEN an `Output` with a non-existent path
- WHEN `Ensure(ctx)` is called
- THEN the directory MUST be created (including parent directories)
- AND no error MUST be returned

#### Scenario: Ensure succeeds when directory already exists

- GIVEN an `Output` with an existing directory path
- WHEN `Ensure(ctx)` is called
- THEN no error MUST be returned

#### Scenario: Ensure returns error on permission failure

- GIVEN an `Output` with a path that cannot be created (e.g., permission denied)
- WHEN `Ensure(ctx)` is called
- THEN a non-nil error MUST be returned

### Requirement: Output returns default path

The system MUST provide a sensible default output path.

```go
func DefaultMusicDir() string
```

#### Scenario: DefaultMusicDir returns ~/Music/music-dl

- GIVEN the `DefaultMusicDir()` function
- WHEN called
- THEN it MUST return the path `"~/Music/music-dl"` (with tilde)
- AND the path MUST be expandable to an absolute path via `os.UserHomeDir()` + `/Music/music-dl`

### Requirement: FullPath method resolves the download target directory

```go
func (o *Output) FullPath() (string, error)
```

#### Scenario: FullPath returns expanded absolute path

- GIVEN an `Output` with `BasePath: "~/Music/music-dl"`
- WHEN `FullPath()` is called
- THEN it MUST return the tilde-expanded absolute path (e.g., `/home/user/Music/music-dl`)
- AND no error on success

#### Scenario: FullPath returns error when home directory cannot be determined

- GIVEN an `Output` with a tilde path
- WHEN `$HOME` is not set
- THEN `FullPath()` MUST return an error

---

## Test Specifications

### Test: Output directory management

**File:** `internal/adapters/filesystem/output_test.go`

**Pattern:** Use `t.TempDir()` for all filesystem operations.

| Case | Setup | Expected |
| ------ | ------- | ---------- |
| `Ensure` creates directory | `Output{BasePath: filepath.Join(t.TempDir(), "sub", "dir")}` | Directory exists after `Ensure()`, no error |
| `Ensure` on existing directory | Create dir first, then `Ensure()` | No error |
| `Ensure` with permission error | (Optional, platform-dependent) | Error returned |
| `FullPath` with absolute path | `Output{BasePath: t.TempDir()}` | Returns same path, no error |
| `DefaultMusicDir` returns expected path | Call `DefaultMusicDir()` | Returns `"~/Music/music-dl"` |
