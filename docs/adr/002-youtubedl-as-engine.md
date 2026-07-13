# ADR-002: yt-dlp as Download Engine

**Status:** Accepted  
**Date:** 2026-07-12  
**Driver:** @juan-arch

## Context

Necesitamos un engine que transforme URLs de YouTube/YouTube Music en archivos MP3 con metadata.

## Decision

Usamos **yt-dlp** como engine de descarga. Se invoca como subproceso vía `os/exec`.

## Rationale

- **Madurez**: yt-dlp es el fork activo de youtube-dl, con mantenimiento constante y actualizaciones ante cambios de YouTube.
- **Soporte de formatos**: extracción de audio en opus, m4a, mp3. Post-processing con ffmpeg.
- **Metadata**: `--embed-metadata` embebe título, artista, álbum, thumbnail, etc.
- **Playlists**: resuelve playlists completas nativamente.
- **YouTube Music**: tiene extractor específico (`ytmusic`) con mejor metadata de artista/álbum.
- **Comunidad**: amplia, documentada, con solución para casi cualquier edge case.

## Consequences

- **Dependencia externa**: el usuario necesita tener yt-dlp instalado, o lo bundleamos.
- **ffmpeg**: yt-dlp necesita ffmpeg para convertir a MP3. Es dependencia indirecta.
- **Rate limiting**: YouTube puede rate-limit después de muchas requests. No lo abordamos en MVP.

## Rejected Alternatives

- **Construir un extractor propio**: inviable. YouTube cambia su HTML/API constantemente.
- **Usar youtube-dl (original)**: sin mantenimiento activo.
- **pytube / yt-dlp como librería Python**: requeriría Python runtime. yt-dlp como binario es más portable.
