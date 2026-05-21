# Migrate from slog to zerolog

**Date:** 2026-05-21
**Status:** Approved
**Goal:** Replace log/slog with github.com/rs/zerolog for a smaller runtime fingerprint, and add configurable log output format (text vs JSON).

## 1. Dependency Change

Add `github.com/rs/zerolog` as a new dependency. Remove all `log/slog` usage from source files. Remove stdlib `log` usage from `main()` and `cliAction` (replace with zerolog's `log` package).

## 2. New CLI Flag

Add a `--log-format` / `-L` flag:

| Flag | Aliases | Default | Values | Env Var |
|---|---|---|---|---|
| `--log-format` | `-L` | `text` | `text`, `json` | `OSCTRL_LOG_FORMAT` |

New field in `JSONConfiguration`:
```go
LogFormat string `json:"logFormat"`
```

## 3. Logger Initialization

Initialize zerolog once in `cliWrapper`, after configuration is loaded (same location where slog is initialized today).

**Text mode** (`--log-format text`): Use `zerolog.ConsoleWriter` writing to `os.Stderr`. This produces human-readable colored output.

**JSON mode** (`--log-format json`): Use zerolog's default JSON encoder writing to `os.Stderr`.

**Level control**: `--verbose` sets `zerolog.DebugLevel`. Default is `zerolog.InfoLevel`. Set via `zerolog.SetGlobalLevel()`.

**Global logger**: Assign the configured logger to `log.Logger` (from `github.com/rs/zerolog/log`). All call sites use this global.

## 4. Call Site Migration

All 5 source files that currently use `slog` get migrated:

| slog pattern | zerolog equivalent |
|---|---|
| `slog.Info("msg", "k1", v1, "k2", v2)` | `log.Info().Str("k1", v1).Str("k2", v2).Msg("msg")` |
| `slog.Debug("msg", "k1", v1)` | `log.Debug().Str("k1", v1).Msg("msg")` |
| `slog.Error("msg", "error", err)` | `log.Error().Err(err).Msg("msg")` |
| `slog.Warn("msg", "k1", v1)` | `log.Warn().Str("k1", v1).Msg("msg")` |
| `slog.Error("msg")` (no fields) | `log.Error().Msg("msg")` |
| `slog.Debug("msg")` (no fields) | `log.Debug().Msg("msg")` |

For structured fields with non-string types:
- `bool` values → `.Bool("key", val)`
- `int32` values (pid) → `.Int32("pid", val)`
- `int` values → `.Int("key", val)`

**Files to modify:**
- `cmd/osctrld/main.go` — Logger init in `cliWrapper`, error exits in `cliWrapper` and `cliAction`, `log.Fatalf` in `main()`
- `cmd/osctrld/actions.go` — All action function logging
- `cmd/osctrld/actions_helpers.go` — 3 log call sites
- `cmd/osctrld/config.go` — 1 log call site
- `cmd/osctrld/http-utils.go` — 1 log call site

## 5. main() and cliAction Fatal Logging

The current `log.Fatalf` calls in `main()` and `cliAction` (stdlib `log`) switch to zerolog:

- `log.Fatalf("Failed to execute %v", err)` → `log.Fatal().Err(err).Msg("failed to execute")`
- `log.Fatalf("Error with help - %s", err)` → `log.Fatal().Err(err).Msg("error showing help")`

This removes the last stdlib `log` import from the codebase.

## 6. Tests

- Existing tests don't assert on log output, so they should pass with only import changes needed.
- No new tests required for the log format flag (it's a presentation concern, not logic).
- Verify all existing tests still pass after migration.

## 7. Scope Boundaries

**Not included:**
- Log file output (deferred to daemon mode spec)
- Log rotation
- Per-component log levels
- Timestamp format customization
