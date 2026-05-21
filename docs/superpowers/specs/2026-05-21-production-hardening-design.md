# osctrld Production Hardening

**Date:** 2026-05-21
**Status:** Approved
**Goal:** Fix bugs, improve test coverage, migrate logging to slog, add CI pipeline.

## 1. Bug Fixes

### 1.1 Force Flag Binding

In `cmd/osctrld/main.go`, the `--force` CLI flag has its `Destination` set to `&jsonConfig.Verbose` instead of `&jsonConfig.Force`. This means `--force` silently overwrites the verbose setting and never actually enables force mode.

**Fix:** Change the `Destination` to `&jsonConfig.Force`.
**Regression test:** Verify that parsing `--force` sets `Force=true` and leaves `Verbose` unchanged.

### 1.2 Typo in HTTP Utils

In `cmd/osctrld/http-utils.go`, the error message reads "Cound not prepare request". Change to "Could not prepare request".

### 1.3 Dead Code in runScript

In `cmd/osctrld/actions_helpers.go`, `runScript()` calls `cmd.CombinedOutput()` (which executes the command and captures output), then immediately calls `cmd.Run()` (which tries to execute the same command again, failing because the process already ran). Remove the `cmd.CombinedOutput()` call — `cmd.Run()` is the intended execution path.

## 2. Logging Migration

### 2.1 Approach

Replace all `log.Printf`/`log.Println` calls with `log/slog` equivalents. No new dependencies.

### 2.2 Level Mapping

| Current pattern | slog level |
|---|---|
| Success messages (`log.Println("✅ ...")`) | `slog.Info` |
| Error messages (`log.Printf("❌ ...")`) | `slog.Error` |
| Verbose-gated messages (`if jsonConfig.Verbose { log.Printf(...) }`) | `slog.Debug` |
| General informational messages | `slog.Info` |

### 2.3 Handler Configuration

- Use `slog.NewTextHandler` (human-readable output).
- Verbose flag controls the minimum level: `slog.LevelDebug` when verbose, `slog.LevelInfo` otherwise.
- Initialize the logger once in `cliWrapper` after configuration is loaded.
- Remove emoji prefixes from all log messages. Use structured key-value fields for context instead (e.g., `slog.String("path", flagFile)`).

### 2.4 Conditional Verbose Removal

Remove all `if jsonConfig.Verbose { ... }` guards around log calls. Replace with `slog.Debug(...)` calls — the handler's level filter replaces the manual gating.

## 3. Tests

### 3.1 Fix Failing Test

`TestLoadConfigurationValid` in `cmd/osctrld/config_test.go` references `tests/osctrld-test.json` as a relative path. This breaks when tests run from the repository root (`go test ./...`).

**Fix:** Use `runtime.Caller` or `os.Getwd` to resolve the path relative to the test file's location, or use `testing.T.TempDir()` with an inline test config.

### 3.2 Action Tests

Add tests for the five CLI actions in a new file `cmd/osctrld/actions_test.go`:

- `enroll` — Mock osctrl API with `httptest.Server`, verify it retrieves and processes the enrollment script.
- `remove` — Same pattern, verify removal script handling.
- `verify` — Verify flag/cert/secret checking logic against mock API responses.
- `getFlags` — Verify flag retrieval and file writing.
- `getCert` — Verify certificate retrieval and file writing.

Each test uses `httptest.NewServer` to simulate osctrl API responses (200 with expected body, non-200 errors, connection failures).

### 3.3 Helper Tests

Add tests for untested helpers in `cmd/osctrld/actions_helpers_test.go`:

- `writeContentExists` — Test file creation, overwrite-with-force, refuse-without-force, directory creation.
- `getOsqueryVersion` — Test version extraction (mock or skip if osquery not installed).
- `retrieveFromURL` — Test via httptest server with various response codes and bodies.

### 3.4 Regression Test

Add a test in `cmd/osctrld/main_test.go` that verifies the `--force` flag correctly sets `jsonConfig.Force` and does not affect `jsonConfig.Verbose`.

### 3.5 Coverage Target

Aim for ~60-70% statement coverage organically. No hard gate — the goal is covering the core logic paths that matter, not hitting a number.

## 4. CI Pipeline

### 4.1 Workflow File

New file `.github/workflows/ci.yml`, triggered on:
- Pull requests to `main`
- Pushes to `main`

### 4.2 Steps

1. Checkout repository
2. Setup Go 1.24 with module caching
3. Run `golangci-lint` (latest stable version via `golangci/golangci-lint-action`)
4. Run `go test ./... -race -coverprofile=coverage.out`
5. Report coverage summary in workflow output

### 4.3 Linter Configuration

New file `.golangci.yml` at repository root with these linters enabled:

- `govet` — Suspicious constructs
- `errcheck` — Unchecked errors
- `staticcheck` — Static analysis
- `unused` — Unused code
- `gosimple` — Simplifications
- `ineffassign` — Ineffective assignments
- `typecheck` — Type checking

No aggressive or opinionated linters (no `gofumpt`, `wsl`, `nlreturn`). Keep it pragmatic.

## 5. Scope Boundaries

**Not included in this work:**

- New features (health checks, metrics, log rotation, daemon mode improvements)
- New dependencies
- Windows service configuration
- Package layout restructuring
- Contributor documentation or comprehensive godoc comments
- Coverage enforcement gates in CI
