# Apply Progress — PR 1: Setup + Config + URL Parsing

## Status

- **Phase:** sdd-apply
- **Change:** spotify-adapters
- **Batch:** PR 1 (Tasks 1.1–1.3, 2.1–2.2)
- **Strict TDD:** Enabled
- **Delivery boundary:** PR 1 — no changes outside `internal/adapters/spotify/`

## Completed Tasks

### Task 1.1 — Package structure + TOML dependency

- [x] Added `github.com/BurntSushi/toml v1.6.0` to `go.mod`
- [x] Ran `go mod tidy` (clean)
- [x] Created `internal/adapters/spotify/` directory
- [x] Created `internal/adapters/spotify/spotify.go` placeholder with `package spotify`

### Task 1.2 — Config loading (`config.go`)

- [x] Implemented `ConfigPath()` — respects `$XDG_CONFIG_HOME`, fallback `~/.config`
- [x] Implemented `Config` struct with `Spotify` sub-struct and TOML tags (`client_id`, `client_secret`)
- [x] Implemented `LoadConfig(path)` — reads/parses TOML, returns `(nil, nil)` for missing file, error for malformed TOML

### Task 1.3 — Config unit tests (`config_test.go`)

- [x] `TestLoadConfig_Valid` — valid TOML, asserts fields
- [x] `TestLoadConfig_FileNotFound` — non-existent path → `(nil, nil)`
- [x] `TestLoadConfig_Malformed` — invalid TOML → error
- [x] `TestConfigPath_Default` — path contains `.config/music-dl/config.toml`
- [x] `TestConfigPath_XDG` — `$XDG_CONFIG_HOME` respected

### Task 2.1 — Spotify URL parsing (`url.go`)

- [x] Implemented `parseSpotifyURL()` (unexported) — supports `https://open.spotify.com/track/{id}` and `spotify:track:{id}`
- [x] Playlist/album/artist URLs → `"only track URLs are supported in this version"`
- [x] Non-Spotify URLs → error
- [x] ID validated with `[a-zA-Z0-9]+`
- [x] Pure function — no domain errors, just `errors.New`

### Task 2.2 — URL parsing unit tests (`url_test.go`)

- [x] Table-driven tests with 10 cases covering all spec scenarios
- [x] Valid track URL, valid URI, invalid URL, playlist, album, artist, non-Spotify, empty, URI with no ID, short ID

## Files Changed

| File | Action |
| ------ | -------- |
| `go.mod` | Modified — added BurntSushi/toml v1.6.0 |
| `go.sum` | Modified — dependency resolution |
| `internal/adapters/spotify/spotify.go` | Created — package placeholder |
| `internal/adapters/spotify/config.go` | Created — config loading implementation |
| `internal/adapters/spotify/config_test.go` | Created — config unit tests (5 cases) |
| `internal/adapters/spotify/url.go` | Created — URL parsing implementation |
| `internal/adapters/spotify/url_test.go` | Created — URL parsing unit tests (10 cases) |
| `openspec/changes/spotify-adapters/tasks.md` | Modified — 5 checkboxes marked `[x]` |
| `openspec/changes/spotify-adapters/apply-progress.md` | Created/Updated — this file |

## Test Results

```
$ go test ./internal/adapters/spotify/... -v -count=1
=== RUN   TestLoadConfig_Valid              --- PASS
=== RUN   TestLoadConfig_FileNotFound       --- PASS
=== RUN   TestLoadConfig_Malformed          --- PASS
=== RUN   TestConfigPath_Default            --- PASS
=== RUN   TestConfigPath_XDG                --- PASS
=== RUN   TestParseSpotifyURL               --- PASS
    ... 10 subtests all PASS
PASS
ok   internal/adapters/spotify 0.009s
```

- **Total tests written:** 15
- **Total tests passing:** 15
- **Layers used:** Unit (15)
- **Pure functions created:** 2 (`parseSpotifyURL`, `ConfigPath`, `LoadConfig` — `ConfigPath` has minimal impure I/O for `os.Getenv`/`os.UserHomeDir`)

## Verification

```
$ go vet ./...
# clean — no output

$ go test ./... -count=1
# All 9 packages pass, zero regressions
```

## Deviations from Design

| Item | Design says | Implemented |
| ------ | ------------- | ------------- |
| Config format | JSON (`config.json`) | TOML (`config.toml`) — as spec and tasks require |
| Config path | `~/.config/music-dl/config.json` | `~/.config/music-dl/config.toml` |
| URL error messages | `domain.Error{Code: ErrorInvalidURL}` | Simple `errors.New` (pure function, no domain deps yet) |

## Remaining Tasks (not in this PR)

- Batch 3: Auth — Client Credentials Flow (PR 2)
- Batch 4: Spotify API Client (PR 2)
- Batch 5: YouTube Resolution via yt-dlp (PR 3)
- Batch 6: Full Search Integration (PR 3)
- Batch 7: TUI Source Selection (PR 4)
- Batch 8: Wiring in main.go (PR 4)
- Batch 9: Tests (parallel, PR 4)

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
| ------ | ----------- | ------- | ------------ | ----- | ------- | ------------- | ---------- |
| 1.1 | N/A (structural) | N/A | N/A | N/A | N/A | N/A | N/A |
| 1.2 | `config_test.go` | Unit | N/A (new) | ✅ Written | ✅ Passed | ✅ 5 cases | ✅ Removed unused `errors` import |
| 1.3 | `config_test.go` | Unit | N/A (new) | ✅ Written | ✅ Passed | ✅ 5 cases | ➖ None needed |
| 2.1 | `url_test.go` | Unit | N/A (new) | ✅ Written | ✅ Passed | ✅ 10 cases | ➖ None needed |
| 2.2 | `url_test.go` | Unit | N/A (new) | ✅ Written | ✅ Passed | ✅ 10 cases | ➖ None needed |

## Workload / PR Boundary

- **PR 1 complete** — ~200 lines touched across 2 new packages + go.mod update
- **Chained PRs recommended:** Yes (next: PR 2 — Auth + Spotify API Client)
- **400-line budget risk for full change:** High (remaining ~900 lines)
