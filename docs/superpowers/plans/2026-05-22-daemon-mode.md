# Daemon Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `service` command that runs osctrld as a long-running daemon, syncing osquery flags and certificate from osctrl on a configurable interval with jitter and graceful shutdown.

**Architecture:** New `service.go` file contains the daemon loop (`serviceNode`), a jitter helper (`intervalWithJitter`), and a sync helper (`syncOnce`). The `service` command is registered in `main.go init()` alongside existing commands and wrapped with `cliWrapper` like all other commands. A new `--interval` / `-I` flag and `Interval` config field control the sync period.

**Tech Stack:** Go 1.24, github.com/rs/zerolog, github.com/urfave/cli/v2, math/rand/v2, os/signal, context

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `cmd/osctrld/config.go` | Add `Interval` field to `JSONConfiguration` |
| Modify | `cmd/osctrld/main.go` | Add `--interval` flag, register `service` command |
| Create | `cmd/osctrld/service.go` | `serviceNode`, `syncOnce`, `intervalWithJitter` |
| Create | `cmd/osctrld/service_test.go` | Tests for jitter and service command registration |
| Modify | `service/linux/systemd.service` | Fix `ExecStart` path |
| Modify | `service/darwin/net.osctrl.daemon.plist` | Update for long-running daemon |

---

### Task 1: Add Interval config field and CLI flag

**Files:**
- Modify: `cmd/osctrld/config.go`
- Modify: `cmd/osctrld/main.go`

- [ ] **Step 1: Add Interval field to JSONConfiguration**

In `cmd/osctrld/config.go`, add `Interval` after the `LogFormat` field:

```go
type JSONConfiguration struct {
	Secret       string `json:"secret"`
	SecretFile   string `json:"secretFile"`
	FlagFile     string `json:"flags"`
	CertFile     string `json:"cert"`
	EnrollScript string `json:"enrollScript"`
	RemoveScript string `json:"removeScript"`
	OsqueryPath  string `json:"osquery"`
	Environment  string `json:"environment"`
	BaseURL      string `json:"baseurl"`
	Insecure     bool   `json:"insecure"`
	Verbose      bool   `json:"verbose"`
	Force        bool   `json:"force"`
	LogFormat    string `json:"logFormat"`
	Interval     int    `json:"interval"`
}
```

- [ ] **Step 2: Add --interval CLI flag**

In `cmd/osctrld/main.go`, inside `init()`, add this flag after the `log-format` StringFlag (before the closing `}` of the `flags` slice):

```go
&cli.IntFlag{
	Name:        "interval",
	Aliases:     []string{"I"},
	Value:       60,
	Usage:       "Sync interval in minutes for service mode",
	EnvVars:     []string{"OSCTRL_INTERVAL"},
	Destination: &jsonConfig.Interval,
},
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean compilation.

- [ ] **Step 4: Run tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All existing tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/config.go cmd/osctrld/main.go
git commit -m "feat: add --interval flag and Interval config field"
```

---

### Task 2: Create service.go with jitter helper and tests

**Files:**
- Create: `cmd/osctrld/service.go`
- Create: `cmd/osctrld/service_test.go`

- [ ] **Step 1: Write the jitter test**

Create `cmd/osctrld/service_test.go`:

```go
package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIntervalWithJitter(t *testing.T) {
	base := 60 * time.Minute
	min := 54 * time.Minute // base - 10%
	max := 66 * time.Minute // base + 10%

	for i := 0; i < 100; i++ {
		result := intervalWithJitter(base)
		assert.GreaterOrEqual(t, result, min, "jitter should not go below -10%%")
		assert.LessOrEqual(t, result, max, "jitter should not go above +10%%")
	}
}

func TestIntervalWithJitter_SmallInterval(t *testing.T) {
	base := 1 * time.Minute
	min := 54 * time.Second  // 1min - 10%
	max := 66 * time.Second  // 1min + 10%

	for i := 0; i < 100; i++ {
		result := intervalWithJitter(base)
		assert.GreaterOrEqual(t, result, min)
		assert.LessOrEqual(t, result, max)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestIntervalWithJitter -v -count=1`
Expected: FAIL — `intervalWithJitter` is not defined.

- [ ] **Step 3: Create service.go with the jitter helper**

Create `cmd/osctrld/service.go`:

```go
package main

import (
	"math/rand/v2"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func intervalWithJitter(base time.Duration) time.Duration {
	jitterRange := float64(base) * 0.2
	jitter := rand.Float64()*jitterRange - jitterRange/2
	return base + time.Duration(jitter)
}

func serviceNode(c *cli.Context) error {
	return nil
}
```

- [ ] **Step 4: Run jitter tests to verify they pass**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestIntervalWithJitter -v -count=1`
Expected: PASS — both tests pass.

- [ ] **Step 5: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All tests pass (existing + 2 new jitter tests).

- [ ] **Step 6: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/service.go cmd/osctrld/service_test.go
git commit -m "feat: add service.go with jitter helper and tests"
```

---

### Task 3: Register service command and add registration test

**Files:**
- Modify: `cmd/osctrld/main.go`
- Modify: `cmd/osctrld/service_test.go`

- [ ] **Step 1: Write the command registration test**

Append to `cmd/osctrld/service_test.go`:

```go
func TestServiceCommandRegistered(t *testing.T) {
	app := buildApp()
	found := false
	for _, cmd := range app.Commands {
		if cmd.Name == "service" {
			found = true
			assert.Equal(t, "Run as a daemon, periodically syncing flags and certificate", cmd.Usage)
			break
		}
	}
	assert.True(t, found, "service command should be registered")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestServiceCommandRegistered -v -count=1`
Expected: FAIL — no `service` command found.

- [ ] **Step 3: Register the service command in init()**

In `cmd/osctrld/main.go`, add this entry at the end of the `commands` slice (after the `cert` command):

```go
{
	Name:   "service",
	Usage:  "Run as a daemon, periodically syncing flags and certificate",
	Action: cliWrapper(serviceNode),
},
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestServiceCommandRegistered -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/main.go cmd/osctrld/service_test.go
git commit -m "feat: register service command in CLI"
```

---

### Task 4: Implement the syncOnce helper

**Files:**
- Modify: `cmd/osctrld/service.go`

- [ ] **Step 1: Implement syncOnce**

In `cmd/osctrld/service.go`, add the `syncOnce` function. This function saves and restores the `Force` flag, calls the existing `getFlags` and `getCert` functions, and logs errors without returning them (the daemon must continue on failure):

```go
func syncOnce(c *cli.Context) {
	originalForce := jsonConfig.Force
	jsonConfig.Force = true
	defer func() { jsonConfig.Force = originalForce }()

	log.Info().Msg("syncing flags")
	if err := getFlags(c); err != nil {
		log.Error().Err(err).Msg("failed to sync flags")
	}
	log.Info().Msg("syncing cert")
	if err := getCert(c); err != nil {
		log.Error().Err(err).Msg("failed to sync cert")
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean compilation.

- [ ] **Step 3: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/service.go
git commit -m "feat: add syncOnce helper for daemon sync loop"
```

---

### Task 5: Implement the serviceNode daemon loop

**Files:**
- Modify: `cmd/osctrld/service.go`

- [ ] **Step 1: Update imports in service.go**

Replace the import block in `cmd/osctrld/service.go` with:

```go
import (
	"context"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)
```

- [ ] **Step 2: Implement serviceNode**

Replace the stub `serviceNode` function with the full implementation:

```go
func serviceNode(c *cli.Context) error {
	interval := time.Duration(jsonConfig.Interval) * time.Minute
	log.Info().Int("interval_minutes", jsonConfig.Interval).Msg("starting service")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	syncOnce(c)

	for {
		wait := intervalWithJitter(interval)
		log.Debug().Dur("next_sync", wait).Msg("waiting for next sync")

		select {
		case <-time.After(wait):
			syncOnce(c)
		case <-ctx.Done():
			log.Info().Msg("shutting down")
			return nil
		}
	}
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean compilation.

- [ ] **Step 4: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/service.go
git commit -m "feat: implement serviceNode daemon loop with graceful shutdown"
```

---

### Task 6: Update service files

**Files:**
- Modify: `service/linux/systemd.service`
- Modify: `service/darwin/net.osctrl.daemon.plist`

- [ ] **Step 1: Fix systemd ExecStart**

In `service/linux/systemd.service`, change the `ExecStart` line:

Old: `ExecStart=/opt/osctrld/service --config=/etc/osctrld/service.json`
New: `ExecStart=/opt/osctrld/osctrld service --config=/etc/osctrld/service.json`

- [ ] **Step 2: Update launchd plist**

Replace the contents of `service/darwin/net.osctrl.daemon.plist` with:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>KeepAlive</key>
  <true/>
  <key>Disabled</key>
  <false/>
  <key>Label</key>
  <string>net.osctrl.daemon</string>
  <key>ProgramArguments</key>
  <array>
    <string>/path/to/osctrld</string>
    <string>service</string>
    <string>--config=/path/to/osctrld.json</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
</dict>
</plist>
```

Changes from the original:
- Added `service` as second program argument
- Removed `ThrottleInterval` (the daemon manages its own interval)

- [ ] **Step 3: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add service/linux/systemd.service service/darwin/net.osctrl.daemon.plist
git commit -m "fix: update service files for daemon mode"
```

---

### Task 7: Final verification

**Files:**
- Verify: all changed files

- [ ] **Step 1: Build**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean build.

- [ ] **Step 2: Run all tests with race detector**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -race -v -count=1`
Expected: All tests pass (existing 39 + 3 new = 42 total).

- [ ] **Step 3: Verify --interval flag in help**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go run ./cmd/osctrld/ --help 2>&1 | grep -A1 'interval'`
Expected: `--interval value, -I value   Sync interval in minutes for service mode (default: 60)`

- [ ] **Step 4: Verify service command in help**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go run ./cmd/osctrld/ --help 2>&1 | grep 'service'`
Expected: `service   Run as a daemon, periodically syncing flags and certificate`

- [ ] **Step 5: Run go mod tidy**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld && go mod tidy
```

Run: `git diff go.mod go.sum`
Expected: No changes (no new dependencies).

- [ ] **Step 6: Run linter (if available)**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && golangci-lint run ./cmd/osctrld/ 2>&1 || echo "linter not installed locally, CI will check"`
Expected: No new warnings, or linter not available (CI handles it).
