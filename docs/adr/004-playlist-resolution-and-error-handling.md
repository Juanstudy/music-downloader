# ADR-004: Playlist Resolution and Error Handling

**Status:** Accepted  
**Date:** 2026-07-12  
**Driver:** @juan-arch

## Context

Dos aspectos del comportamiento del MVP que necesitan definición:
1. Cómo maneja la TUI una URL de playlist (resolución vs descarga directa).
2. Qué pasa cuando algo sale mal (binarios faltantes, URL inválida, video no disponible, sin conexión).

---

## Part 1: Playlist Resolution

### Decision

Usamos el modelo **híbrido (C)**:
1. Al pegar una URL de playlist, la app resuelve la lista completa de canciones usando yt-dlp `--flat-playlist --dump-json`.
2. Muestra la lista en la TUI con título, duración y estado "pendiente" para cada track.
3. El usuario puede seleccionar/deseleccionar tracks individuales o descargar todos.
4. Al confirmar, arranca la descarga secuencial.

### Rationale

- El usuario necesita ver qué va a descargar antes de mandar 20 tracks al engine.
- Permite filtrar tracks no deseados (versiones en vivo, covers, etc.).
- La resolución inicial es rápida (solo metadata, sin descargar nada).
- La descarga secuencial evita rate limiting temprano.

### Consequences

- Hay una pausa visible entre "pegar URL" y "ver la lista". Para playlists muy grandes (+100 tracks) puede tomar unos segundos.
- La TUI necesita un estado intermedio: "resolviendo playlist..." con spinner.

---

## Part 2: Error Handling

### Decision

| Escenario | Comportamiento MVP |
|---|---|
| **yt-dlp no instalado** | Verificación al iniciar la app (antes de la TUI). Si falta, mostrar error y salir. |
| **ffmpeg no instalado** | Verificación al iniciar la app (antes de la TUI). Si falta, mostrar error y salir. |
| **URL inválida** | Detectar formato inválido antes de llamar a yt-dlp. Mostrar error inline en la TUI, pedir otra URL. No sale de la app. |
| **Video privado / eliminado / age-restricted** | yt-dlp falla. Marcar track como "failed" y continuar con el siguiente. |
| **Sin conexión** | Reintentar 1 vez. Si vuelve a fallar, marcar como "failed" y continuar. |
| **Formato de URL no soportado** | Si no es YouTube ni YouTube Music, mostrar error explicando que solo esas fuentes están soportadas en MVP. |

### Rationale

- **Check pre-TUI**: mejor fallar rápido y con un mensaje claro que tener una TUI que después no funciona.
- **URL inválida**: se detecta temprano con un parseo simple de URLs antes de invocar yt-dlp.
- **Fail fast, continue**: el MVP prioriza no bloquear al usuario. Si un track falla, se salta y sigue.
- **Sin conexión**: el reintento único cubre cortes transitorios. Si es persistente, el usuario ve los fallos y decide.

### Consequences

- El entrypoint de la app (antes de inicializar la TUI) hace dos cosas: verificar binarios y parsear argumentos.
- Si el usuario pasa una URL como argumento CLI, se parsea antes de abrir la TUI y se muestra error sin abrir la TUI.
- El estado "failed" por track necesita ser visible en la lista de resultados.
