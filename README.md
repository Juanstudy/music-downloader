# music-dl ♪

**TUI music downloader** — pega una URL de YouTube o YouTube Music, seleccioná los tracks, y descargalos como MP3 con metadata incrustada.

Construido con [Go](https://go.dev) + [Bubble Tea](https://github.com/charmbracelet/bubbletea), impulsado por [yt-dlp](https://github.com/yt-dlp/yt-dlp).

## Características

- Pegar URLs de YouTube y YouTube Music (soporta tracks individuales y playlists)
- Resolución plana de playlists → selección individual de tracks
- Descarga secuencial con feedback en pantalla
- Salida en MP3 con metadata (artista, título, álbum) y thumbnail incrustados
- Detección automática de dependencias faltantes al arranque

### Roadmap (post-MVP)

- Búsqueda semántica por nombre/artista/álbum
- Calidad configurable (128k, 320k, lossless)
- Progress bars en descargas
- Descarga concurrente
- Cola persistente (sobrevive al cierre de la app)
- Tema claro/oscuro automático

### Implementado recientemente

- **Spotify como fuente** — pegar URLs de Spotify, resuelve metadata y busca el track en YouTube automáticamente
- Filtro `/` en la playlist (filtrado en tiempo real por título o artista)
- Help overlay con `?`
- Manejo consistente de errores entre pantallas

## Requisitos

- **yt-dlp** — engine de descarga
- **ffmpeg** — conversión a MP3 y post-procesamiento

### Instalación de dependencias

```bash
# macOS
brew install yt-dlp ffmpeg

# Debian/Ubuntu
sudo apt install yt-dlp ffmpeg

# Arch Linux
sudo pacman -S yt-dlp ffmpeg

# O con pip (cualquier SO)
pip install yt-dlp
# e instalá ffmpeg por separado: https://ffmpeg.org/download.html
```

## Spotify (fuente adicional)

Spotify está soportado como fuente de búsqueda: pegás una URL de Spotify, la app obtiene la metadata
(título, artista, duración) y busca automáticamente el track en YouTube para descargarlo.

### Requisito: cuenta Premium

> ⚠️ Desde Febrero 2026, Spotify exige que el **dueño de la app en el Developer Dashboard**
> tenga una **suscripción Premium activa**. Sin Premium, la API devuelve `403 Forbidden`.

Si tenés Premium, el proceso es:

1. Andá a [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard)
2. Creá una app → copiá **Client ID** y **Client Secret**
3. Configurá el archivo local:

```bash
mkdir -p ~/.config/music-dl
cat > ~/.config/music-dl/config.toml << 'EOF'
[spotify]
client_id = "tu_client_id"
client_secret = "tu_client_secret"
EOF
chmod 600 ~/.config/music-dl/config.toml
```

1. En la app, apretá **Tab** para cambiar la fuente a Spotify.

Sin este archivo, la app arranca igual (solo YouTube) y el Tab solo cicla Auto/YouTube.

## Instalación

### Desde GitHub Releases

Descargá el binario para tu plataforma desde [releases](https://github.com/Juanstudy/music-downloader/releases), hacele `chmod +x` y movelo al `$PATH`:

```bash
chmod +x music-dl
mv music-dl ~/.local/bin/
```

### Desde fuente

```bash
git clone https://github.com/Juanstudy/music-downloader.git
cd music-downloader
go build -o music-dl ./cmd/music-dl
```

## Uso

```bash
music-dl
```

Se abre una interfaz interactiva:

1. **Pegá una URL** de YouTube o YouTube Music
2. La app resuelve el contenido (track individual o playlist)
3. Si es una playlist, **seleccioná** los tracks que querés con `Space`
4. Presioná `Enter` para comenzar la descarga
5. Los archivos MP3 se guardan en `~/Music/music-dl/`

### Controles

| Tecla | Acción |
| ------- | -------- |
| `Enter` | Confirmar / resolver / descargar |
| `j` / `↓` | Mover cursor hacia abajo |
| `k` / `↑` | Mover cursor hacia arriba |
| `Space` | Seleccionar/deseleccionar track |
| `a` | Seleccionar todos |
| `n` | Deseleccionar todos |
| `r` | Nueva URL (pantalla de resultado) |
| `Esc` | Retroceder / cancelar |
| `q` | Salir |
| `?` | Ayuda |

## Arquitectura

```
cmd/music-dl/main.go          ← Entrypoint, preflight check, DI
internal/
  core/
    domain/                   ← Tipos de dominio (Media, Status, Error)
    ports/                    ← Interfaces (Searcher, Downloader)
    service/                  ← Orchestrator (coordina búsqueda + descarga)
  adapters/
    searcher/                 ← Implementación yt-dlp (resolución de URLs)
    downloader/               ← Implementación yt-dlp (descarga + conversión)
    filesystem/               ← Manejo de archivos de salida
    preflight/                ← Verificación de dependencias al arranque
  tui/                        ← Bubble Tea TUI (5 pantallas)
    model.go                  ← Estado global y screens
    update.go                 ← Manejo de eventos y transiciones
    view.go                   ← Renderizado de cada pantalla
    styles.go                 ← Sistema de colores semánticos
    messages.go               ← Mensajes asíncronos (Bubble Tea Cmds)
    keys.go                   ← Definición de keybindings
```

### Stack

| Capa | Tecnología |
| ------ | ----------- |
| Lenguaje | Go 1.26 |
| UI | [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Bubbles](https://github.com/charmbracelet/bubbles) |
| Estilos | [Lipgloss](https://github.com/charmbracelet/lipgloss) |
| Engine | [yt-dlp](https://github.com/yt-dlp/yt-dlp) (subproceso) |
| Post-procesamiento | ffmpeg (vía yt-dlp) |

## Desarrollo

```bash
# Build
go build -o music-dl ./cmd/music-dl

# Test
go test ./...

# Test con cobertura
go test -cover ./...
```

## Troubleshooting

| Problema | Causa probable | Solución |
| ---------- | --------------- | ---------- |
| `yt-dlp not found` | yt-dlp no está instalado | `pip install yt-dlp` o `brew install yt-dlp` |
| `ffmpeg not found` | ffmpeg no está instalado | Instalá ffmpeg con tu gestor de paquetes |
| No se descarga nada | Sin conexión o URL inválida | Verificá tu conexión y que la URL sea de YouTube |
| Error en medio de una playlist | Video privado, eliminado o age-restricted | La app salta el track y sigue con el próximo |
| `permission denied` al ejecutar | El binario no tiene permisos de ejecución | `chmod +x music-dl` |

## Licencia

MIT
