# ADR-003: MVP Scope

**Status:** Accepted  
**Date:** 2026-07-12  
**Driver:** @juan-arch

## Context

Definir qué entra en el MVP y qué queda para post-1.0.0.

## Decision

El MVP incluye:

- **Fuentes**: solo URLs de YouTube y YouTube Music.
- **Input**: solo URLs (sin búsqueda semántica integrada).
- **Playlists**: resolución y descarga completa de playlists.
- **Concurrencia**: descarga secuencial (una atrás de otra).
- **Errores**: si una descarga falla, se salta y continúa con la siguiente. Sin manejo sofisticado de errores.
- **Calidad**: la que yt-dlp decide por defecto (opus ~160k).
- **Salida**: MP3 con metadata embedida.
- **Carpeta**: configurable por usuario (`~/Music/music-dl/` por defecto).
- **Progreso**: feedback mínimo (qué se está descargando actualmente, qué falló, qué terminó). Sin progress bars, cola visual detallada, ni notificaciones.
- **Persistencia**: no hay cola persistente (si cerrás la app, la cola se pierde).
- **Caché de búsquedas**: no.

Queda fuera del MVP (post-1.0.0):

- Búsqueda por nombre/artista/álbum.
- Providers de Spotify y SoundCloud.
- Calidad configurable (128k, 320k, lossless).
- Progress bars, cola visual enriquecida, notificaciones.
- Configuración de naming de archivos.
- Descarga concurrente.
- Cola persistente.
- Caché de búsquedas.

## Rationale

- **Principio YAGNI**: no agregar funcionalidad que no se necesita hasta que se necesita.
- **Time-to-value**: que ande rápido, que descargue música, que sea usable. El refinamiento visual y funcional viene después.
- **Feedback loop temprano**: un MVP pequeño permite obtener feedback de usuarios reales antes de invertir en features complejas.

## Consequences

- El MVP va a ser "feo" en lo visual pero funcional.
- Algunos usuarios van a pedir features del post-MVP desde el día uno. Eso valida qué priorizar.
- La arquitectura de providers (interfaces) se define desde el MVP aunque solo haya un provider concreto (YouTube). Así agregar Spotify/SoundCloud después no requiere refactor.
