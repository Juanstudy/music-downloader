# Glossary

Términos del dominio usados en el proyecto music-downloader.

| Término | Definición |
|---|---|
| **Provider** | Fuente de contenido musical. Implementa las interfaces `Searcher` y/o `Downloader`. Ej: YouTube, Spotify, SoundCloud. |
| **Engine** | Módulo que ejecuta la descarga concreta. En el MVP es yt-dlp. |
| **Media** | Representación abstracta de un ítem musical: track, álbum, playlist. Contiene metadata (título, artista, álbum, duración, URL, source). |
| **Playlist** | Colección ordenada de `Media`. Puede venir de YouTube (video playlist) o resolverse desde un álbum/playlist de Spotify. |
| **Queue** | Cola de descargas pendientes. En el MVP vive en memoria y se pierde al cerrar la app. |
| **ISRC** | International Standard Recording Code. Identificador único por grabación musical. Usado para resolver tracks de Spotify a YouTube. |
| **Extractor** | Componente de yt-dlp que sabe cómo obtener metadata y streams de un sitio específico (YouTube, SoundCloud, etc.). |
| **Post-processing** | Etapa posterior a la descarga del stream: conversión a MP3, embedding de metadata, thumbnail. yt-dlp + ffmpeg. |
| **Bubble Tea** | Framework Elm-style para construir TUIs en Go. Modelo: `Model` → `Update` → `View`. |
| **Bubbles** | Componentes reutilizables para Bubble Tea: input, list, spinner, progress bar, table, etc. |
| **Screen** | Una de las 5 vistas principales de la TUI: Input, Resolving, Playlist, Downloading, Done. Cada screen tiene su propio handler de teclas y su propia vista. |
| **Layout Paradigm** | Estrategia visual definida en el skill `tui-design`: Multi-Panel, Miller Columns, Widget Dashboard, etc. El MVP usa Focused Single-Panel. |
| **Semantic Slot** | Color definido por función (ej: `colorSuccess`, `colorError`) en lugar de por valor hex. Permite cambiar temas sin modificar vistas. |
| **Footer** | Barra inferior que muestra las teclas disponibles en la screen actual. Formato: `[key] acción`. |
| **Help Overlay** | Superposición que se activa con `?` y muestra todos los shortcuts de la screen actual. |
| **Status Bar** | (futuro) Línea de estado persistente que muestra información contextual. |
| **L0/L1/L2** | Capas del modelo de interacción del skill `tui-design`: Universal, Vim motions, Actions. Determinan qué teclas se muestran y a quién están dirigidas. |
| **AdaptiveColor** | Función de Lipgloss que elige color claro u oscuro según el tema del terminal. Usada para los semantic slots. |
