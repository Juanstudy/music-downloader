## Skeleton TUI para music-downloader

Basado en el skill `tui-design` y los ADRs del proyecto.

---

### 1. Layout Paradigm: Focused Single-Panel (MVP)

Del selector de layouts del skill, para el MVP uso **Header + Scrollable List** con transiciones entre pantallas enfocadas. Esto mantiene el MVP simple y funcional. La arquitectura queda preparada para migrar a **Persistent Multi-Panel** post-MVP.

**Flujo de pantallas:**

```
Input ──→ Resolving ──→ Playlist ──→ Downloading ──→ Done
                ↑          │              │
                └──────────┘              │
                     (single track)       │
                               └──────────┘
                                 (cancel/quit)
   ↑ = spinner breve mientras resuelve
```

Cada pantalla es una vista completa (no paneles simultáneos), como recomienda el skill para "single-purpose, <20 actions" → direct keybinding.

---

### 2. Screen Definitions (patrón gentleman-bubbletea)

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

**Cada screen tiene:**
- Constante `Screen` en model.go
- Estado en `Model` struct
- Handler `handle{Nombre}Keys` en `update.go`
- Vista `render{Nombre}View` en `view.go`

---

### 3. Wireframes por Screen

**ScreenInput** — Pegar URL
```
┌─────────────────────────────────────┐
│  ♪ music-dl                         │
│                                     │
│  ┌ Enter URL ─────────────────────┐ │
│  │ https://youtube.com/...        │ │
│  └────────────────────────────────┘ │
│                                     │
│  [Enter] add  [d]ownload now  [q]   │
└─────────────────────────────────────┘
```

**ScreenResolving** — Spinner con estado
```
┌─────────────────────────────────────┐
│  ♪ music-dl                         │
│                                     │
│     ⠋ Resolving playlist...          │
│        fetching 24 tracks            │
│                                     │
└─────────────────────────────────────┘
```
Usa el spinner del skill (braille dots `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`, 80ms).

**ScreenPlaylist** — Tracklist con selección
```
┌─────────────────────────────────────┐
│  ♪ "Rock Classics" · 24 tracks      │
│                                     │
│  ───────────────────────────────── │
│  ✓ Bohemian Rhapsody      Queen   │
│  ✓ Stairway to Heaven     Led Zep │
│  ☐ Back in Black          AC/DC   │
│  ☐ Hotel California       Eagles  │
│  ☐ Sweet Child O' Mine    GnR     │
│  ...                              │
│  ───────────────────────────────── │
│                                     │
│  [Space] toggle  [a]ll/none  [d]l  │
│  [/]search  [Enter] download  [q]   │
└─────────────────────────────────────┘
```
Usa patrón de lista scrolleable del skill.

**ScreenDownloading** — Cola con estados
```
┌─────────────────────────────────────┐
│  ♪ Downloading (3/5)                │
│                                     │
│  ✓ Bohemian Rhapsody              │
│  ✓ Stairway to Heaven             │
│  █ Back in Black     ◄ descargando │
│    Hotel California               │
│    Sweet Child O' Mine            │
│                                     │
│  1 failed · Saved to ~/Music/music-dl/
│                                     │
│  [q] cancel & quit  [Esc] background│
└─────────────────────────────────────┘
```
Usa estilo de lista con status: `✓` success, `✗` failed, `█` downloading, `⏎` pending.

**ScreenDone** — Resumen final
```
┌─────────────────────────────────────┐
│  ♪ Downloads Complete               │
│                                     │
│  ✓ 4 downloaded                    │
│  ✗ 1 failed (video privado)        │
│                                     │
│  ~/Music/music-dl/                  │
│                                     │
│  [Enter] new download  [q] quit     │
└─────────────────────────────────────┘
```

---

### 4. Color System (del skill tui-design)

Uso los **slots semánticos** del skill mapeados a Lipgloss:

```go
// styles.go
var Colors = struct {
    Default     lipgloss.Color
    Muted       lipgloss.Color
    Emphasis    lipgloss.Color
    Base        lipgloss.Color
    Surface     lipgloss.Color
    Selection   lipgloss.Color
    Accent      lipgloss.Color
    Error       lipgloss.Color
    Warning     lipgloss.Color
    Success     lipgloss.Color
    Info        lipgloss.Color
}{
    // Valores base compatibles con temas claro/oscuro
    // Usando colores ANSI 16 como foundation
}
```

**Estrategia del skill:** diseñar primero en ANSI 16 (fondo negro/blanco), luego capa true color. Uso el `lipgloss.AdaptiveColor` para light/dark.

---

### 5. Interaction Model (del skill)

| Capa | Keys | Visible |
|------|------|---------|
| **L0: Universal** | ↑↓, Enter, Esc, q | Footer siempre visible |
| **L1: Vim motions** | j/k, /, gg, G | Footer + `?` help |
| **L2: Actions** | d=download, a=all, n=new URL | `?` help overlay |

**Footer format:** `[q]uit [/]search [Space]toggle [Enter]select`

---

### 6. File Structure

```
cmd/
  music-dl/
    main.go              # Entrypoint, pre-flight checks (yt-dlp, ffmpeg)

internal/
  tui/
    model.go             # Model struct, Screen enum, app state
    update.go            # tea.Update handler, message types, key routing
    view.go              # tea.View, screen routing
    styles.go            # Lipgloss colors, borders, spacing (semantic slots)
    screens/
      input.go           # ScreenInput: view + key handler
      resolving.go       # ScreenResolving: spinner view
      playlist.go        # ScreenPlaylist: track list + selection
      downloading.go     # ScreenDownloading: queue + progress
      done.go            # ScreenDone: results summary

  download/
    engine.go            # Downloader interface
    ytdlp.go             # yt-dlp implementation (os/exec)

  model/
    media.go             # Media struct (Title, Artist, Duration, URL, Status)
    queue.go             # Queue type ([]Media, current index, etc.)
```

---

### 7. Keyboard Design (mapeo completo)

| Key | ScreenInput | ScreenPlaylist | ScreenDownloading | ScreenDone |
|-----|-------------|----------------|-------------------|------------|
| `Enter` | Add URL / download | Download selected | — | New download |
| `Esc` | — | Back to input | — | — |
| `q` | Quit | Quit | Cancel + quit | Quit |
| `↑/k` | — | Move up | — | — |
| `↓/j` | — | Move down | — | — |
| `Space` | — | Toggle selection | — | — |
| `a` | — | Select all | — | — |
| `n` | — | Deselect all | — | — |
| `/` | — | Filter list | — | — |
| `d` | Download now | — | — | — |
| `?` | Help overlay | Help overlay | Help overlay | Help overlay |

---

### 8. MVC Pattern (Bubble Tea)

```
┌─────────────────────────────────────────────────┐
│                   main.go                        │
│  checkDeps() → p := tea.NewProgram(Model{})     │
│  p.Run()                                         │
└──────────┬──────────────────────────┬────────────┘
           │ Model                    │ View
           ▼                          ▼
┌────────────────────┐    ┌──────────────────────┐
│    model.go        │    │     view.go          │
│  Screen            │    │  ScreenInput → view  │
│  URLInput          │    │  ScreenResolving →   │
│  Playlist          │◄──►│  ScreenPlaylist →    │
│  Queue             │    │  ScreenDownloading → │
│  Results           │    │  ScreenDone → view   │
│  Width, Height     │    │                      │
└──────────┬─────────┘    └──────────────────────┘
           │ Update
           ▼
┌──────────────────────┐
│    update.go         │
│  tea.KeyMsg →        │
│   route by Screen    │
│  tea.WindowSizeMsg → │
│   resize             │
│  downloadMsg →       │
│   update queue       │
└──────────────────────┘
```

---

### 9. Compatibilidad Checklist (del skill)

- [x] Works at 80x24 minimum (cada pantalla es vertical, scroll si necesario)
- [x] Handles SIGWINCH (tea.WindowSizeMsg nativo)
- [x] Dark AND light themes (lipgloss.AdaptiveColor)
- [x] Respects NO_COLOR (check inicial + skip styles)
- [ ] Works inside tmux/zellij (Bubble Tea maneja esto)
- [x] Keyboard-only (mouse no requerido)

---

### 10. Constraints del MVP (ADR-003) que impactan el diseño

| ADR Constraint | Impacto en TUI |
|---------------|---------------|
| Sin progress bars | ScreenDownloading muestra solo texto de estado, sin barra |
| Sin cola persistente | Queue en memoria, se pierde al cerrar |
| Descarga secuencial | Un track a la vez, cola ordenada |
| Feedback mínimo | Texto "descargando X", "falló Y", "completó Z" |
| Sin notificaciones | Sin toast/sistema de notificaciones |

---

### Resumen de principios del skill aplicados

1. **Keyboard-first**: cada acción tiene binding, footer visible
2. **Spatial consistency**: misma estructura de layout en todas las screens
3. **Progressive disclosure**: footer con essentials, `?` con todo
4. **Async**: resolución y descarga en goroutines con tea.Cmd
5. **Semantic color**: slots funcionales, no hardcode hex
6. **Contextual intelligence**: keybindings cambian por screen
7. **Design in layers**: ANSI 16 primero, true color después