# Adapters — Downloader (yt-dlp) Specification

## Purpose

Implement the `Downloader` port by invoking `yt-dlp -x --audio-format mp3 --embed-metadata` to download one track and convert it to MP3.

## Requirements

### Requirement: Downloader invokes yt-dlp with audio-extraction flags

The system MUST provide a `Downloader` implementation that shells out to `yt-dlp` for audio extraction.

```go
type Downloader struct{}

func NewDownloader() *Downloader
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

---

## Test Specifications

### Test: Downloader (integration)

**File:** `internal/adapters/downloader/ytdlp_test.go`

**Skip condition:** `testing.Short()` — integration tests require real `yt-dlp` on `$PATH`.

| Case | Input | Expected |
| ------ | ------- | ---------- |
| Download single track | Valid resolved Media, TempDir output | Returns `DownloadResult` with `StatusDone`, file exists on disk |
| Download with missing binary | PATH without yt-dlp | `domain.Error` with `ErrorBinaryNotFound` |
| Download with invalid URL | Media with bad URL | Non-nil error |
