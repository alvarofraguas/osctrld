# Zerolog Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `log/slog` with `github.com/rs/zerolog` for a smaller runtime fingerprint, and add a `--log-format` flag to choose between human-readable text and JSON log output.

**Architecture:** Single-pass migration — add zerolog dependency, add the new CLI flag and config field, rewrite logger initialization in `cliWrapper`, then mechanically replace every `slog.*` and `log.*` call site across 5 source files. No behavioral changes; existing tests pass with only import adjustments.

**Tech Stack:** Go 1.24, github.com/rs/zerolog, github.com/urfave/cli/v2

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `go.mod` / `go.sum` | Add zerolog dependency |
| Modify | `cmd/osctrld/config.go` | Add `LogFormat` field to `JSONConfiguration` |
| Modify | `cmd/osctrld/main.go` | Add `--log-format` flag, rewrite logger init in `cliWrapper`, migrate `cliAction` and `main()` fatal calls |
| Modify | `cmd/osctrld/actions.go` | Migrate all slog calls to zerolog |
| Modify | `cmd/osctrld/actions_helpers.go` | Migrate 3 slog call sites to zerolog |
| Modify | `cmd/osctrld/http-utils.go` | Migrate 1 slog call site to zerolog |
| Verify | `cmd/osctrld/*_test.go` | No code changes needed — tests don't assert on log output |

---

### Task 1: Add zerolog dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add zerolog module**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld && go get github.com/rs/zerolog
```

- [ ] **Step 2: Verify dependency added**

Run: `grep zerolog go.mod`
Expected: `github.com/rs/zerolog v1.x.x` appears in the require block

- [ ] **Step 3: Run go mod tidy**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld && go mod tidy
```

- [ ] **Step 4: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add go.mod go.sum
git commit -m "deps: add github.com/rs/zerolog dependency"
```

---

### Task 2: Add LogFormat config field and CLI flag

**Files:**
- Modify: `cmd/osctrld/config.go` — Add `LogFormat` field to `JSONConfiguration`
- Modify: `cmd/osctrld/main.go` — Add `--log-format` / `-L` CLI flag in `init()`

- [ ] **Step 1: Add LogFormat field to JSONConfiguration**

In `cmd/osctrld/config.go`, add `LogFormat` after the `Force` field in the `JSONConfiguration` struct:

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
}
```

- [ ] **Step 2: Add the --log-format CLI flag**

In `cmd/osctrld/main.go`, inside the `init()` function, add this flag entry after the `force` flag (before the closing `}` of the `flags` slice):

```go
&cli.StringFlag{
	Name:        "log-format",
	Aliases:     []string{"L"},
	Value:       "text",
	Usage:       "Log output format: text or json",
	EnvVars:     []string{"OSCTRL_LOG_FORMAT"},
	Destination: &jsonConfig.LogFormat,
},
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean compilation, no errors.

- [ ] **Step 4: Run tests to make sure nothing broke**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All 39 tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/config.go cmd/osctrld/main.go
git commit -m "feat: add --log-format flag and LogFormat config field"
```

---

### Task 3: Migrate logger initialization in cliWrapper and fatal calls

**Files:**
- Modify: `cmd/osctrld/main.go` — Replace slog logger init with zerolog, replace all slog/log calls

This is the core task. The entire `cmd/osctrld/main.go` file gets its imports and logging calls migrated.

- [ ] **Step 1: Update imports**

Replace the import block in `cmd/osctrld/main.go`. Remove `"log"` and `"log/slog"`, add zerolog imports:

Old imports:
```go
import (
	"log"
	"log/slog"
	"os"
	"runtime"

	"github.com/urfave/cli/v2"
)
```

New imports:
```go
import (
	"os"
	"runtime"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)
```

- [ ] **Step 2: Replace logger initialization in cliWrapper**

In the `cliWrapper` function, replace the slog logger setup block (lines 205–213 in the current file) with zerolog initialization.

Old code (remove this):
```go
		logLevel := slog.LevelInfo
		if jsonConfig.Verbose {
			logLevel = slog.LevelDebug
		}
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		}))
		slog.SetDefault(logger)
		slog.Debug("initializing", "app", appName)
```

New code (replace with this):
```go
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		if jsonConfig.Verbose {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		}
		if jsonConfig.LogFormat == "json" {
			log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
		} else {
			log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
		}
		log.Debug().Str("app", appName).Msg("initializing")
```

- [ ] **Step 3: Migrate slog calls in cliWrapper**

Replace each remaining slog call in cliWrapper:

**Error reading config** (around line 202):
Old: `slog.Error("error reading configuration file", "path", configFile, "error", err)`
New: `log.Error().Str("path", configFile).Err(err).Msg("error reading configuration file")`

**Environment required** (around line 276):
Old: `slog.Error("environment for osctrl is required")`
New: `log.Error().Msg("environment for osctrl is required")`

**Base URL required** (around line 280):
Old: `slog.Error("base URL for osctrl is required")`
New: `log.Error().Msg("base URL for osctrl is required")`

**Configuration loaded debug** (around line 285):
Old:
```go
		slog.Debug("configuration loaded",
			"osquery_path", jsonConfig.OsqueryPath,
			"flag_file", jsonConfig.FlagFile,
			"secret_file", jsonConfig.SecretFile,
			"cert_file", jsonConfig.CertFile,
			"enroll_script", jsonConfig.EnrollScript,
			"remove_script", jsonConfig.RemoveScript,
			"base_url", jsonConfig.BaseURL,
			"environment", jsonConfig.Environment,
			"insecure", jsonConfig.Insecure,
			"verbose", jsonConfig.Verbose,
			"force", jsonConfig.Force,
			"command", c.Command.Name,
		)
```
New:
```go
		log.Debug().
			Str("osquery_path", jsonConfig.OsqueryPath).
			Str("flag_file", jsonConfig.FlagFile).
			Str("secret_file", jsonConfig.SecretFile).
			Str("cert_file", jsonConfig.CertFile).
			Str("enroll_script", jsonConfig.EnrollScript).
			Str("remove_script", jsonConfig.RemoveScript).
			Str("base_url", jsonConfig.BaseURL).
			Str("environment", jsonConfig.Environment).
			Bool("insecure", jsonConfig.Insecure).
			Bool("verbose", jsonConfig.Verbose).
			Bool("force", jsonConfig.Force).
			Str("command", c.Command.Name).
			Msg("configuration loaded")
```

- [ ] **Step 4: Migrate cliAction fatal calls and slog calls**

In the `cliAction` function, replace:

Old (two occurrences of `log.Fatalf`):
```go
log.Fatalf("Error with help - %s", err)
```
New (both occurrences):
```go
log.Fatal().Err(err).Msg("error showing help")
```

Old: `slog.Error("no command provided")`
New: `log.Error().Msg("no command provided")`

Old: `slog.Error("invalid command")`
New: `log.Error().Msg("invalid command")`

- [ ] **Step 5: Migrate main() fatal call**

In the `main()` function:

Old:
```go
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Failed to execute %v", err)
	}
```
New:
```go
	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("failed to execute")
	}
```

- [ ] **Step 6: Verify it compiles**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Compilation fails because actions.go, actions_helpers.go, config.go, and http-utils.go still import `"log/slog"`. This is expected — we migrate those in the next tasks.

- [ ] **Step 7: Commit (without building — partial migration)**

We cannot compile yet, but we commit main.go's migration separately for clean history:

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/main.go
git commit -m "refactor: migrate main.go from slog to zerolog"
```

---

### Task 4: Migrate actions.go to zerolog

**Files:**
- Modify: `cmd/osctrld/actions.go` — Replace all slog calls with zerolog equivalents

- [ ] **Step 1: Update imports**

Old imports:
```go
import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/urfave/cli/v2"
)
```

New imports:
```go
import (
	"fmt"
	"runtime"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/urfave/cli/v2"
)
```

- [ ] **Step 2: Migrate enrollNode (1 call)**

Old: `slog.Debug("enrolling node", "url", osctrlURLs.Enroll)`
New: `log.Debug().Str("url", osctrlURLs.Enroll).Msg("enrolling node")`

- [ ] **Step 3: Migrate getFlags (3 calls)**

Old: `slog.Debug("getting flags", "url", osctrlURLs.Flags)`
New: `log.Debug().Str("url", osctrlURLs.Flags).Msg("getting flags")`

Old: `slog.Debug("flags content", "flags", flags)`
New: `log.Debug().Str("flags", flags).Msg("flags content")`

Old: `slog.Info("flags ready", "path", jsonConfig.FlagFile)`
New: `log.Info().Str("path", jsonConfig.FlagFile).Msg("flags ready")`

- [ ] **Step 4: Migrate getCert (3 calls)**

Old: `slog.Debug("getting cert", "url", osctrlURLs.Cert)`
New: `log.Debug().Str("url", osctrlURLs.Cert).Msg("getting cert")`

Old: `slog.Debug("cert content", "cert", cert)`
New: `log.Debug().Str("cert", cert).Msg("cert content")`

Old: `slog.Info("cert ready", "path", jsonConfig.CertFile)`
New: `log.Info().Str("path", jsonConfig.CertFile).Msg("cert ready")`

- [ ] **Step 5: Migrate removeNode (1 call)**

Old: `slog.Debug("removing node", "url", osctrlURLs.Remove)`
New: `log.Debug().Str("url", osctrlURLs.Remove).Msg("removing node")`

- [ ] **Step 6: Migrate verifyNode (all 16 calls)**

Replace every slog call in verifyNode. Here is the complete list:

```
slog.Debug("comparing secret", "path", jsonConfig.SecretFile)
→ log.Debug().Str("path", jsonConfig.SecretFile).Msg("comparing secret")

slog.Info("osquery secret is valid")
→ log.Info().Msg("osquery secret is valid")

slog.Warn("osquery secret mismatch")
→ log.Warn().Msg("osquery secret mismatch")

slog.Debug("retrieving verification", "url", osctrlURLs.Verify)
→ log.Debug().Str("url", osctrlURLs.Verify).Msg("retrieving verification")

slog.Debug("comparing flags", "path", jsonConfig.FlagFile)
→ log.Debug().Str("path", jsonConfig.FlagFile).Msg("comparing flags")

slog.Info("flags are valid")
→ log.Info().Msg("flags are valid")

slog.Warn("flags mismatch")
→ log.Warn().Msg("flags mismatch")

slog.Debug("comparing certificate", "path", jsonConfig.CertFile)
→ log.Debug().Str("path", jsonConfig.CertFile).Msg("comparing certificate")

slog.Info("osquery certificate is valid")
→ log.Info().Msg("osquery certificate is valid")

slog.Warn("osquery certificate mismatch")
→ log.Warn().Msg("osquery certificate mismatch")

slog.Debug("checking local file", "path", l)
→ log.Debug().Str("path", l).Msg("checking local file")

slog.Warn("local file missing", "path", l)
→ log.Warn().Str("path", l).Msg("local file missing")

slog.Info("osquery local files are present")
→ log.Info().Msg("osquery local files are present")

slog.Debug("expected osquery version", "version", verification.OsqueryVersion)
→ log.Debug().Str("version", verification.OsqueryVersion).Msg("expected osquery version")

slog.Debug("existing osquery version", "version", existingVersion)
→ log.Debug().Str("version", existingVersion).Msg("existing osquery version")

slog.Warn("osquery version too low", "existing", existingVersion, "required", verification.OsqueryVersion)
→ log.Warn().Str("existing", existingVersion).Str("required", verification.OsqueryVersion).Msg("osquery version too low")

slog.Info("osquery version is valid", "version", existingVersion)
→ log.Info().Str("version", existingVersion).Msg("osquery version is valid")

slog.Debug("checking running process")
→ log.Debug().Msg("checking running process")

slog.Info("osqueryd is running", "pid", osqueryPid)
→ log.Info().Int32("pid", osqueryPid).Msg("osqueryd is running")

slog.Warn("osqueryd is not running")
→ log.Warn().Msg("osqueryd is not running")

slog.Error("osquery is not installed")
→ log.Error().Msg("osquery is not installed")
```

Note: `osqueryPid` is `int32` (from `p.Pid`), so use `.Int32("pid", osqueryPid)`.

- [ ] **Step 7: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/actions.go
git commit -m "refactor: migrate actions.go from slog to zerolog"
```

---

### Task 5: Migrate actions_helpers.go to zerolog

**Files:**
- Modify: `cmd/osctrld/actions_helpers.go` — Replace 3 slog call sites

- [ ] **Step 1: Update imports**

Old imports:
```go
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)
```

New imports:
```go
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/rs/zerolog/log"
)
```

- [ ] **Step 2: Migrate checkFileContent (1 call)**

Old: `slog.Error("error opening file", "path", path, "error", err)`
New: `log.Error().Str("path", path).Err(err).Msg("error opening file")`

- [ ] **Step 3: Migrate getOsqueryVersion (1 call)**

Old: `slog.Error("error running osqueryd", "error", err, "output", string(out))`
New: `log.Error().Err(err).Str("output", string(out)).Msg("error running osqueryd")`

- [ ] **Step 4: Migrate runScript (1 call)**

Old: `slog.Warn("script generated warnings", "stderr", errOutput)`
New: `log.Warn().Str("stderr", errOutput).Msg("script generated warnings")`

- [ ] **Step 5: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/actions_helpers.go
git commit -m "refactor: migrate actions_helpers.go from slog to zerolog"
```

---

### Task 6: Migrate config.go to zerolog

**Files:**
- Modify: `cmd/osctrld/config.go` — Replace 1 slog call site

- [ ] **Step 1: Update imports**

Old imports:
```go
import (
	"log/slog"

	"github.com/spf13/viper"
)
```

New imports:
```go
import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)
```

- [ ] **Step 2: Migrate loadConfiguration (1 call)**

Old: `slog.Debug("loading configuration", "path", file)`
New: `log.Debug().Str("path", file).Msg("loading configuration")`

- [ ] **Step 3: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/config.go
git commit -m "refactor: migrate config.go from slog to zerolog"
```

---

### Task 7: Migrate http-utils.go to zerolog

**Files:**
- Modify: `cmd/osctrld/http-utils.go` — Replace 1 slog call site

- [ ] **Step 1: Update imports**

Old imports:
```go
import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)
```

New imports:
```go
import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/rs/zerolog/log"
)
```

- [ ] **Step 2: Migrate SendRequest (1 call)**

Old: `slog.Error("failed to close response body", "error", err)`
New: `log.Error().Err(err).Msg("failed to close response body")`

- [ ] **Step 3: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/http-utils.go
git commit -m "refactor: migrate http-utils.go from slog to zerolog"
```

---

### Task 8: Final verification

**Files:**
- Verify: all `cmd/osctrld/*.go` files

- [ ] **Step 1: Verify no slog imports remain**

Run: `grep -rn '"log/slog"' /Users/afraguas/CursorTest/osctrld/osctrld/cmd/osctrld/`
Expected: No output (no matches).

Run: `grep -rn 'slog\.' /Users/afraguas/CursorTest/osctrld/osctrld/cmd/osctrld/`
Expected: No output (no slog usage).

- [ ] **Step 2: Verify no stdlib log imports remain**

Run: `grep -rn '"log"' /Users/afraguas/CursorTest/osctrld/osctrld/cmd/osctrld/*.go`
Expected: No output. The only `log` import should be `"github.com/rs/zerolog/log"`.

- [ ] **Step 3: Build**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean build, no errors.

- [ ] **Step 4: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -race -v -count=1`
Expected: All 39 tests pass. No test asserts on log output, so switching the logging library should not cause any failures.

- [ ] **Step 5: Run linter**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && golangci-lint run ./cmd/osctrld/`
Expected: No new warnings or errors.

- [ ] **Step 6: Tidy modules**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld && go mod tidy
```

Run: `git diff go.mod go.sum`
Expected: Either no changes (already tidy) or minor cleanup. If changes exist:

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add go.mod go.sum
git commit -m "chore: go mod tidy after zerolog migration"
```

- [ ] **Step 7: Verify --log-format flag exists**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go run ./cmd/osctrld/ --help`
Expected: Output includes `--log-format value, -L value   Log output format: text or json (default: "text")` in the global flags section.
