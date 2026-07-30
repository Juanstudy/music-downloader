# Internal TUI Specification

## Purpose

Implement the 5-screen Bubble Tea terminal UI with deterministic state transitions, keyboard navigation, and async communication with the core service layer. The TUI is separated from adapters and only depends on `core/service` and `core/domain`.

## Requirements

### Requirement: Package layout follows Bubble Tea screen patterns

The `internal/tui` package MUST be organized into focused files following the gentleman-bubbletea skill patterns.

| File | Responsibility |
| ------ | --------------- |
| `model.go` | `Screen` enum, `Model` struct, `Init()` |
| `update.go` | `Update()` loop, key routing to screen handlers, async command dispatch |
| `view.go` | `View()` rendering for all 5 screens |
| `styles.go` | Semantic color slots using Lipgloss |
| `keys.go` | Keybinding constants and help text |

#### Scenario: Package compiles with correct imports

- GIVEN the package structure
- WHEN compiled with `go build ./internal/tui/`
- THEN it MUST succeed
- AND the package MUST import `core/service` and `core/domain`
- AND it MUST NOT import `adapters/*`, `os/exec`, or any yt-dlp-related packages

### Requirement: Screen enum with 5 screens

```go
type Screen int

const (
    ScreenInput Screen = iota
    ScreenResolving
    ScreenPlaylist
    ScreenDownloading
    ScreenDone
)
```

#### Scenario: Screen constants define the 5 states

- GIVEN the `Screen` type
- WHEN each constant is evaluated
- THEN `ScreenInput` MUST be 0
- AND `ScreenResolving` MUST be 1
- AND `ScreenPlaylist` MUST be 2
- AND `ScreenDownloading` MUST be 3
- AND `ScreenDone` MUST be 4

### Requirement: Model struct holds all TUI state

```go
type Model struct {
    // Orchestrator (injected dependency)
    orchestrator *service.Orchestrator

    // Navigation
    Screen     Screen
    PrevScreen Screen
    Width      int
    Height     int

    // Input screen
    InputText  string
    InputError string

    // Resolving screen
    ResolvingError string

    // Playlist screen
    Tracks     []domain.Media
    Cursor     int
    Scroll     int

    // Downloading screen
    CurrentDownload  int
    DownloadComplete bool

    // Done screen
    TotalTracks   int
    Succeeded     int
    Failed        int
    FailedTracks  []domain.Media
}
```

#### Scenario: Model zero-value shows Input screen

- GIVEN a zero-value `Model{}`
- WHEN checking initial state
- THEN `Screen` MUST be `ScreenInput`
- AND `InputText` MUST be `""`
- AND `InputError` MUST be `""`
- AND `PrevScreen` MUST be `ScreenInput`

### Requirement: Constructor wires orchestrator dependency

```go
func NewModel(orchestrator *service.Orchestrator) Model
```

#### Scenario: NewModel creates Model with Input screen

- GIVEN an `*Orchestrator` instance
- WHEN `NewModel(orchestrator)` is called
- THEN it MUST return a `Model`
- AND `Model.Screen` MUST be `ScreenInput`
- AND `Model.orchestrator` MUST be set to the provided instance

### Requirement: Init returns initial Cmd

```go
func (m Model) Init() tea.Cmd
```

#### Scenario: Init returns nil (no startup command needed)

- GIVEN a `Model`
- WHEN `Init()` is called
- THEN it MUST return nil (no initial command)

### Requirement: Update dispatches messages

All message handling MUST go through `Update()` with a type switch.

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
```

#### Scenario: Update handles tea.KeyMsg, routes by current screen

- GIVEN a `Model`
- WHEN `Update()` receives a `tea.KeyMsg`
- THEN it MUST route to the handler for the current `Screen`
- AND return `(Model, tea.Cmd)`

#### Scenario: Update handles tea.WindowSizeMsg

- GIVEN a `Model`
- WHEN `Update()` receives a `tea.WindowSizeMsg`
- THEN `m.Width` and `m.Height` MUST be updated
- AND the method MUST return `(m, nil)`

#### Scenario: Update handles custom async messages

The system MUST define custom message types for async TUI operations:

```go
type resolveResultMsg struct {
    tracks []domain.Media
    err    error
}

type downloadProgressMsg struct {
    index int
    media domain.Media
    err   error
}

type downloadCompleteMsg struct{}
```

- GIVEN a `Model`
- WHEN `Update()` receives a `resolveResultMsg`
- THEN on success, `m.Tracks` MUST be set AND `m.Screen` MUST transition to `ScreenPlaylist`
- AND on error, `m.ResolvingError` MUST be set AND `m.Screen` MUST return to `ScreenInput`
- WHEN `Update()` receives a `downloadProgressMsg`
- THEN the corresponding track status MUST be updated
- AND `m.CurrentDownload` MUST advance
- WHEN `Update()` receives a `downloadCompleteMsg`
- THEN `m.DownloadComplete` MUST be true
- AND `m.Screen` MUST transition to `ScreenDone`

### Requirement: Input screen behavior

#### Scenario: Enter key with valid URL triggers resolving

- GIVEN a `Model` on `ScreenInput` with `InputText` containing a YouTube URL
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST transition to `ScreenResolving`
- AND a `tea.Cmd` MUST be returned that calls `orchestrator.ResolveTrack(ctx, url)` and sends the result as a `resolveResultMsg`

#### Scenario: Enter key with empty URL does nothing

- GIVEN a `Model` on `ScreenInput` with `InputText == ""`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST remain `ScreenInput`
- AND `m.InputError` MUST indicate the URL is empty

#### Scenario: Enter key with non-YouTube URL shows inline error

- GIVEN a `Model` on `ScreenInput` with a non-YouTube URL
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST remain `ScreenInput`
- AND `m.InputError` MUST contain a message about unsupported URL

#### Scenario: Typing characters update InputText

- GIVEN a `Model` on `ScreenInput`
- WHEN `Update()` receives a `tea.KeyMsg` with a printable character
- THEN the character MUST be appended to `m.InputText`

#### Scenario: Backspace removes last character

- GIVEN a `Model` on `ScreenInput` with non-empty `InputText`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyBackspace}`
- THEN the last character MUST be removed from `m.InputText`

#### Scenario: Esc on Input screen quits

- GIVEN a `Model` on `ScreenInput`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEsc}`
- THEN a `tea.Quit` command MUST be returned

#### Scenario: Ctrl+C quits from any screen

- GIVEN a `Model` on any screen
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyCtrlC}`
- THEN a `tea.Quit` command MUST be returned

### Requirement: Resolving screen behavior

#### Scenario: Resolving shows spinner state

- GIVEN a `Model` on `ScreenResolving`
- WHEN `View()` is called
- THEN the rendered view MUST include a spinner or message indicating resolution is in progress

#### Scenario: Esc on Resolving screen returns to Input

- GIVEN a `Model` on `ScreenResolving`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEsc}`
- THEN `m.Screen` MUST transition to `ScreenInput`
- AND `m.Cursor` MUST be reset to 0

#### Scenario: Successful resolution transitions to Playlist

- GIVEN a `Model` on `ScreenResolving`
- WHEN `Update()` receives a `resolveResultMsg` with tracks
- THEN `m.Screen` MUST be `ScreenPlaylist`
- AND `m.Tracks` MUST contain the resolved tracks
- AND `m.Cursor` MUST be 0

#### Scenario: Failed resolution shows error and returns to Input

- GIVEN a `Model` on `ScreenResolving`
- WHEN `Update()` receives a `resolveResultMsg` with an error
- THEN `m.Screen` MUST be `ScreenInput`
- AND `m.InputError` MUST contain the error message

### Requirement: Playlist screen behavior

#### Scenario: Up/Down (j/k) navigate track list

- GIVEN a `Model` on `ScreenPlaylist` with tracks
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyUp}` or `"k"`
- THEN `m.Cursor` MUST decrement (but not below 0)
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyDown}` or `"j"`
- THEN `m.Cursor` MUST increment (but not above len(tracks)-1)

#### Scenario: Space toggles track selection

- GIVEN a `Model` on `ScreenPlaylist` with tracks
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeySpace}`
- THEN the currently selected track's `Status` MUST toggle between `StatusPending` and `StatusResolved` (selected/deselected)

#### Scenario: 'a' selects all, 'n' selects none

- GIVEN a `Model` on `ScreenPlaylist` with tracks
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}`
- THEN all tracks MUST have `Status == StatusResolved` (selected)
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}`
- THEN all tracks MUST have `Status == StatusPending` (deselected)

#### Scenario: Enter starts download of selected tracks

- GIVEN a `Model` on `ScreenPlaylist` with at least one selected track
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST transition to `ScreenDownloading`
- AND `m.CurrentDownload` MUST be 0
- AND a `tea.Cmd` MUST be returned that starts the download sequence

#### Scenario: Enter with no selection shows hint

- GIVEN a `Model` on `ScreenPlaylist` with no selected tracks
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST remain `ScreenPlaylist`
- AND no download command MUST be issued

#### Scenario: Esc returns to Input

- GIVEN a `Model` on `ScreenPlaylist`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEsc}`
- THEN `m.Screen` MUST transition to `ScreenInput`
- AND `m.Cursor` MUST be reset to 0
- AND `m.Tracks` MUST be cleared

#### Scenario: Empty playlist shows message

- GIVEN a `Model` on `ScreenPlaylist` with 0 tracks
- WHEN `View()` is called
- THEN the rendered view MUST show "No tracks found" or equivalent message

### Requirement: Downloading screen behavior

#### Scenario: Downloading shows sequential progress per track

- GIVEN a `Model` on `ScreenDownloading`
- WHEN `View()` is called
- THEN the rendered view MUST show each track with its current status (pending, downloading, done, failed)

#### Scenario: Per-track completion is sent as downloadProgressMsg

- GIVEN a `Model` on `ScreenDownloading`
- WHEN a download completes
- THEN a `downloadProgressMsg` MUST be sent to `Update()`
- AND the corresponding track's `Status` MUST be updated

#### Scenario: All tracks complete transitions to Done

- GIVEN a `Model` on `ScreenDownloading`
- WHEN all tracks have been processed
- THEN `m.Screen` MUST transition to `ScreenDone`
- AND `m.TotalTracks`, `m.Succeeded`, `m.Failed`, and `m.FailedTracks` MUST be populated

#### Scenario: Esc during download shows warning (or is disabled)

- GIVEN a `Model` on `ScreenDownloading` with active downloads
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEsc}`
- THEN the behavior MUST be either: show a "quit anyway?" confirmation, or silently ignore Esc during active downloads
- AND active downloads MUST NOT be aborted without confirmation

#### Scenario: Esc before downloads start returns to Playlist

- GIVEN a `Model` on `ScreenDownloading` with `CurrentDownload == 0` and no downloads started
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEsc}`
- THEN `m.Screen` MUST transition to `ScreenPlaylist`

### Requirement: Done screen behavior

#### Scenario: Done screen shows summary

- GIVEN a `Model` on `ScreenDone`
- WHEN `View()` is called
- THEN the rendered view MUST show total/succeeded/failed counts
- AND if there are failures, each failed track + error message MUST be listed

#### Scenario: Enter on Done returns to Input

- GIVEN a `Model` on `ScreenDone`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyEnter}`
- THEN `m.Screen` MUST transition to `ScreenInput`
- AND all download-specific state MUST be reset (tracks cleared, counts reset)

#### Scenario: q on Done quits

- GIVEN a `Model` on `ScreenDone`
- WHEN `Update()` receives `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}`
- THEN a `tea.Quit` command MUST be returned

### Requirement: View rendering per screen

The `View()` method MUST render a distinct screen based on `m.Screen`.

#### Scenario: Input screen renders a text input area

- GIVEN a `Model` on `ScreenInput`
- WHEN `View()` is called
- THEN the output MUST include a prompt asking for a URL
- AND the current `InputText` MUST be displayed
- AND if `InputError` is set, it MUST be shown in a style indicating an error
- AND a footer MUST show available keybindings

#### Scenario: Resolving screen renders a spinner

- GIVEN a `Model` on `ScreenResolving`
- WHEN `View()` is called
- THEN the output MUST include a resolving/spinner message
- AND `ResolvingError` MUST NOT be displayed (it's only set when returning to Input)

#### Scenario: Playlist screen renders a track list

- GIVEN a `Model` on `ScreenPlaylist` with tracks
- WHEN `View()` is called
- THEN the output MUST include a list of tracks with selection indicators (checkboxes)
- AND the cursor position MUST be visible
- AND selected tracks MUST be visually distinct

#### Scenario: Downloading screen renders progress

- GIVEN a `Model` on `ScreenDownloading` with tracks
- WHEN `View()` is called
- THEN each track MUST show its current status (with icons like ✓/✗/spinner)
- AND the currently downloading track MUST be visually highlighted

#### Scenario: Done screen renders summary

- GIVEN a `Model` on `ScreenDone`
- WHEN `View()` is called
- THEN the total/succeeded/failed MUST be shown
- AND failed tracks MUST be listed with their errors

### Requirement: Styles use semantic color slots

```go
var (
    StyleNormal    lipgloss.Style  // default text
    StyleEmphasis  lipgloss.Style  // headers, cursor
    StyleMuted     lipgloss.Style  // secondary text
    StyleError     lipgloss.Style  // error messages
    StyleSuccess   lipgloss.Style  // success indicators
    StyleWarning   lipgloss.Style  // warnings
    StyleSelected  lipgloss.Style  // selected tracks
    StyleDimmed    lipgloss.Style  // deselected/dimmed items
)
```

#### Scenario: All styles are defined in styles.go

- GIVEN the `styles.go` file
- WHEN inspecting exported style variables
- THEN at minimum `StyleNormal`, `StyleEmphasis`, `StyleMuted`, `StyleError`, `StyleSuccess`, `StyleSelected` MUST be defined

#### Scenario: Styles use semantic naming (no hardcoded hex in views)

- GIVEN `view.go`
- WHEN searching for direct hex color codes (`#...`)
- THEN none SHOULD appear; all color references MUST be via Lipgloss style variables from `styles.go`

### Requirement: Keybindings are centralized

```go
var (
    KeyQuit    = "q"
    KeyBack    = "esc"
    KeySelect  = "enter"
    KeyToggle  = " "
    KeyAll     = "a"
    KeyNone    = "n"
    KeyUp      = "up"
    KeyDown    = "down"
    KeyUpAlt   = "k"
    KeyDownAlt = "j"
)
```

#### Scenario: Key constants are defined in keys.go

- GIVEN the `keys.go` file
- WHEN inspecting exported keybinding variables
- THEN they MUST define all keys used in the TUI

---

## Async Command Pattern

### Requirement: resolveURL returns a tea.Cmd that sends resolveResultMsg

```go
func (m Model) resolveURL() tea.Cmd
```

#### Scenario: resolveURL calls orchestrator.ResolveTrack asynchronously

- GIVEN a `Model` with a valid URL
- WHEN `resolveURL()` is invoked as a `tea.Cmd`
- THEN it MUST call `m.orchestrator.ResolveTrack(ctx, m.InputText)` in a goroutine
- AND send the result as a `resolveResultMsg` to the update loop

### Requirement: downloadTracks returns a tea.Cmd that sends download progress

```go
func (m Model) downloadTracks() tea.Cmd
```

#### Scenario: downloadTracks processes selected tracks sequentially

- GIVEN a `Model` with selected tracks
- WHEN `downloadTracks()` is invoked
- THEN it MUST iterate over selected tracks
- AND call `m.orchestrator.DownloadTracks()` (which returns a channel)
- AND send `downloadProgressMsg` for each completed track
- AND send `downloadCompleteMsg` when all tracks are done

---

## Test Specifications

### Test: Screen transitions

**File:** `internal/tui/update_test.go`

**Pattern:** Direct `Model.Update()` with `tea.KeyMsg` and custom messages. No real yt-dlp calls.

| Case | Msg | Initial State | Expected Final State |
| ------ | ----- | --------------- | --------------------- |
| Enter with URL → Resolving | `tea.KeyMsg{Type: tea.KeyEnter}` | `ScreenInput`, non-empty URL | `ScreenResolving` |
| Esc on Input → Quit | `tea.KeyMsg{Type: tea.KeyEsc}` | `ScreenInput` | `tea.Quit` command |
| Resolve success → Playlist | `resolveResultMsg{tracks: [...]}` | `ScreenResolving` | `ScreenPlaylist`, tracks set |
| Resolve failure → Input | `resolveResultMsg{err: ...}` | `ScreenResolving` | `ScreenInput`, InputError set |
| Enter on Playlist → Download | `tea.KeyMsg{Type: tea.KeyEnter}` | `ScreenPlaylist`, tracks selected | `ScreenDownloading` |
| All downloads done → Done | `downloadCompleteMsg{}` | `ScreenDownloading` | `ScreenDone` |
| Enter on Done → Input | `tea.KeyMsg{Type: tea.KeyEnter}` | `ScreenDone` | `ScreenInput`, tracks cleared |
| Empty URL Enter stays Input | `tea.KeyMsg{Type: tea.KeyEnter}` | `ScreenInput`, empty URL | `ScreenInput`, InputError set |
| Esc on Playlist → Input | `tea.KeyMsg{Type: tea.KeyEsc}` | `ScreenPlaylist` | `ScreenInput`, tracks cleared |
| Esc on Resolving → Input | `tea.KeyMsg{Type: tea.KeyEsc}` | `ScreenResolving` | `ScreenInput` |

### Test: Playlist selection

| Case | Msg | Initial | Expected |
| ------ | ----- | --------- | ---------- |
| Space toggles selection | `tea.KeyMsg{Type: tea.KeySpace}` | Track at cursor, `StatusPending` | Track `StatusResolved` |
| 'a' selects all | `tea.KeyMsg{Runes: ['a']}` | All pending | All resolved |
| 'n' selects none | `tea.KeyMsg{Runes: ['n']}` | All resolved | All pending |
| Enter with no selection | `tea.KeyMsg{Type: tea.KeyEnter}` | No selected tracks | Stay on Playlist |

### Test: Window resize

| Case | Msg | Expected |
|------|-----|----------|
| Resize updates dimensions | `tea.WindowSizeMsg{Width: 100, Height: 40}` | `m.Width == 100`, `m.Height == 40` |

### Test: Ctrl+C quits from any screen

| Case | Screen | Expected |
| ------ | -------- | ---------- |
| Ctrl+C on Input | `ScreenInput` | `tea.Quit` command |
| Ctrl+C on Playlist | `ScreenPlaylist` | `tea.Quit` command |
| Ctrl+C on Downloading | `ScreenDownloading` | `tea.Quit` command |
| Ctrl+C on Done | `ScreenDone` | `tea.Quit` command |
