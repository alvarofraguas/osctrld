# Daemon Mode for osctrld

**Date:** 2026-05-22
**Status:** Approved
**Goal:** Add a `service` command that runs osctrld as a long-running daemon, periodically syncing osquery flags and certificate from osctrl.

## 1. New `service` Command

Register a new CLI command:

```
osctrld service [global options]
```

The `service` command runs forever (until signalled). It performs an initial sync immediately on startup, then repeats on a configurable interval with jitter.

## 2. Sync Behavior

Each sync tick does two things in order:

1. **Fetch flags** — calls the existing `getFlags()` logic (HTTP POST to osctrl flags endpoint, write result to the flags file)
2. **Fetch cert** — calls the existing `getCert()` logic (HTTP POST to osctrl cert endpoint, write result to the cert file)

Both operations use the `--force` flag behavior — the daemon always runs with force=true so that changed files get overwritten without prompting.

The enrollment secret is **not** synced. It is set once during enrollment and does not change.

## 3. Interval and Jitter

**New CLI flag:**

| Flag | Aliases | Default | Type | Env Var |
|---|---|---|---|---|
| `--interval` | `-I` | `60` | int (minutes) | `OSCTRL_INTERVAL` |

**New field in `JSONConfiguration`:**
```go
Interval int `json:"interval"`
```

**Jitter:** Each tick adds random jitter of +/- 10% to the interval. For a 60-minute interval, each tick fires between 54 and 66 minutes. This prevents thundering herd when many endpoints restart simultaneously.

The jitter is recalculated on each tick (not fixed at startup).

## 4. Error Handling

If either sync operation fails (network error, server error, file write error):

- Log the error at `Error` level
- Continue to the next tick
- No retry logic — the next scheduled tick will attempt again

This is acceptable because:
- Flags and certs on disk remain valid until successfully overwritten
- At 60-minute intervals, transient failures self-heal quickly
- No risk of hammering a struggling server with retries

## 5. Graceful Shutdown

The daemon catches `SIGINT` and `SIGTERM` using `signal.NotifyContext` to create a cancellable context.

On signal:
1. Log "shutting down" at `Info` level
2. Stop the ticker
3. If a sync is in-flight, let it finish (the context cancels the *wait*, not the sync)
4. Exit with code 0

## 6. Startup Behavior

On startup, the `service` command:

1. Logs "starting service" with the configured interval
2. Runs one immediate sync (flags + cert)
3. Starts the ticker loop

If the initial sync fails, the daemon logs the error and continues into the ticker loop (it does not exit on first failure).

## 7. Force Flag Override

The `service` command always syncs with `force=true` regardless of the `--force` CLI flag setting. A daemon that stops syncing because a file already exists would be silently broken. The `--force` flag remains relevant only for one-shot commands (`flags`, `cert`).

## 8. New File

Create `cmd/osctrld/service.go` containing:

- `serviceNode(c *cli.Context) error` — the action function registered as the `service` command
- Ticker loop logic with jitter
- `signal.NotifyContext` setup

This keeps the daemon logic isolated from the existing one-shot action functions.

## 9. Service File Updates

**`service/linux/systemd.service`:**
- Change `ExecStart` from `/opt/osctrld/service` to `/opt/osctrld/osctrld service`
- The binary name is `osctrld`, and `service` is a subcommand

**`service/darwin/net.osctrl.daemon.plist`:**
- Add `service` as a program argument (after `osctrld` binary path and `--config`)
- Remove `ThrottleInterval` since the daemon manages its own interval
- Keep `KeepAlive=true` for crash recovery

## 10. Tests

- **Unit test for jitter calculation:** Verify jitter stays within +/- 10% of the base interval.
- **Unit test for service command registration:** Verify the `service` command exists in the CLI app.
- **No integration test for the ticker loop itself** — testing a long-running loop with real timers is fragile and low-value. The sync logic is already tested via the existing `getFlags`/`getCert` tests.

## 11. Scope Boundaries

**Not included:**
- Config hot-reload (watch config file for changes during runtime)
- Health check endpoint (HTTP server for liveness probes)
- PID file management
- Log file output / rotation (deferred to separate spec)
- osquery lifecycle management (restart osquery on config changes) — separate feature
- Extension deployment — separate feature
- Metrics / telemetry
