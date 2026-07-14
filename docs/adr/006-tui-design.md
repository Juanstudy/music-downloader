# ADR-006: TUI Design

**Status:** Accepted  
**Date:** 2026-07-13  
**Driver:** @juan-arch

## Context

El proyecto necesita una interfaz de terminal (TUI) para que usuarios interactúen con el downloader. Teníamos un skill de diseño de TUI (`tui-design`) con patrones framework-agnóstico, y necesitábamos aplicarlos al stack concreto (Go + Bubble Tea).

Las preguntas de diseño eran:

1. ¿Qué paradigma de layout usar (multi-panel, focused screens, dashboard)?
2. ¿Cómo estructurar la navegación entre estados (input URL, resolver, seleccionar tracks, descargar, resultado)?
3. ¿Qué modelo de interacción por teclado aplicar?
4. ¿Cómo diseñar el sistema de colores para que funcione en cualquier terminal?

## Decisiones

### 1. Layout Paradigm: Focused Single-Panel (MVP)

Usamos **Header + Scrollable List** del [Layout Paradigm Selector](../.agents/skills/tui-design/SKILL.md#1-layout-paradigm-selector) del skill `tui-design`.

Cada pantalla ocupa toda la terminal, con transiciones entre ellas. No hay paneles simultáneos.

**Rationale:**
- El MVP tiene pocas acciones (<20) y un flujo lineal (URL → tracks → descarga → resultado).
- Single-panel es más simple de implementar y testear.
- El modelo mental del usuario es "asistente paso a paso", no "dashboard con paneles simultáneos".
- Cuando el producto crezca, se puede migrar a **Persistent Multi-Panel** (que es el paradigma recomendado para apps multi-herramienta) sin romper la estructura de screens existente.

### 2. Screen Structure: 5 Screens

| Screen | Propósito | Transición |
|---|---|---|
| `ScreenInput` | Pegar URL | Enter → Resolving |
| `ScreenResolving` | Spinner mientras yt-dlp resuelve | OK → Playlist o Downloading; Error → Input |
| `ScreenPlaylist` | Lista de tracks con selección | Enter → Downloading; Esc → Input |
| `ScreenDownloading` | Cola de descarga secuencial | Completado → Done |
| `ScreenDone` | Resumen final | Enter → Input; q → quit |

**Rationale:**
- Cada screen tiene un propósito único y bien definido.
- El flujo sigue el ciclo de vida de una descarga: input → resolver → elegir → descargar → ver resultado.
- Separación clara de responsabilidades: cada screen tiene su handler y su view.
- Las transiciones son predecibles y el usuario siempre sabe dónde está.

### 3. Interaction Model: Vim-Style + Direct Keybinding

Usamos el modelo de **4 capas** del skill `tui-design`:

| Layer | Keys | Visibility |
|---|---|---|
| **L0: Universal** | Arrow keys, Enter, Esc, q | Footer siempre visible |
| **L1: Vim motions** | j/k, /, gg, G | Footer + `?` help |
| **L2: Actions** | Space=toggle, a=all, n=none | `?` help overlay |
| **L3: Power** | (futuro: composición, macros) | Docs |

**Convenciones adoptadas** (lingua franca del skill):

- `j`/`k` — mover cursor arriba/abajo
- `Space` — toggle selección
- `Enter` — confirmar / avanzar
- `Esc` — retroceder / cancelar
- `q` — salir
- `?` — help overlay
- `/` — filtrar (planeado)

**Principio:** el footer muestra solo las teclas relevantes para la pantalla activa. `?` despliega la guía completa.

### 4. Color System: Semantic Slots con AdaptiveColor

Aplicamos la **Table de Semantic Color Slots** del skill `tui-design`:

| Slot | Uso | ANSI 16 equiv | AdaptiveColor dark |
|---|---|---|---|
| `colorDefault` | Texto base | 7 (white) | `#c0caf5` |
| `colorMuted` | Metadata, contadores | 8 (bright black) | `#565f89` |
| `colorAccent` | Teclas, elementos interactivos | 12 (bright blue) | `#7aa2f7` |
| `colorSuccess` | Descargas completadas | 10 (bright green) | `#9ece6a` |
| `colorError` | Errores | 9 (bright red) | `#f7768e` |
| `colorWarning` | Advertencias | 11 (bright yellow) | `#e0af68` |
| `colorInfo` | Información, spinner | 14 (bright cyan) | `#7dcfff` |

**Estrategia de degradación** (del skill §4):

1. `$COLORTERM=truecolor` → true color hex
2. `$TERM` contiene `256color` → 256 color (AdaptiveColor)
3. `$NO_COLOR` está seteado → sin color
4. Default → ANSI 16 (compatible con todo)

Usamos `lipgloss.AdaptiveColor` que automáticamente usa valores distintos para terminales claras y oscuras.

### 5. Ayuda en Tres Niveles

Del skill `tui-design` §3 (Help System):

| Nivel | Trigger | Contenido |
|---|---|---|
| **Siempre visible** | Footer | 3-5 shortcuts esenciales de la screen activa |
| **On demand** | `?` | Overlay con todos los shortcuts de la screen activa |
| **Documentación** | `--help` y man page | Por definir (post-MVP) |

### 6. Animation & Feedback

- **Spinner**: braille dots (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`) a 80ms como recomienda el skill.
- **Sin progress bars**: el MVP usa solo texto ("downloading X"). La arquitectura acepta progress bars post-MVP sin refactor (el canal `progress` ya está en la interfaz `Engine`).
- **Transiciones**: instantáneas (el usuario aprieta una tecla y la pantalla cambia inmediatamente).

### 7. File Naming Output

Per ADR-005: el formato es `{artist} - {title}.mp3` en `~/Music/music-dl/` plano.

## Arquitectura Implementada

```
cmd/music-dl/main.go          ← Entrypoint, pre-flight deps check
internal/
  model/
    media.go                  ← Media struct + Status enum
    queue.go                  ← Queue con máquina de estados
  download/
    engine.go                 ← Interfaz Downloader
    ytdlp.go                  ← Implementación yt-dlp (os/exec)
  tui/
    model.go                  ← Screen enum, Model, Init
    update.go                 ← Update loop, key routing, async msgs
    view.go                   ← Render de las 5 pantallas
    styles.go                 ← Semantic color slots + Lipgloss styles
```

### Bubble Tea Architecture

```
main.go
  │
  ▼
NewProgram(Model{...}) ──► Init() ──► Update() ──► View()
                                  │              │
                                  ▼              ▼
                            handleKeyMsg     renderInputView
                            handleResolve    renderResolvingView
                            handlePlaylist   renderPlaylistView
                            handleDownload   renderDownloadingView
                            handleDone       renderDoneView
                                  │
                                  ▼
                            resolveCmd() → resolveDoneMsg
                            downloadCmd() → downloadDoneMsg
```

## Consequences

- **Positivo**: el usuario tiene una TUI funcional desde el día 1 con <10 teclas para aprender.
- **Positivo**: el sistema de colores semánticos permite agregar temas claro/oscuro sin cambiar el código de las vistas.
- **Positivo**: la interfaz `Engine` permite cambiar yt-dlp por otro backend sin tocar la TUI.
- **Positivo**: la estructura de screens hace trivial agregar nuevas pantallas (config, search, etc.) post-MVP.
- **Negativo**: single-panel no es ideal para usuarios que quieren ver la cola y el detalle simultáneamente (se aborda post-MVP).
- **Negativo**: sin progress bars, el feedback de descarga es mínimo (consciente, por MVP).

## Referencias

- [tui-design skill](../.agents/skills/tui-design/SKILL.md) — framework-agnostic design patterns
- [gentleman-bubbletea skill](../.agents/skills/gentleman-bubbletea/SKILL.md) — Bubble Tea code patterns concretos
- [ADR-001: Stack decision](001-language-and-stack.md) — Go + Bubble Tea
- [ADR-003: MVP Scope](003-mvp-scope.md) — constraints que definen el alcance de la TUI
- [ADR-004: Error Handling](004-playlist-resolution-and-error-handling.md) — estados de error en la TUI
- [ADR-005: Distribution & File Naming](005-distribution-and-file-naming.md) — solo TUI, sin flags CLI
