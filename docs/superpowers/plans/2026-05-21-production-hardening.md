# osctrld Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix bugs, migrate logging to slog, add tests for core logic, and add a CI pipeline with linting.

**Architecture:** All changes are within the existing `cmd/osctrld/` package. No new packages, no new dependencies, no structural changes. Tests use `httptest.Server` to mock the osctrl API. Logging migrates from `log` to `log/slog` using the Go stdlib `TextHandler`.

**Tech Stack:** Go 1.24, log/slog (stdlib), net/http/httptest (stdlib), stretchr/testify, golangci-lint

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `cmd/osctrld/main.go` | Modify | Fix force flag binding, migrate logging to slog |
| `cmd/osctrld/http-utils.go` | Modify | Fix typo, migrate logging to slog |
| `cmd/osctrld/actions_helpers.go` | Modify | Fix dead code in runScript, migrate logging to slog |
| `cmd/osctrld/actions.go` | Modify | Migrate logging to slog |
| `cmd/osctrld/config.go` | Modify | Migrate logging to slog |
| `cmd/osctrld/config_test.go` | Modify | Fix failing test path |
| `cmd/osctrld/main_test.go` | Create | Force flag regression test |
| `cmd/osctrld/actions_test.go` | Create | Tests for enroll, remove, getFlags, getCert actions |
| `cmd/osctrld/actions_helpers_test.go` | Create | Tests for writeContentExists, retrieveFlags, retrieveScript, retrieveCert, retrieveVerify, genericRetrieve |
| `.golangci.yml` | Create | Linter configuration |
| `.github/workflows/ci.yml` | Create | CI pipeline for PRs |
| `Makefile` | Modify | Fix test targets to use `./cmd/osctrld/` path |

---

### Task 1: Fix the Force Flag Binding Bug

**Files:**
- Modify: `cmd/osctrld/main.go:162`
- Create: `cmd/osctrld/main_test.go`

- [ ] **Step 1: Write the regression test**

Create `cmd/osctrld/main_test.go`:

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForceFlagDoesNotAffectVerbose(t *testing.T) {
	// Reset global state
	jsonConfig = JSONConfiguration{}

	app := buildApp()
	err := app.Run([]string{"osctrld", "--force", "--environment", "dev", "--osctrl-url", "http://localhost", "flags"})

	// The command will fail (no server), but flag parsing should succeed
	assert.True(t, jsonConfig.Force, "expected Force to be true")
	assert.False(t, jsonConfig.Verbose, "expected Verbose to remain false")
}

func TestVerboseFlagDoesNotAffectForce(t *testing.T) {
	jsonConfig = JSONConfiguration{}

	app := buildApp()
	err := app.Run([]string{"osctrld", "--verbose", "--environment", "dev", "--osctrl-url", "http://localhost", "flags"})

	_ = err
	assert.True(t, jsonConfig.Verbose, "expected Verbose to be true")
	assert.False(t, jsonConfig.Force, "expected Force to remain false")
}
```

- [ ] **Step 2: Extract app builder from main.go to make it testable**

The `init()` function sets global vars that `main()` consumes. To test flag parsing, we need a `buildApp()` function. Add it to `cmd/osctrld/main.go` by extracting the app construction from `main()`:

Replace the current `main()` function:

```go
// buildApp creates and configures the CLI application
func buildApp() *cli.App {
	app := cli.NewApp()
	app.Name = appName
	app.Usage = appUsage
	app.Version = appVersion
	app.Description = appDescription
	app.Flags = flags
	app.Commands = commands
	app.Action = cliAction
	return app
}

// Go go!
func main() {
	app := buildApp()
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Failed to execute %v", err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails (force flag bug still present)**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestForceFlagDoesNotAffectVerbose -v`

Expected: FAIL — `Force` is `false`, `Verbose` is `true` (because `Destination` points to `Verbose`)

- [ ] **Step 4: Fix the force flag binding**

In `cmd/osctrld/main.go`, line 162, change:

```go
Destination: &jsonConfig.Verbose,
```

to:

```go
Destination: &jsonConfig.Force,
```

- [ ] **Step 5: Run tests to verify both pass**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run "TestForceFlagDoesNotAffectVerbose|TestVerboseFlagDoesNotAffectForce" -v`

Expected: Both PASS

- [ ] **Step 6: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/main.go cmd/osctrld/main_test.go
git commit -m "fix: force flag destination pointed to Verbose instead of Force

Add regression tests verifying --force and --verbose flags bind
to their correct config fields independently."
```

---

### Task 2: Fix Typo and Dead Code

**Files:**
- Modify: `cmd/osctrld/http-utils.go:57`
- Modify: `cmd/osctrld/actions_helpers.go:180`

- [ ] **Step 1: Fix the typo in http-utils.go**

In `cmd/osctrld/http-utils.go`, line 57, change:

```go
return 0, []byte("Cound not prepare request"), err
```

to:

```go
return 0, []byte("Could not prepare request"), err
```

- [ ] **Step 2: Fix the dead code in runScript**

In `cmd/osctrld/actions_helpers.go`, the `runScript` function (lines 154-201), remove the premature `cmd.CombinedOutput()` call at line 180. Replace lines 178-185 with:

```go
	// Execute the script
	cmd := exec.Command(tmpFile.Name())

	// Set the command's output to the buffers
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
```

This removes `cmd.CombinedOutput()` which was executing the command and discarding the result before `cmd.Run()` tried (and failed) to execute it again.

- [ ] **Step 3: Run existing tests to verify nothing broke**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v`

Expected: All existing passing tests still pass

- [ ] **Step 4: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/http-utils.go cmd/osctrld/actions_helpers.go
git commit -m "fix: typo in error message and dead code in runScript

Fix 'Cound' -> 'Could' in http-utils.go.
Remove premature cmd.CombinedOutput() in runScript that executed the
script and discarded the result before cmd.Run() tried to run it again."
```

---

### Task 3: Fix Failing Config Test

**Files:**
- Modify: `cmd/osctrld/config_test.go`

- [ ] **Step 1: Run the existing config test to confirm it fails**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestLoadConfigurationValid -v`

Expected: FAIL — cannot find `tests/osctrld-test.json` (relative path doesn't resolve from the `cmd/osctrld/` directory)

- [ ] **Step 2: Rewrite config_test.go to use inline test data**

Replace the entire contents of `cmd/osctrld/config_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigurationInvalid(t *testing.T) {
	_, err := loadConfiguration("nonexistent-file.json", false)
	assert.Error(t, err)
}

func TestLoadConfigurationValid(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "osctrld-test.json")
	configData := []byte(`{
  "osctrld": {
    "secret": "test-secret",
    "secretFile": "/tmp/osquery.secret",
    "flags": "/tmp/osquery.flags",
    "cert": "/tmp/osctrl.crt",
    "environment": "dev",
    "baseurl": "https://localhost:9000",
    "insecure": true,
    "verbose": true,
    "force": true
  }
}`)
	err := os.WriteFile(configPath, configData, 0644)
	assert.NoError(t, err)

	cfg, err := loadConfiguration(configPath, false)
	assert.NoError(t, err)
	assert.Equal(t, "test-secret", cfg.Secret)
	assert.Equal(t, "dev", cfg.Environment)
	assert.Equal(t, "https://localhost:9000", cfg.BaseURL)
	assert.True(t, cfg.Insecure)
	assert.True(t, cfg.Verbose)
	assert.True(t, cfg.Force)
}
```

- [ ] **Step 3: Run to verify it passes**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestLoadConfiguration -v`

Expected: Both tests PASS

- [ ] **Step 4: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/config_test.go
git commit -m "fix: config test uses inline temp file instead of relative path

TestLoadConfigurationValid was failing because it referenced
tests/osctrld-test.json with a relative path that didn't resolve
when running from the repository root."
```

---

### Task 4: Migrate Logging to slog

**Files:**
- Modify: `cmd/osctrld/main.go`
- Modify: `cmd/osctrld/actions.go`
- Modify: `cmd/osctrld/actions_helpers.go`
- Modify: `cmd/osctrld/config.go`
- Modify: `cmd/osctrld/http-utils.go`

This task touches every file that imports `log`. The migration is mechanical: replace `log.Printf`/`log.Println` with `slog.Info`/`slog.Error`/`slog.Debug`, remove emoji prefixes, remove `if jsonConfig.Verbose` guards (slog's level filter replaces them), and initialize the slog handler once.

- [ ] **Step 1: Initialize slog in cliWrapper (main.go)**

In `cmd/osctrld/main.go`, add `"log/slog"` and `"os"` to the imports (keep `"log"` for `main()`'s `log.Fatalf`). In the `cliWrapper` function, add logger initialization right after the configuration loading block (after the `configFile` if-block, before the OS switch), replacing the existing verbose init log:

Replace lines 205-208:

```go
		if jsonConfig.Verbose {
			log.Printf("⏳ Initializing %s...", appName)
			fmt.Println()
		}
```

with:

```go
		// Initialize structured logger
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

- [ ] **Step 2: Migrate cliWrapper verbose logging (main.go)**

Replace the verbose config dump block (lines 280-294):

```go
		if jsonConfig.Verbose {
			log.Printf("📌 Osquery Path: %s", jsonConfig.OsqueryPath)
			log.Printf("🔎 Flag file: %s", jsonConfig.FlagFile)
			log.Printf("🔑 Secret file: %s", jsonConfig.SecretFile)
			log.Printf("🔏 Certificate: %s", jsonConfig.CertFile)
			log.Printf("+ Enroll script: %s", jsonConfig.EnrollScript)
			log.Printf("- Remove script: %s", jsonConfig.RemoveScript)
			log.Printf("🔗 BaseURL: %s", jsonConfig.BaseURL)
			log.Printf("📍 Environment: %s", jsonConfig.Environment)
			log.Printf("🔴 Insecure: %v", jsonConfig.Insecure)
			log.Printf("📢 Verbose: %v", jsonConfig.Verbose)
			log.Printf("🦾 Force: %v", jsonConfig.Force)
			log.Printf("💻 Command: %s", c.Command.Name)
			fmt.Println()
		}
```

with:

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

- [ ] **Step 3: Migrate error messages in cliWrapper (main.go)**

Replace the three error-exit messages in `cliWrapper`:

Line 201-203 — config error:
```go
				exitError := fmt.Sprintf("\n❌ Error reading configuration file (%s) - %v", configFile, err)
				return cli.Exit(exitError, 2)
```
becomes:
```go
				slog.Error("error reading configuration file", "path", configFile, "error", err)
				return cli.Exit("", 2)
```

Line 271-272 — missing environment:
```go
			exitError := fmt.Sprintln("\n❌ Environment for osctrl is required")
			return cli.Exit(exitError, 2)
```
becomes:
```go
			slog.Error("environment for osctrl is required")
			return cli.Exit("", 2)
```

Line 275-276 — missing base URL:
```go
			exitError := fmt.Sprintln("\n❌ Base URL for osctrl is required")
			return cli.Exit(exitError, 2)
```
becomes:
```go
			slog.Error("base URL for osctrl is required")
			return cli.Exit("", 2)
```

- [ ] **Step 4: Migrate cliAction (main.go)**

Replace:
```go
		return cli.Exit("❌ No command provided", 2)
```
with:
```go
		slog.Error("no command provided")
		return cli.Exit("", 2)
```

Replace:
```go
		return cli.Exit("❌ Invalid command", 2)
```
with:
```go
		slog.Error("invalid command")
		return cli.Exit("", 2)
```

- [ ] **Step 5: Migrate actions.go**

Add `"log/slog"` to imports, remove `"log"`.

`enrollNode` — replace:
```go
	if jsonConfig.Verbose {
		log.Printf("Enrolling node in %s", osctrlURLs.Enroll)
	}
```
with:
```go
	slog.Debug("enrolling node", "url", osctrlURLs.Enroll)
```

`getFlags` — replace:
```go
	if jsonConfig.Verbose {
		log.Printf("Getting flags from %s", osctrlURLs.Flags)
	}
```
with:
```go
	slog.Debug("getting flags", "url", osctrlURLs.Flags)
```

Replace:
```go
	if jsonConfig.Verbose {
		fmt.Println(flags)
	}
```
with:
```go
	slog.Debug("flags content", "flags", flags)
```

Replace:
```go
	log.Printf("✅ flags ready in %s", jsonConfig.FlagFile)
```
with:
```go
	slog.Info("flags ready", "path", jsonConfig.FlagFile)
```

`getCert` — replace:
```go
	if jsonConfig.Verbose {
		log.Printf("Getting cert from %s", osctrlURLs.Cert)
	}
```
with:
```go
	slog.Debug("getting cert", "url", osctrlURLs.Cert)
```

Replace:
```go
	if jsonConfig.Verbose {
		fmt.Println(cert)
	}
```
with:
```go
	slog.Debug("cert content", "cert", cert)
```

Replace:
```go
	log.Printf("✅ cert ready in %s", jsonConfig.CertFile)
```
with:
```go
	slog.Info("cert ready", "path", jsonConfig.CertFile)
```

`removeNode` — replace:
```go
	if jsonConfig.Verbose {
		log.Printf("Removing node in %s", osctrlURLs.Remove)
	}
```
with:
```go
	slog.Debug("removing node", "url", osctrlURLs.Remove)
```

`verifyNode` — replace all `log.Printf`/`log.Println` calls:

```go
	if jsonConfig.Verbose {
		log.Printf("Comparing secret with %s", jsonConfig.SecretFile)
	}
```
→ `slog.Debug("comparing secret", "path", jsonConfig.SecretFile)`

```go
	log.Println("✅ osquery secret is valid")
```
→ `slog.Info("osquery secret is valid")`

```go
	log.Printf("❌ osquery secret mismatch")
```
→ `slog.Warn("osquery secret mismatch")`

```go
	if jsonConfig.Verbose {
		log.Printf("Retrieving verification from %s", osctrlURLs.Verify)
	}
```
→ `slog.Debug("retrieving verification", "url", osctrlURLs.Verify)`

```go
	if jsonConfig.Verbose {
		log.Printf("Comparing flags with %s", jsonConfig.FlagFile)
	}
```
→ `slog.Debug("comparing flags", "path", jsonConfig.FlagFile)`

```go
	log.Println("✅ flags are valid")
```
→ `slog.Info("flags are valid")`

```go
	log.Printf("❌ flags mismatch")
```
→ `slog.Warn("flags mismatch")`

```go
	if jsonConfig.Verbose {
		log.Printf("Comparing certificate with %s", jsonConfig.CertFile)
	}
```
→ `slog.Debug("comparing certificate", "path", jsonConfig.CertFile)`

```go
	log.Println("✅ osquery certificate is valid")
```
→ `slog.Info("osquery certificate is valid")`

```go
	log.Printf("❌ osquery certificate mismatch")
```
→ `slog.Warn("osquery certificate mismatch")`

```go
	if jsonConfig.Verbose {
		log.Printf("Checking %s", l)
	}
```
→ `slog.Debug("checking local file", "path", l)`

```go
	log.Printf("❌ %s is missing", l)
```
→ `slog.Warn("local file missing", "path", l)`

```go
	log.Println("✅ osquery local files are present")
```
→ `slog.Info("osquery local files are present")`

```go
	if jsonConfig.Verbose {
		log.Printf("Expecting osquery %s or higher", verification.OsqueryVersion)
	}
```
→ `slog.Debug("expected osquery version", "version", verification.OsqueryVersion)`

```go
	if jsonConfig.Verbose {
		log.Printf("Existing version is %s", existingVersion)
	}
```
→ `slog.Debug("existing osquery version", "version", existingVersion)`

```go
	log.Printf("❌ osquery version (%s) is lower than required (%s)", existingVersion, verification.OsqueryVersion)
```
→ `slog.Warn("osquery version too low", "existing", existingVersion, "required", verification.OsqueryVersion)`

```go
	log.Printf("✅ osquery version (%s) is valid", existingVersion)
```
→ `slog.Info("osquery version is valid", "version", existingVersion)`

```go
	if jsonConfig.Verbose {
		log.Println("Checking running process")
	}
```
→ `slog.Debug("checking running process")`

```go
	log.Printf("✅ osqueryd is running (pid %d)", osqueryPid)
```
→ `slog.Info("osqueryd is running", "pid", osqueryPid)`

```go
	log.Printf("❌ osqueryd is NOT running")
```
→ `slog.Warn("osqueryd is not running")`

```go
	log.Printf("❌ please install osquery")
```
→ `slog.Error("osquery is not installed")`

Also remove all `fmt.Println()` separator lines in `verifyNode` (there are 4 of them after log messages). Structured logging makes separators unnecessary.

- [ ] **Step 6: Migrate actions_helpers.go**

Add `"log/slog"` to imports, remove `"log"`.

`checkFileContent` — replace:
```go
		log.Printf("error opening %s - %v", path, err)
```
with:
```go
		slog.Error("error opening file", "path", path, "error", err)
```

`getOsqueryVersion` — replace:
```go
		log.Printf("error running osqueryd - %v - %s", err, string(out))
```
with:
```go
		slog.Error("error running osqueryd", "error", err, "output", string(out))
```

`runScript` — replace:
```go
		log.Printf("script generated warnings: %s", errOutput)
```
with:
```go
		slog.Warn("script generated warnings", "stderr", errOutput)
```

- [ ] **Step 7: Migrate config.go**

Add `"log/slog"` to imports, remove `"log"`.

Replace:
```go
	if verbose {
		log.Printf("Loading %s", file)
	}
```
with:
```go
	slog.Debug("loading configuration", "path", file)
```

- [ ] **Step 8: Migrate http-utils.go**

Add `"log/slog"` to imports, remove `"log"`.

Replace:
```go
			log.Printf("Failed to close body %v", err)
```
with:
```go
			slog.Error("failed to close response body", "error", err)
```

- [ ] **Step 9: Clean up imports — remove unused "fmt" where applicable**

After the migration, check each file for unused `"fmt"` imports. In `actions.go`, the `fmt.Println()` calls are removed, but `fmt.Printf` and `fmt.Errorf` remain, so `"fmt"` stays. Verify with the compiler.

- [ ] **Step 10: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v`

Expected: All tests pass. The `captureOutput` helper in `http-utils_test.go` redirects `log.SetOutput` — since `http-utils.go` no longer uses `log`, this helper is dead code. Remove `captureOutput` from `http-utils_test.go` if it's unused (check if any test calls it).

- [ ] **Step 11: Build to verify compilation**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`

Expected: Compiles without errors

- [ ] **Step 12: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/main.go cmd/osctrld/actions.go cmd/osctrld/actions_helpers.go cmd/osctrld/config.go cmd/osctrld/http-utils.go cmd/osctrld/http-utils_test.go
git commit -m "refactor: migrate logging from stdlib log to log/slog

Replace log.Printf/Println with slog.Info/Warn/Error/Debug.
Remove emoji prefixes, use structured key-value fields instead.
Remove manual verbose guards — slog level filter handles this.
Initialize TextHandler in cliWrapper with level based on --verbose flag."
```

---

### Task 5: Add Helper Tests

**Files:**
- Create: `cmd/osctrld/actions_helpers_test.go`

- [ ] **Step 1: Write tests for writeContentExists**

Create `cmd/osctrld/actions_helpers_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteContentExists_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "newfile.txt")

	err := writeContentExists(path, "hello", "test", false)
	assert.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))
}

func TestWriteContentExists_SameContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("hello"), 0700)

	err := writeContentExists(path, "hello", "test", false)
	assert.NoError(t, err)
}

func TestWriteContentExists_DifferentContentNoForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("old"), 0700)

	err := writeContentExists(path, "new", "test", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "please use --force")

	content, _ := os.ReadFile(path)
	assert.Equal(t, "old", string(content))
}

func TestWriteContentExists_DifferentContentWithForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("old"), 0700)

	err := writeContentExists(path, "new", "test", true)
	assert.NoError(t, err)

	content, _ := os.ReadFile(path)
	assert.Equal(t, "new", string(content))
}
```

- [ ] **Step 2: Run to verify they pass**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestWriteContentExists -v`

Expected: All 4 tests PASS

- [ ] **Step 3: Write tests for genericRetrieve and retrieve helpers**

Append to `cmd/osctrld/actions_helpers_test.go`:

```go
func mockOsctrlServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/env/osctrld-flags", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("--tls_hostname=osctrl.example.com\n--tls_server_certs=/etc/osquery/osctrl.crt"))
	})
	mux.HandleFunc("/env/osctrld-cert", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("-----BEGIN CERTIFICATE-----\nMIIBxTCCAWugAwIBAgIJALP...\n-----END CERTIFICATE-----"))
	})
	mux.HandleFunc("/env/osctrld-verify", func(w http.ResponseWriter, r *http.Request) {
		resp := VerifyResponse{
			Flags:          "--tls_hostname=osctrl.example.com",
			Certificate:    "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
			OsqueryVersion: "5.0.0",
		}
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/env/enroll/darwin/osctrld-script", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("#!/bin/bash\necho enroll"))
	})
	mux.HandleFunc("/env/remove/darwin/osctrld-script", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("#!/bin/bash\necho remove"))
	})
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})
	return httptest.NewServer(mux)
}

func TestGenericRetrieve_Success(t *testing.T) {
	server := mockOsctrlServer()
	defer server.Close()

	data := ScriptRequest{Secret: "test-secret"}
	body, err := genericRetrieve(server.URL+"/env/enroll/darwin/osctrld-script", false, data)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "echo enroll")
}

func TestGenericRetrieve_ServerError(t *testing.T) {
	server := mockOsctrlServer()
	defer server.Close()

	data := ScriptRequest{Secret: "test-secret"}
	_, err := genericRetrieve(server.URL+"/error", false, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestGenericRetrieve_ConnectionRefused(t *testing.T) {
	data := ScriptRequest{Secret: "test-secret"}
	_, err := genericRetrieve("http://127.0.0.1:1/unreachable", false, data)
	assert.Error(t, err)
}

func TestRetrieveScript(t *testing.T) {
	server := mockOsctrlServer()
	defer server.Close()

	script, err := retrieveScript("test-secret", server.URL+"/env/enroll/darwin/osctrld-script", false)
	assert.NoError(t, err)
	assert.Contains(t, script, "echo enroll")
}

func TestRetrieveCert(t *testing.T) {
	server := mockOsctrlServer()
	defer server.Close()

	cert, err := retrieveCert("test-secret", server.URL+"/env/osctrld-cert", false)
	assert.NoError(t, err)
	assert.Contains(t, cert, "BEGIN CERTIFICATE")
}

func TestRetrieveVerify(t *testing.T) {
	server := mockOsctrlServer()
	defer server.Close()

	v, err := retrieveVerify("secret", "/tmp/secret", "/tmp/cert", server.URL+"/env/osctrld-verify", false)
	assert.NoError(t, err)
	assert.Equal(t, "5.0.0", v.OsqueryVersion)
	assert.Contains(t, v.Flags, "tls_hostname")
}

func TestRetrieveFlags(t *testing.T) {
	server := mockOsctrlServer()
	defer server.Close()

	// retrieveFlags uses the global osctrlURLs, so set it
	osctrlURLs.Flags = server.URL + "/env/osctrld-flags"
	jsonConfig.Insecure = false

	flags, err := retrieveFlags("test-secret", "/tmp/secret", "/tmp/cert")
	assert.NoError(t, err)
	assert.Contains(t, flags, "tls_hostname")
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run "TestGenericRetrieve|TestRetrieveScript|TestRetrieveCert|TestRetrieveVerify|TestRetrieveFlags" -v`

Expected: All PASS

- [ ] **Step 5: Write tests for checkFileExist and checkFileContent**

Append to `cmd/osctrld/actions_helpers_test.go`:

```go
func TestCheckFileExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	os.WriteFile(path, []byte("data"), 0644)

	assert.True(t, checkFileExist(path))
	assert.False(t, checkFileExist(filepath.Join(dir, "nope.txt")))
}

func TestCheckFileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "content.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	assert.True(t, checkFileContent(path, "hello world"))
	assert.False(t, checkFileContent(path, "different"))
	assert.False(t, checkFileContent(filepath.Join(dir, "nope.txt"), "anything"))
}

func TestCheckFileContent_Whitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ws.txt")
	os.WriteFile(path, []byte("  hello  \n"), 0644)

	assert.True(t, checkFileContent(path, "hello"))
}
```

- [ ] **Step 6: Run to verify they pass**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run "TestCheckFile" -v`

Expected: All PASS

- [ ] **Step 7: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/actions_helpers_test.go
git commit -m "test: add tests for action helpers

Cover writeContentExists (create, same-content skip, no-force reject,
force overwrite), genericRetrieve (success, server error, connection
refused), retrieveScript, retrieveCert, retrieveVerify, retrieveFlags,
checkFileExist, and checkFileContent."
```

---

### Task 6: Add Action Tests

**Files:**
- Create: `cmd/osctrld/actions_test.go`

- [ ] **Step 1: Write tests for getFlags and getCert actions**

Create `cmd/osctrld/actions_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func setupTestConfig(t *testing.T, server *httptest.Server) (cleanup func()) {
	dir := t.TempDir()

	jsonConfig = JSONConfiguration{
		Secret:       "test-secret",
		SecretFile:   filepath.Join(dir, "osquery.secret"),
		FlagFile:     filepath.Join(dir, "osquery.flags"),
		CertFile:     filepath.Join(dir, "osctrl.crt"),
		OsqueryPath:  dir,
		Environment:  "env",
		BaseURL:      server.URL,
		Insecure:     false,
		Verbose:      false,
		Force:        true,
		EnrollScript: filepath.Join(dir, "osctrld-enroll.sh"),
		RemoveScript: filepath.Join(dir, "osctrld-remove.sh"),
	}
	osctrlURLs = genURLs(server.URL, "env", false)

	return func() {
		jsonConfig = JSONConfiguration{}
		osctrlURLs = OsctrlURLs{}
	}
}

func newTestCLIContext() *cli.Context {
	app := cli.NewApp()
	return cli.NewContext(app, nil, nil)
}

func TestGetFlags_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("--tls_hostname=osctrl.example.com"))
	}))
	defer server.Close()

	cleanup := setupTestConfig(t, server)
	defer cleanup()
	osctrlURLs.Flags = server.URL + "/flags"

	c := newTestCLIContext()
	err := getFlags(c)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonConfig.FlagFile)
	require.NoError(t, err)
	assert.Equal(t, "--tls_hostname=osctrl.example.com", string(content))
}

func TestGetFlags_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	cleanup := setupTestConfig(t, server)
	defer cleanup()
	osctrlURLs.Flags = server.URL + "/flags"

	c := newTestCLIContext()
	err := getFlags(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error retrieving flags")
}

func TestGetCert_Success(t *testing.T) {
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIBtest\n-----END CERTIFICATE-----"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(certPEM))
	}))
	defer server.Close()

	cleanup := setupTestConfig(t, server)
	defer cleanup()
	osctrlURLs.Cert = server.URL + "/cert"

	c := newTestCLIContext()
	err := getCert(c)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonConfig.CertFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "BEGIN CERTIFICATE")
}

func TestGetCert_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer server.Close()

	cleanup := setupTestConfig(t, server)
	defer cleanup()
	osctrlURLs.Cert = server.URL + "/cert"

	c := newTestCLIContext()
	err := getCert(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error retrieving cert")
}
```

- [ ] **Step 2: Run to verify they pass**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run "TestGetFlags|TestGetCert" -v`

Expected: All 4 tests PASS

- [ ] **Step 3: Write tests for enrollNode and removeNode**

Append to `cmd/osctrld/actions_test.go`:

```go
func TestEnrollNode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("#!/bin/bash\necho enrolled"))
	}))
	defer server.Close()

	cleanup := setupTestConfig(t, server)
	defer cleanup()
	osctrlURLs.Enroll = server.URL + "/enroll"

	c := newTestCLIContext()
	err := enrollNode(c)
	assert.NoError(t, err)
}

func TestEnrollNode_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer server.Close()

	cleanup := setupTestConfig(t, server)
	defer cleanup()
	osctrlURLs.Enroll = server.URL + "/enroll"

	c := newTestCLIContext()
	err := enrollNode(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error retrieving enroll")
}

func TestRemoveNode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("#!/bin/bash\necho removed"))
	}))
	defer server.Close()

	cleanup := setupTestConfig(t, server)
	defer cleanup()
	osctrlURLs.Remove = server.URL + "/remove"

	c := newTestCLIContext()
	err := removeNode(c)
	assert.NoError(t, err)
}

func TestRemoveNode_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer server.Close()

	cleanup := setupTestConfig(t, server)
	defer cleanup()
	osctrlURLs.Remove = server.URL + "/remove"

	c := newTestCLIContext()
	err := removeNode(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error retrieving remove")
}
```

- [ ] **Step 4: Write test for verifyNode**

Append to `cmd/osctrld/actions_test.go`:

```go
func TestVerifyNode_Success(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "osquery.secret")
	flagPath := filepath.Join(dir, "osquery.flags")
	certPath := filepath.Join(dir, "osctrl.crt")

	flagContent := "--tls_hostname=osctrl.example.com\n--tls_server_certs=/etc/osquery/osctrl.crt"
	certContent := "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"

	os.WriteFile(secretPath, []byte("test-secret"), 0644)
	os.WriteFile(flagPath, []byte(flagContent), 0644)
	os.WriteFile(certPath, []byte(certContent), 0644)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := VerifyResponse{
			Flags:          flagContent,
			Certificate:    certContent,
			OsqueryVersion: "5.0.0",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	jsonConfig = JSONConfiguration{
		Secret:      "test-secret",
		SecretFile:  secretPath,
		FlagFile:    flagPath,
		CertFile:    certPath,
		OsqueryPath: dir,
		Environment: "env",
		BaseURL:     server.URL,
		Verbose:     false,
	}
	osctrlURLs.Verify = server.URL + "/verify"

	c := newTestCLIContext()
	err := verifyNode(c)
	assert.NoError(t, err)
}
```

- [ ] **Step 5: Run all action tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run "TestEnrollNode|TestRemoveNode|TestGetFlags|TestGetCert|TestVerifyNode" -v`

Expected: All 9 tests PASS

- [ ] **Step 6: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/actions_test.go
git commit -m "test: add tests for CLI action functions

Cover getFlags, getCert, enrollNode, removeNode, and verifyNode
using httptest servers to mock the osctrl API. Tests cover both
success and server error paths."
```

---

### Task 7: Fix Makefile Test Targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Update Makefile test targets**

The current `test` and `test_cover` targets run `go test . -v` which doesn't match the package location. Update them:

Replace:
```makefile
# Run all tests
test:
	go test . -v

# Check test coverage
test_cover:
	go test -cover .
```

with:

```makefile
# Run all tests
test:
	go test ./cmd/osctrld/ -v

# Check test coverage
test_cover:
	go test -cover ./cmd/osctrld/
```

- [ ] **Step 2: Verify**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && make test`

Expected: All tests run and pass

- [ ] **Step 3: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add Makefile
git commit -m "fix: Makefile test targets use correct package path"
```

---

### Task 8: Add CI Pipeline and Linter Config

**Files:**
- Create: `.golangci.yml`
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create golangci-lint configuration**

Create `.golangci.yml` at repository root:

```yaml
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck

linters-settings:
  errcheck:
    check-type-assertions: true

issues:
  exclude-use-default: true
```

- [ ] **Step 2: Create CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches:
      - main
  pull_request:
    branches:
      - main

permissions:
  contents: read

jobs:
  lint-and-test:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ">=1.24.3"
          cache: true

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

      - name: Run tests
        run: go test ./cmd/osctrld/ -race -coverprofile=coverage.out -v

      - name: Coverage summary
        run: go tool cover -func=coverage.out
```

- [ ] **Step 3: Run linter locally to check for issues**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go vet ./cmd/osctrld/`

Expected: No issues (go vet is the core of govet linter). If golangci-lint is installed locally, also run `golangci-lint run ./cmd/osctrld/` and fix any issues before committing.

- [ ] **Step 4: Run full test suite one final time**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -race -v`

Expected: All tests pass with race detector enabled

- [ ] **Step 5: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add .golangci.yml .github/workflows/ci.yml
git commit -m "ci: add golangci-lint and test pipeline for PRs

Runs govet, errcheck, staticcheck, unused, gosimple, ineffassign,
and typecheck on every PR and push to main. Tests run with -race
and coverage reporting."
```

---

### Task 9: Final Verification

- [ ] **Step 1: Run full test suite with coverage**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -race -cover -v`

Expected: All tests pass, coverage should be in the 50-70% range.

- [ ] **Step 2: Verify build**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build -o /dev/null ./cmd/osctrld/`

Expected: Clean build, no warnings

- [ ] **Step 3: Review git log**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && git log --oneline -10`

Expected: 8 new commits (design spec, force flag fix, typo/dead code fix, config test fix, slog migration, helper tests, action tests, Makefile fix, CI pipeline)
