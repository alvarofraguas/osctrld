# Osquery Lifecycle Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When the daemon detects that synced flags or cert changed on disk, automatically restart osquery via the OS service manager.

**Architecture:** Modify `writeContentExists` to return `(bool, error)`, propagate the change signal through `getFlags`/`getCert` → `syncOnce`, and add a `restartOsquery()` function that calls `systemctl restart osqueryd` (Linux) or `launchctl kickstart -k system/io.osquery.agent` (macOS).

**Tech Stack:** Go 1.24, github.com/rs/zerolog, github.com/urfave/cli/v2, os/exec, runtime

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `cmd/osctrld/actions_helpers.go` | Change `writeContentExists` to return `(bool, error)` |
| Modify | `cmd/osctrld/actions_helpers_test.go` | Update tests for new return type |
| Modify | `cmd/osctrld/actions.go` | Update `getFlags`/`getCert` to return `(bool, error)` |
| Modify | `cmd/osctrld/actions_test.go` | Update action tests for new return type |
| Create | `cmd/osctrld/osquery.go` | `restartOsquery()` function |
| Create | `cmd/osctrld/osquery_test.go` | Tests for `restartOsquery` |
| Modify | `cmd/osctrld/service.go` | Update `syncOnce` to detect changes and call `restartOsquery` |

---

### Task 1: Change `writeContentExists` to return `(bool, error)`

**Files:**
- Modify: `cmd/osctrld/actions_helpers.go`
- Modify: `cmd/osctrld/actions_helpers_test.go`

- [ ] **Step 1: Update tests to expect `(bool, error)`**

In `cmd/osctrld/actions_helpers_test.go`, update all four `TestWriteContentExists_*` tests:

```go
func TestWriteContentExists_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "newfile.txt")

	changed, err := writeContentExists(path, "hello", "test", false)
	assert.NoError(t, err)
	assert.True(t, changed, "new file should report changed")

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))
}

func TestWriteContentExists_SameContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("hello"), 0700)

	changed, err := writeContentExists(path, "hello", "test", false)
	assert.NoError(t, err)
	assert.False(t, changed, "same content should not report changed")
}

func TestWriteContentExists_DifferentContentNoForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("old"), 0700)

	changed, err := writeContentExists(path, "new", "test", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "please use --force")
	assert.False(t, changed, "should not report changed on error")

	content, _ := os.ReadFile(path)
	assert.Equal(t, "old", string(content))
}

func TestWriteContentExists_DifferentContentWithForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("old"), 0700)

	changed, err := writeContentExists(path, "new", "test", true)
	assert.NoError(t, err)
	assert.True(t, changed, "forced overwrite should report changed")

	content, _ := os.ReadFile(path)
	assert.Equal(t, "new", string(content))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestWriteContentExists -v -count=1`
Expected: FAIL — `writeContentExists` returns 1 value, tests expect 2.

- [ ] **Step 3: Update `writeContentExists` implementation**

In `cmd/osctrld/actions_helpers.go`, change the function:

```go
func writeContentExists(path, content, name string, force bool) (bool, error) {
	if checkFileExist(path) {
		if !checkFileContent(path, content) {
			if force {
				if err := os.WriteFile(path, []byte(content), 0700); err != nil {
					return false, fmt.Errorf("error overwriting %s to %s - %v", name, path, err)
				}
				return true, nil
			}
			return false, fmt.Errorf("%s exists, please use --force to overwrite", path)
		}
		return false, nil
	}
	if err := os.WriteFile(path, []byte(content), 0700); err != nil {
		return false, fmt.Errorf("error writing %s to %s - %v", name, path, err)
	}
	return true, nil
}
```

- [ ] **Step 4: Run `writeContentExists` tests to verify they pass**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestWriteContentExists -v -count=1`
Expected: PASS — all 4 tests pass.

- [ ] **Step 5: Verify it compiles (will fail — callers not updated yet)**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/ 2>&1 || echo "Expected: callers not updated yet"`
Expected: Compile error in `actions.go` — callers still assign to single `err`.

- [ ] **Step 6: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/actions_helpers.go cmd/osctrld/actions_helpers_test.go
git commit -m "feat: change writeContentExists to return (bool, error) for change detection"
```

---

### Task 2: Update `getFlags` and `getCert` to return `(bool, error)`

**Files:**
- Modify: `cmd/osctrld/actions.go`
- Modify: `cmd/osctrld/actions_test.go`

- [ ] **Step 1: Update action tests for new return types**

In `cmd/osctrld/actions_test.go`, update the four getFlags/getCert tests:

```go
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
	changed, err := getFlags(c)
	assert.NoError(t, err)
	assert.True(t, changed, "new flags file should report changed")

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
	_, err := getFlags(c)
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
	changed, err := getCert(c)
	assert.NoError(t, err)
	assert.True(t, changed, "new cert file should report changed")

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
	_, err := getCert(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error retrieving cert")
}
```

- [ ] **Step 2: Update `getFlags` in `actions.go`**

Change `getFlags` to return `(bool, error)`:

```go
func getFlags(c *cli.Context) (bool, error) {
	log.Debug().Str("url", osctrlURLs.Flags).Msg("getting flags")
	flags, err := retrieveFlags(jsonConfig.Secret, jsonConfig.SecretFile, jsonConfig.CertFile)
	if err != nil {
		return false, fmt.Errorf("error retrieving flags - %v", err)
	}
	log.Debug().Str("flags", flags).Msg("flags content")
	changed, err := writeContentExists(jsonConfig.FlagFile, flags, "flags", jsonConfig.Force)
	if err != nil {
		return false, err
	}
	log.Info().Str("path", jsonConfig.FlagFile).Msg("flags ready")
	return changed, nil
}
```

- [ ] **Step 3: Update `getCert` in `actions.go`**

Change `getCert` to return `(bool, error)`:

```go
func getCert(c *cli.Context) (bool, error) {
	log.Debug().Str("url", osctrlURLs.Cert).Msg("getting cert")
	cert, err := retrieveCert(jsonConfig.Secret, osctrlURLs.Cert, jsonConfig.Insecure)
	if err != nil {
		return false, fmt.Errorf("error retrieving cert - %v", err)
	}
	log.Debug().Str("cert", cert).Msg("cert content")
	changed, err := writeContentExists(jsonConfig.CertFile, cert, "cert", jsonConfig.Force)
	if err != nil {
		return false, err
	}
	log.Info().Str("path", jsonConfig.CertFile).Msg("cert ready")
	return changed, nil
}
```

- [ ] **Step 4: Update `syncOnce` in `service.go` to handle new return types**

The existing `syncOnce` calls `getFlags(c)` and `getCert(c)` assigning to `err`. Update to discard the bool for now (the restart integration comes in Task 4):

```go
func syncOnce(c *cli.Context) {
	originalForce := jsonConfig.Force
	jsonConfig.Force = true
	defer func() { jsonConfig.Force = originalForce }()

	log.Info().Msg("syncing flags")
	if _, err := getFlags(c); err != nil {
		log.Error().Err(err).Msg("failed to sync flags")
	}
	log.Info().Msg("syncing cert")
	if _, err := getCert(c); err != nil {
		log.Error().Err(err).Msg("failed to sync cert")
	}
}
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean compilation.

- [ ] **Step 6: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/actions.go cmd/osctrld/actions_test.go cmd/osctrld/service.go
git commit -m "feat: propagate change detection through getFlags and getCert"
```

---

### Task 3: Create `restartOsquery` function

**Files:**
- Create: `cmd/osctrld/osquery.go`
- Create: `cmd/osctrld/osquery_test.go`

- [ ] **Step 1: Write test for `restartOsquery`**

Create `cmd/osctrld/osquery_test.go`:

```go
package main

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRestartOsquery_Command(t *testing.T) {
	cmd, args := osqueryRestartCommand()
	switch runtime.GOOS {
	case "linux":
		assert.Equal(t, "systemctl", cmd)
		assert.Equal(t, []string{"restart", "osqueryd"}, args)
	case "darwin":
		assert.Equal(t, "launchctl", cmd)
		assert.Equal(t, []string{"kickstart", "-k", "system/io.osquery.agent"}, args)
	default:
		assert.Empty(t, cmd, "unsupported OS should return empty command")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestRestartOsquery -v -count=1`
Expected: FAIL — `osqueryRestartCommand` is not defined.

- [ ] **Step 3: Create `osquery.go`**

Create `cmd/osctrld/osquery.go`:

```go
package main

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/rs/zerolog/log"
)

func osqueryRestartCommand() (string, []string) {
	switch runtime.GOOS {
	case LinuxOS:
		return "systemctl", []string{"restart", "osqueryd"}
	case DarwinOS:
		return "launchctl", []string{"kickstart", "-k", "system/io.osquery.agent"}
	default:
		return "", nil
	}
}

func restartOsquery() error {
	cmd, args := osqueryRestartCommand()
	if cmd == "" {
		return fmt.Errorf("osquery restart not supported on %s", runtime.GOOS)
	}
	log.Info().Str("command", cmd).Strs("args", args).Msg("restarting osquery")
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart osquery: %v (output: %s)", err, string(out))
	}
	log.Info().Msg("osquery restarted successfully")
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestRestartOsquery -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/osquery.go cmd/osctrld/osquery_test.go
git commit -m "feat: add restartOsquery function for OS service manager integration"
```

---

### Task 4: Integrate restart into `syncOnce`

**Files:**
- Modify: `cmd/osctrld/service.go`

- [ ] **Step 1: Update `syncOnce` to detect changes and trigger restart**

In `cmd/osctrld/service.go`, replace the `syncOnce` function:

```go
func syncOnce(c *cli.Context) {
	originalForce := jsonConfig.Force
	jsonConfig.Force = true
	defer func() { jsonConfig.Force = originalForce }()

	var flagsChanged, certChanged bool

	log.Info().Msg("syncing flags")
	changed, err := getFlags(c)
	if err != nil {
		log.Error().Err(err).Msg("failed to sync flags")
	} else {
		flagsChanged = changed
	}

	log.Info().Msg("syncing cert")
	changed, err = getCert(c)
	if err != nil {
		log.Error().Err(err).Msg("failed to sync cert")
	} else {
		certChanged = changed
	}

	if flagsChanged || certChanged {
		log.Info().Bool("flags_changed", flagsChanged).Bool("cert_changed", certChanged).Msg("configuration changed, restarting osquery")
		if err := restartOsquery(); err != nil {
			log.Error().Err(err).Msg("failed to restart osquery")
		}
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean compilation.

- [ ] **Step 3: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/service.go
git commit -m "feat: integrate osquery restart into daemon sync loop"
```

---

### Task 5: Final verification

**Files:**
- Verify: all changed files

- [ ] **Step 1: Build**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean build.

- [ ] **Step 2: Run all tests with race detector**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -race -v -count=1`
Expected: All tests pass.

- [ ] **Step 3: Run go mod tidy**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld && go mod tidy
```

Run: `git diff go.mod go.sum`
Expected: No changes (no new dependencies).

- [ ] **Step 4: Run linter (if available)**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && golangci-lint run ./cmd/osctrld/ 2>&1 || echo "linter not installed locally, CI will check"`
Expected: No new warnings, or linter not available.
