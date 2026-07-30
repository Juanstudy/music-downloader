# Entrypoint — cmd/music-dl Specification

## Purpose

Define the application entrypoint that wires all layers together via explicit dependency injection, runs preflight checks, and starts the Bubble Tea TUI program.

## Requirements

### Requirement: main function is a thin composer

The `main()` function in `cmd/music-dl/main.go` MUST compose adapters into the service layer, inject the service into the TUI, and start the program. It MUST NOT contain business logic.

```go
func main()
```

#### Scenario: main constructs adapters, service, and TUI in order

- GIVEN the `main()` function
- WHEN executed
- THEN it MUST construct adapters in this order:
  1. `preflight.NewChecker()` — preflight dependency check
  2. `filesystem.NewOutput(filesystem.DefaultMusicDir())` — output directory manager
  3. `searcher.NewSearcher()` — YouTube search adapter
  4. `downloader.NewDownloader()` — audio download adapter
- THEN it MUST construct the service: `service.NewOrchestrator(searcher, downloader)`
- THEN it MUST construct the TUI: `tui.NewModel(orchestrator)`
- THEN it MUST start the Bubble Tea program

### Requirement: Preflight runs synchronously before TUI starts

The system MUST run the preflight checker before initializing the TUI program. If preflight fails, the program MUST print errors and exit with code 1 without starting the TUI.

#### Scenario: Preflight passes → TUI starts

- GIVEN a system where `yt-dlp` and `ffmpeg` are on `$PATH`
- WHEN `main()` executes preflight
- THEN the `PreflightChecker.Check(ctx)` MUST return empty errors
- AND the TUI program MUST start

#### Scenario: Preflight fails → errors printed, exit 1

- GIVEN a system where `yt-dlp` or `ffmpeg` is missing from `$PATH`
- WHEN `main()` executes preflight
- THEN errors MUST be printed to stderr (one per missing binary)
- AND the program MUST exit with code 1
- AND the TUI MUST NOT start

### Requirement: Output directory is ensured before TUI starts

The system MUST ensure the output directory exists before starting the TUI.

#### Scenario: Output directory is created on startup

- GIVEN the `main()` function
- WHEN executing
- THEN it MUST call `output.Ensure(ctx)` to create the output directory
- AND if `Ensure` fails, the error MUST be reported and the program SHOULD exit with code 1

### Requirement: Bubble Tea program starts with the composed Model

```go
func main() {
    // ... DI wiring ...
    // ... preflight ...
    // ... output ensure ...

    p := tea.NewProgram(model, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

#### Scenario: Program uses alternate screen buffer

- GIVEN the `tea.NewProgram()` call
- WHEN inspecting options
- THEN `tea.WithAltScreen()` MUST be passed

#### Scenario: Program Run error is reported

- GIVEN the `tea.Program.Run()` call returns an error
- WHEN writing the error
- THEN it MUST be written to stderr prefixed with `"Error: "`
- AND the program MUST exit with code 1

### Requirement: Import graph is acyclic

#### Scenario: cmd/music-dl imports only allowed packages

- GIVEN the `cmd/music-dl/main.go` file
- WHEN inspecting imports
- THEN it MUST import:
  - `internal/tui`
  - `internal/core/service`
  - `internal/core/ports` (if needed for type references)
  - `internal/adapters/searcher`
  - `internal/adapters/downloader`
  - `internal/adapters/preflight`
  - `internal/adapters/filesystem`
- AND it MUST NOT import `core/domain` directly (unless needed for preflight)

### Requirement: No unused adapter packages

All four adapters (searcher, downloader, preflight, filesystem) MUST be imported in `main.go` even though only searcher and downloader are passed to the orchestrator. This ensures the `go build` compiles all adapter packages.

#### Scenario: All adapter packages are imported

- GIVEN `cmd/music-dl/main.go`
- WHEN checking imports
- THEN all four adapter packages MUST be imported

---

## Test Specifications

The entrypoint is intentionally minimal and not unit-tested directly (would require a real TTY). Integration/compilation verification is done via:

### Compilation test

| Case | Command | Expected |
|------|---------|----------|
| Build succeeds | `go build ./cmd/music-dl/` | Binary produced, exit code 0 |
| Vet passes | `go vet ./cmd/music-dl/` | No warnings |

### Preflight error output (manual/integration)

| Case | Setup | Expected |
|------|-------|----------|
| Missing binary | Temporarily remove yt-dlp from PATH | Error message printed, exit 1 |
| Both missing | Remove both yt-dlp and ffmpeg from PATH | Two error messages printed, exit 1 |
