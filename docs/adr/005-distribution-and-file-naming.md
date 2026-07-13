# ADR-005: Distribution and File Naming

**Status:** Accepted  
**Date:** 2026-07-12  
**Driver:** @juan-arch

## Part 1: Distribution

### Decision

**Opción A** — El usuario instala yt-dlp y ffmpeg por su cuenta como dependencias externas.

El binario `music-dl` es un único binario Go estático. Las dependencias se resuelven al `$PATH` del sistema, verificadas al arranque (ver ADR-004).

### Rationale

- **Tamaño**: el binario es ~10 MB vs ~100 MB si bundleamos ffmpeg.
- **Mantenimiento**: no tener que distribuir actualizaciones de yt-dlp. El usuario lo actualiza con `yt-dlp -U`.
- **Libertad**: el usuario puede tener yt-dlp y ffmpeg por su cuenta (homebrew, pip, distro packages).
- **Licencias**: evitamos el drama de redistribuir binarios de ffmpeg.

### Distribution Method

- GitHub releases con el binario compilado para linux/amd64, linux/arm64, darwin/amd64, darwin/arm64.
- README detalla instalación de dependencias: `brew install yt-dlp ffmpeg` o `pip install yt-dlp` + instalar ffmpeg.
- Script de instalación opcional (post-MVP).

### Consequences

- Fricción inicial: el usuario tiene que instalar yt-dlp + ffmpeg manualmente.
- Error temprano: si faltan, la app no arranca (check pre-TUI).
- Si en el futuro queremos bundlear, es un cambio no disruptivo.

---

## Part 2: Interface

### Decision

**Solo TUI** — No hay flags CLI para descarga directa.

El flujo es siempre: abrir la TUI → pegar URL → interactuar. Sin modo batch.

### Rationale

- Simplifica el entrypoint: no hay que parsear flags, decidir si abrir o no la TUI.
- El usuario target usa la app interactivamente, no en scripts.
- Si en el futuro se necesita batch, se agrega sin romper la TUI.

### Consequences

- No se puede hacer `music-dl "URL"` desde la terminal y obtener el archivo. Siempre hay que pasar por la TUI.
- Ideal para el MVP. Se puede agregar modo batch post-1.0.0 si hay demanda.

---

## Part 3: File Naming

### Decision

**Opción C** — `{artist} - {title}.mp3`. Sin subcarpetas.

Ejemplos:
- `Eagles - Hotel California.mp3`
- `Queen - Bohemian Rhapsody.mp3`
- `Nirvana - Smells Like Teen Spirit.mp3`

Si yt-dlp no provee metadata de artista (ej: video sin metadata musical), se usa el nombre del canal como artista.

### Rationale

- Formato limpio y portable. Funciona en cualquier reproductor.
- Sin subcarpetas: simplicidad. El usuario tiene todos los archivos en `~/Music/music-dl/`.
- El artista y título vienen de la metadata que yt-dlp extrae (`--embed-metadata`). Si no hay metadata confiable, se usa fallback título del video (sanitizado).

### Consequences

- Posible colisión si dos canciones tienen el mismo `artist - title`. Remoto pero posible. En el MVP se sobrescribe (documentado como limitación).
- Si en el futuro se quiere estructura de carpetas (`{artist}/{album}/{track} - {title}.mp3`), se agrega como feature configurable sin romper el naming actual.
- Sin subcarpetas = directorio plano. Si un usuario baja 500 canciones, la carpeta se llena. Aceptado como limitación del MVP.
