# Extension Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable the daemon to fetch osquery extension binaries from osctrl and deploy them to the local extensions directory, restarting osquery when extensions change.

**Architecture:** New `extensions.go` with manifest retrieval and binary download functions. Extends URL generation in `utils.go`, config in `config.go`, and daemon loop in `service.go`. Follows existing patterns for HTTP requests, file writing with change detection, and daemon sync integration.

**Tech Stack:** Go 1.24, github.com/rs/zerolog, github.com/urfave/cli/v2, encoding/json, net/http, os

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `cmd/osctrld/config.go` | Add `ExtensionsDir` field |
| Modify | `cmd/osctrld/main.go` | Add default extensions dir path in `cliWrapper` |
| Modify | `cmd/osctrld/utils.go` | Add extensions URL constant, generator, OsctrlURLs.Extensions |
| Modify | `cmd/osctrld/utils_test.go` | Test for extensions URL generation |
| Modify | `cmd/osctrld/actions.go` | Add `ExtensionEntry` and `ExtensionsRequest` types |
| Create | `cmd/osctrld/extensions.go` | `retrieveExtensionManifest`, `downloadExtension`, `syncExtensions` |
| Create | `cmd/osctrld/extensions_test.go` | Tests for extension functions |
| Modify | `cmd/osctrld/service.go` | Add extension sync to `syncOnce` |

---

### Task 1: Add ExtensionsDir config and extensions URL

**Files:**
- Modify: `cmd/osctrld/config.go`
- Modify: `cmd/osctrld/main.go`
- Modify: `cmd/osctrld/utils.go`
- Modify: `cmd/osctrld/utils_test.go`

- [ ] **Step 1: Add ExtensionsDir to JSONConfiguration**

In `cmd/osctrld/config.go`, add after the `Interval` field:

```go
ExtensionsDir string `json:"extensionsDir"`
```

- [ ] **Step 2: Add extensions URL constant and OsctrlURLs field**

In `cmd/osctrld/utils.go`, add constant after `OsctrlURLScript`:

```go
OsctrlURLExtensions = "%s/osctrld-extensions"
```

Add field to `OsctrlURLs` struct after `Remove`:

```go
Extensions string
```

Add generator function after `genRemoveURL`:

```go
func genExtensionsURL(osctrl string) string {
	return fmt.Sprintf(OsctrlURLExtensions, osctrl)
}
```

In `genURLs`, add before the return:

```go
urls.Extensions = genExtensionsURL(osctrlURL)
```

- [ ] **Step 3: Add default ExtensionsDir in cliWrapper**

In `cmd/osctrld/main.go`, in the `cliWrapper` function, add after each OS block's `RemoveScript` default assignment (for both Darwin and Linux cases):

```go
if jsonConfig.ExtensionsDir == "" {
	jsonConfig.ExtensionsDir = genFullPath(jsonConfig.OsqueryPath, "extensions/")
}
```

For the Windows case, add the same block (even though extensions aren't supported yet on Windows, the path default should still be set for config consistency).

- [ ] **Step 4: Add URL generation test**

In `cmd/osctrld/utils_test.go`, add:

```go
func TestGenExtensionsURL(t *testing.T) {
	result := genExtensionsURL("https://osctrl.example.com/env")
	assert.Equal(t, "https://osctrl.example.com/env/osctrld-extensions", result)
}
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean compilation.

- [ ] **Step 6: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All tests pass (including new URL test).

- [ ] **Step 7: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/config.go cmd/osctrld/main.go cmd/osctrld/utils.go cmd/osctrld/utils_test.go
git commit -m "feat: add ExtensionsDir config and extensions URL generation"
```

---

### Task 2: Add extension data types

**Files:**
- Modify: `cmd/osctrld/actions.go`

- [ ] **Step 1: Add ExtensionEntry and ExtensionsRequest types**

In `cmd/osctrld/actions.go`, add after the `VerifyResponse` struct:

```go
type ExtensionEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type ExtensionsRequest struct {
	Secret string `json:"secret"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go build ./cmd/osctrld/`
Expected: Clean compilation.

- [ ] **Step 3: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/actions.go
git commit -m "feat: add extension data types"
```

---

### Task 3: Create extensions.go with retrieval and deployment logic

**Files:**
- Create: `cmd/osctrld/extensions.go`
- Create: `cmd/osctrld/extensions_test.go`

- [ ] **Step 1: Write tests**

Create `cmd/osctrld/extensions_test.go`:

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
)

func TestRetrieveExtensionManifest_Success(t *testing.T) {
	manifest := []ExtensionEntry{
		{Name: "test_ext.ext", URL: "http://example.com/test_ext.ext"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(manifest)
	}))
	defer server.Close()

	entries, err := retrieveExtensionManifest("test-secret", server.URL, false)
	assert.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "test_ext.ext", entries[0].Name)
}

func TestRetrieveExtensionManifest_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ExtensionEntry{})
	}))
	defer server.Close()

	entries, err := retrieveExtensionManifest("test-secret", server.URL, false)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

func TestRetrieveExtensionManifest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer server.Close()

	_, err := retrieveExtensionManifest("test-secret", server.URL, false)
	assert.Error(t, err)
}

func TestDownloadExtension_Success(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho extension")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "test_ext.ext")

	changed, err := downloadExtension(server.URL, path, false)
	assert.NoError(t, err)
	assert.True(t, changed)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, content)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

func TestDownloadExtension_NoChange(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho extension")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "test_ext.ext")
	os.WriteFile(path, binaryContent, 0755)

	changed, err := downloadExtension(server.URL, path, false)
	assert.NoError(t, err)
	assert.False(t, changed, "same content should not report changed")
}

func TestSyncExtensions_Success(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho extension")

	mux := http.NewServeMux()
	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		manifest := []ExtensionEntry{
			{Name: "test_ext.ext", URL: ""},
		}
		manifest[0].URL = "http://" + r.Host + "/binary"
		json.NewEncoder(w).Encode(manifest)
	})
	mux.HandleFunc("/binary", func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	dir := t.TempDir()
	jsonConfig.Secret = "test-secret"
	jsonConfig.ExtensionsDir = dir
	jsonConfig.Insecure = false
	osctrlURLs.Extensions = server.URL + "/manifest"

	changed, err := syncExtensions()
	assert.NoError(t, err)
	assert.True(t, changed)

	content, err := os.ReadFile(filepath.Join(dir, "test_ext.ext"))
	require.NoError(t, err)
	assert.Equal(t, binaryContent, content)
}

func TestSyncExtensions_EmptyManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ExtensionEntry{})
	}))
	defer server.Close()

	jsonConfig.Secret = "test-secret"
	jsonConfig.Insecure = false
	osctrlURLs.Extensions = server.URL

	changed, err := syncExtensions()
	assert.NoError(t, err)
	assert.False(t, changed)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run TestRetrieveExtensionManifest -v -count=1`
Expected: FAIL — functions not defined.

- [ ] **Step 3: Create extensions.go**

Create `cmd/osctrld/extensions.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

func retrieveExtensionManifest(secret, url string, insecure bool) ([]ExtensionEntry, error) {
	reqData := ExtensionsRequest{Secret: secret}
	body, err := genericRetrieve(url, insecure, reqData)
	if err != nil {
		return nil, fmt.Errorf("error retrieving extension manifest - %v", err)
	}
	var entries []ExtensionEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("error parsing extension manifest - %v", err)
	}
	return entries, nil
}

func downloadExtension(url, destPath string, insecure bool) (bool, error) {
	code, body, err := SendRequest(http.MethodGet, url, nil, map[string]string{}, insecure)
	if err != nil {
		return false, fmt.Errorf("error downloading extension - %v", err)
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("HTTP %d downloading extension", code)
	}
	changed, err := writeContentExists(destPath, string(body), filepath.Base(destPath), true)
	if err != nil {
		return false, err
	}
	if changed {
		if err := os.Chmod(destPath, 0755); err != nil {
			return false, fmt.Errorf("error setting extension permissions - %v", err)
		}
	}
	return changed, nil
}

func syncExtensions() (bool, error) {
	log.Info().Msg("syncing extensions")
	manifest, err := retrieveExtensionManifest(jsonConfig.Secret, osctrlURLs.Extensions, jsonConfig.Insecure)
	if err != nil {
		return false, err
	}
	if len(manifest) == 0 {
		log.Debug().Msg("no extensions in manifest")
		return false, nil
	}
	if err := os.MkdirAll(jsonConfig.ExtensionsDir, 0755); err != nil {
		return false, fmt.Errorf("error creating extensions directory - %v", err)
	}
	anyChanged := false
	for _, ext := range manifest {
		destPath := filepath.Join(jsonConfig.ExtensionsDir, ext.Name)
		log.Debug().Str("name", ext.Name).Str("url", ext.URL).Msg("downloading extension")
		changed, err := downloadExtension(ext.URL, destPath, jsonConfig.Insecure)
		if err != nil {
			log.Error().Err(err).Str("name", ext.Name).Msg("failed to download extension")
			continue
		}
		if changed {
			log.Info().Str("name", ext.Name).Str("path", destPath).Msg("extension updated")
			anyChanged = true
		}
	}
	return anyChanged, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -run "TestRetrieveExtensionManifest|TestDownloadExtension|TestSyncExtensions" -v -count=1`
Expected: All 7 tests pass.

- [ ] **Step 5: Run all tests**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && go test ./cmd/osctrld/ -v -count=1`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/afraguas/CursorTest/osctrld/osctrld
git add cmd/osctrld/extensions.go cmd/osctrld/extensions_test.go
git commit -m "feat: add extension manifest retrieval and binary deployment"
```

---

### Task 4: Integrate extension sync into daemon loop

**Files:**
- Modify: `cmd/osctrld/service.go`

- [ ] **Step 1: Update syncOnce to include extension sync**

In `cmd/osctrld/service.go`, replace `syncOnce`:

```go
func syncOnce(c *cli.Context) {
	originalForce := jsonConfig.Force
	jsonConfig.Force = true
	defer func() { jsonConfig.Force = originalForce }()

	var flagsChanged, certChanged, extensionsChanged bool

	log.Info().Msg("syncing flags")
	if changed, err := getFlags(c); err != nil {
		log.Error().Err(err).Msg("failed to sync flags")
	} else {
		flagsChanged = changed
	}

	log.Info().Msg("syncing cert")
	if changed, err := getCert(c); err != nil {
		log.Error().Err(err).Msg("failed to sync cert")
	} else {
		certChanged = changed
	}

	if changed, err := syncExtensions(); err != nil {
		log.Error().Err(err).Msg("failed to sync extensions")
	} else {
		extensionsChanged = changed
	}

	if flagsChanged || certChanged || extensionsChanged {
		log.Info().
			Bool("flags_changed", flagsChanged).
			Bool("cert_changed", certChanged).
			Bool("extensions_changed", extensionsChanged).
			Msg("configuration changed, restarting osquery")
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
git commit -m "feat: integrate extension sync into daemon loop"
```

---

### Task 5: Final verification

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
Expected: No changes.

- [ ] **Step 4: Run linter (if available)**

Run: `cd /Users/afraguas/CursorTest/osctrld/osctrld && golangci-lint run ./cmd/osctrld/ 2>&1 || echo "linter not installed locally, CI will check"`
