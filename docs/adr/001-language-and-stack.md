# ADR-001: Language and Stack

**Status:** Accepted  
**Date:** 2026-07-12  
**Driver:** @juan-arch

## Context

Necesitamos una TUI para un downloader de música que usa yt-dlp como engine. Las opciones principales son TypeScript (Node.js) o Go.

## Decision

Usamos **Go** con **Bubble Tea** (Charm) para la TUI.

## Rationale

- **Binario único**: `go build` produce un solo binario, sin dependencias de runtime. El target es usuario tech, distribución vía GitHub releases.
- **Concurrencia nativa**: goroutines para manejar descargas en background sin complicar el modelo de UI.
- **`os/exec` sólido**: yt-dlp se invoca como subproceso. Go tiene APIs maduras para spawn, pipeo de stdout/stderr, y señales.
- **Bubble Tea**: framework Elm-style (model → update → view) que evita los problemas clásicos de TUIs con estado mutable. Ecosistema amplio (Bubbles: input, list, spinner, progress, table).
- **Zero runtime**: no requiere Node.js, npm, ni empaquetado.

## Consequences

- **Positivo**: distribución trivial, menor curva de aprendizaje para contribuciones si el contributor sabe Go.
- **Negativo**: TypeScript developer podría preferir JS/TS, pero Go es el mejor fit para el dominio.

## Rejected Alternatives

- **TypeScript + Blessed/Ink**: requiere Node runtime o empaquetado (nexe, pkg). Concurrencia menos natural. Más boilerplate para manejo de subprocesos.
