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
