# Adapters — Downloader (yt-dlp) Specification

> **Change history:** updated by `audio-quality` (2026-08-01) — configurable `--audio-bitrate` (128k / 192k / 320k, default 320k), constructor option + mid-session setter. Port signature unchanged. See `openspec/changes/audio-quality/spec/spec.md`.

## Purpose

Implement the `Downloader` port by invoking `yt-dlp -x --audio-format mp3 [--audio-bitrate <q>] --embed-metadata` to download one track and convert it to MP3. The bitrate flag is present when a quality is configured and absent otherwise.

## Requirements

### Requirement: Downloader invokes yt-dlp with audio-extraction flags

The system MUST provide a `Downloader` implementation that shells out to `yt-dlp` for audio extraction.

```go
type Downloader struct{}

func NewDownloader(opts ...Option) *Downloader
func (d *Downloader) Download(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error)
```

#### Scenario: Download calls yt-dlp with correct audio flags

- GIVEN a `Downloader` instance and a resolved `domain.Media`
- WHEN `Download(ctx, media, outputDir)` is called
- THEN the executable `yt-dlp` MUST be invoked
- AND the arguments MUST include `-x` (extract audio)
- AND the arguments MUST include `--audio-format mp3`
- AND the arguments MUST include `--embed-metadata`
- AND the arguments MUST include `-o` with a template like `{artist} - {title}.mp3`
- AND the output template path MUST be prefixed with `outputDir`

#### Scenario: Download includes --audio-bitrate when a quality is configured

- GIVEN a `Downloader` configured with quality `192k`
- WHEN `Download(ctx, media, outputDir)` is called
- THEN the arguments MUST include `--audio-bitrate 192k`
- AND `--audio-bitrate 192k` MUST appear immediately after `--audio-format mp3`

#### Scenario: Download omits --audio-bitrate when no quality is configured

- GIVEN a `Downloader` constructed without a quality option
- WHEN `Download(ctx, media, outputDir)` is called
- THEN the arguments MUST NOT include `--audio-bitrate`
- AND the invocation MUST be identical to the pre-change behavior

#### Scenario: Download uses artist - title filename template

- GIVEN a `Downloader` instance
- WHEN downloading a track with artist "Some Artist" and title "Some Song"
- THEN the output filename MUST be `Some Artist - Some Song.mp3`
- AND `DownloadResult.OutputPath` MUST be the full path to that file

#### Scenario: Download returns the updated Media with StatusDone

- GIVEN a `Downloader` instance
- WHEN `Download(ctx, media, outputDir)` succeeds
- THEN `DownloadResult.Media.Status` MUST be `StatusDone`
- AND `DownloadResult.Media.OutputPath` MUST be set to the downloaded file path
- AND `DownloadResult.OutputPath` MUST match `DownloadResult.Media.OutputPath`

#### Scenario: Download returns error when yt-dlp is not found

- GIVEN a `Downloader` instance
- WHEN `yt-dlp` is not on `$PATH`
- THEN `Download` MUST return a `domain.Error` with code `ErrorBinaryNotFound`

#### Scenario: Download returns error on yt-dlp non-zero exit

- GIVEN a `Downloader` instance
- WHEN `yt-dlp` exits with a non-zero status (network error, disk full, etc.)
- THEN `Download` MUST return a non-nil error
- AND the error type SHOULD be `domain.Error` with appropriate code

#### Scenario: Download validates output directory exists

- GIVEN a `Downloader` instance
- WHEN `outputDir` does not exist
- THEN `Download` SHOULD return a `domain.Error` with code `ErrorGeneric` or `ErrorDiskFull`
- OR the caller (`filesystem` package) is responsible for ensuring the directory exists before calling `Download`

#### Scenario: Download returns error when yt-dlp is killed/context cancelled

- GIVEN a `Downloader` instance
- WHEN the context is cancelled during download
- THEN `Download` MUST return a non-nil error
- AND the error SHOULD wrap `context.Canceled`

#### Scenario: Download reports actual output path via --print filename

- GIVEN a `Downloader` instance
- WHEN `Download` completes successfully
- THEN `OutputPath` MUST be determined by running `yt-dlp --print filename -o <template> <url>` to get the actual filename (or using a post-download file discovery mechanism)

### Requirement: Downloader supports configurable audio bitrate

The system MUST allow the MP3 audio bitrate to be configured at construction time (`WithAudioBitrate`) or changed during a session via a setter, so a quality change takes effect for the next download without a restart. Valid values are `128k`, `192k`, and `320k`. When no quality is configured, the downloader MUST behave exactly as before (no `--audio-bitrate` flag). `--audio-bitrate` applies to lossy formats only (MP3 scope); yt-dlp MUST NOT up-sample a source encoded below the requested bitrate.

#### Scenario: each quality level produces the corresponding flag

- GIVEN a `Downloader` constructed with `128k`, then one with `192k`, then one with `320k`
- WHEN `Download` is called for each
- THEN the arguments MUST include `--audio-bitrate 128k`, `--audio-bitrate 192k`, and `--audio-bitrate 320k` respectively

#### Scenario: mid-session setter affects the next download

- GIVEN a `Downloader` running with quality `128k`
- WHEN the setter changes quality to `320k`
- THEN the next `Download` call MUST use `--audio-bitrate 320k`

#### Scenario: change mid-download leaves the in-flight track unaffected

- GIVEN a download in progress for track A at `128k`
- WHEN the quality is changed to `320k` while track A is downloading
- THEN track A MUST complete with `--audio-bitrate 128k`
- AND track B (next in the queue) MUST use `--audio-bitrate 320k`

#### Scenario: source below requested bitrate is not up-sampled

- GIVEN a `Downloader` configured with quality `320k`
- WHEN downloading a track whose source is encoded at `128k`
- THEN the download MUST complete successfully at the source's bitrate
- AND the downloader MUST NOT instruct yt-dlp to up-sample beyond the source

---

## Test Specifications

### Test: Downloader args builder (unit, no real yt-dlp)

**File:** `internal/adapters/downloader/ytdlp_test.go`

| Case | Input | Expected |
| ------ | ------- | ---------- |
| No option | `NewDownloader()` | No `--audio-bitrate` in args |
| `128k` / `192k` / `320k` | `WithAudioBitrate(...)` | `--audio-bitrate` immediately after `--audio-format mp3` |
| Setter mid-session | Set after construction | Next args use new bitrate |
| Existing flags intact | Any config | `-x`, `--audio-format mp3`, `--embed-metadata`, `-o` template present |

### Test: Downloader (integration)

**File:** `internal/adapters/downloader/ytdlp_test.go`

**Skip condition:** `testing.Short()` — integration tests require real `yt-dlp` on `$PATH`.

| Case | Input | Expected |
| ------ | ------- | ---------- |
| Download single track | Valid resolved Media, TempDir output | Returns `DownloadResult` with `StatusDone`, file exists on disk |
| Download with missing binary | PATH without yt-dlp | `domain.Error` with `ErrorBinaryNotFound` |
| Download with invalid URL | Media with bad URL | Non-nil error |
