# Osquery Lifecycle Management for osctrld

**Date:** 2026-05-22
**Status:** Approved
**Goal:** When the daemon syncs new flags or cert from osctrl and detects a change, automatically restart osquery so the new configuration takes effect.

## 1. Change Detection

Modify `writeContentExists` to return `(bool, error)` instead of `error`:

```go
func writeContentExists(path, content, name string, force bool) (bool, error)
```

The bool returns `true` when content was actually written to disk:
- File didn't exist → write → return `true`
- File existed with different content + force=true → overwrite → return `true`
- File existed with same content → skip → return `false`
- File existed with different content + force=false → return `false, error`

All callers (`getFlags`, `getCert`) must be updated. `getFlags` and `getCert` must propagate the bool upward, changing their signatures from `error` to `(bool, error)`.

## 2. Propagating Change Signals

**`getFlags` and `getCert`** return `(bool, error)` where the bool indicates whether the file was actually updated on disk.

**`syncOnce`** collects both bools and triggers a restart if either is `true`:

```
flagsChanged, flagsErr := getFlags(c)
certChanged, certErr := getCert(c)
if flagsChanged || certChanged {
    restartOsquery()
}
```

Errors in individual syncs are logged but do not prevent the other sync or the restart check.

## 3. Restart Mechanism

A new `restartOsquery()` function uses the OS service manager:

| OS | Command |
|----|---------|
| Linux | `systemctl restart osqueryd` |
| macOS | `launchctl kickstart -k system/io.osquery.agent` |

Uses `runtime.GOOS` to select the correct command. Returns an error if the command fails.

## 4. Restart Timing

Osquery is restarted **once** after both flags and cert syncs complete, not per-file. This avoids restarting osquery twice when both files change in the same tick.

## 5. Error Handling

If the restart command fails:
- Log the error at `Error` level
- Continue to the next tick
- No retry logic — the next tick will detect the same changes and try again

This is consistent with how sync failures are handled.

## 6. Platform Support

Linux and macOS only. Windows support is deferred to a separate feature. The `restartOsquery` function returns an error with a clear message on unsupported platforms.

## 7. One-Shot Commands

The one-shot `flags` and `cert` commands are updated to handle the new `(bool, error)` return from `writeContentExists` but ignore the bool. They do not trigger osquery restarts — restarts only happen in daemon (`service`) mode.

## 8. Files Changed

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `cmd/osctrld/actions_helpers.go` | Change `writeContentExists` return type |
| Modify | `cmd/osctrld/actions.go` | Update `getFlags`/`getCert` to return `(bool, error)` |
| Modify | `cmd/osctrld/service.go` | Update `syncOnce` to use change bools and call `restartOsquery` |
| Create | `cmd/osctrld/osquery.go` | `restartOsquery()` function |
| Modify | `cmd/osctrld/actions_helpers_test.go` | Update `writeContentExists` tests for `(bool, error)` |
| Create | `cmd/osctrld/osquery_test.go` | Tests for `restartOsquery` |

## 9. Tests

- **Update existing `writeContentExists` tests**: Verify the bool return value in all four cases (new file, same content, different+force, different+no-force).
- **Update existing `getFlags`/`getCert` action tests**: Verify they return `(bool, error)`.
- **New `restartOsquery` test**: Verify correct command is built per OS (mock exec, don't actually restart).
- **No integration test for the full sync→restart flow** — the daemon loop is not unit-testable without real timers.

## 10. Scope Boundaries

**Not included:**
- Windows support
- Restart cooldown/debounce
- Osquery health check after restart
- Configurable restart command path
- Restart on enrollment secret changes
