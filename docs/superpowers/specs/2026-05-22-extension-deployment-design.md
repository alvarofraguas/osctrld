# Extension Deployment for osctrld

**Date:** 2026-05-22
**Status:** Approved
**Goal:** Enable the daemon to fetch osquery extension binaries from osctrl and deploy them to the local extensions directory, restarting osquery when extensions change.

## 1. Extension Manifest Endpoint

New osctrl endpoint:

```
POST {base}/{env}/osctrld-extensions
```

Request body (same auth pattern as flags/cert):
```json
{"secret": "enrollment-secret"}
```

Response: JSON array of extension metadata:
```json
[
  {"name": "example_ext.ext", "url": "https://osctrl.example.com/ext/example_ext.ext"}
]
```

Each entry contains:
- `name` — filename for the extension binary
- `url` — direct download URL for the binary

If the server returns an empty array `[]`, no extensions are deployed. If the endpoint returns a non-200 status, log the error and continue (same pattern as flags/cert failures).

## 2. Extension Binary Download

Each extension binary is downloaded via a simple GET request to the `url` from the manifest. The binary is written to the OS-specific extensions directory with executable permissions (`0755`).

Change detection uses the existing `writeContentExists` pattern — compare downloaded content against the file on disk. If the content is identical, no write occurs.

## 3. Extension Directory

OS-specific default paths:

| OS | Default Path |
|----|-------------|
| Linux | `/etc/osquery/extensions/` |
| macOS | `/private/var/osquery/extensions/` |

Windows is deferred (consistent with osquery lifecycle feature).

The directory is created automatically if it doesn't exist (`os.MkdirAll`).

A new `ExtensionsDir` field is added to `JSONConfiguration`:
```go
ExtensionsDir string `json:"extensionsDir"`
```

Default value follows the same pattern as other osquery paths — derived from `OsqueryPath` + `extensions/` in `cliWrapper`.

## 4. New CLI Command

No new CLI command. Extension deployment is daemon-only — it runs as part of `syncOnce` in `service` mode. There's no use case for one-shot extension deployment since extensions require osquery to be running to load them.

## 5. Integration with Daemon

`syncOnce` is extended with a third sync step:

```
sync flags → sync cert → sync extensions → restart osquery if anything changed
```

The `syncExtensions` function:
1. Fetches the manifest from `{base}/{env}/osctrld-extensions`
2. For each extension in the manifest, downloads the binary
3. Writes each binary to `{extensionsDir}/{name}`
4. Returns `(bool, error)` — true if any extension was written/updated

Extension changes trigger osquery restart along with flags/cert changes.

## 6. Data Types

```go
type ExtensionEntry struct {
    Name string `json:"name"`
    URL  string `json:"url"`
}

type ExtensionsRequest struct {
    Secret string `json:"secret"`
}
```

## 7. URL Generation

New constant and function:
```go
OsctrlURLExtensions = "%s/osctrld-extensions"
```

New field in `OsctrlURLs`:
```go
Extensions string
```

## 8. Error Handling

- Manifest fetch failure: log error, return `(false, error)` — other syncs still run
- Individual extension download failure: log error, continue with remaining extensions
- File write failure: log error, continue with remaining extensions
- If any extension succeeds and changes, return `true` to trigger restart
- Empty manifest: return `(false, nil)` — no extensions to deploy, no error

## 9. Security

- Extension binaries are downloaded over HTTPS (respects `--insecure` flag for self-signed certs)
- Extensions are written with `0755` permissions (executable by owner, readable by others)
- The manifest endpoint uses the same enrollment secret authentication as other endpoints
- Binary URLs from the manifest are trusted (they come from the authenticated osctrl server)

## 10. Files Changed

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `cmd/osctrld/config.go` | Add `ExtensionsDir` field |
| Modify | `cmd/osctrld/utils.go` | Add extensions URL constant, generator, and `OsctrlURLs.Extensions` |
| Modify | `cmd/osctrld/utils_test.go` | Test for extensions URL generation |
| Modify | `cmd/osctrld/actions.go` | Add `ExtensionEntry`, `ExtensionsRequest`, `syncExtensions` |
| Create | `cmd/osctrld/extensions.go` | `retrieveExtensionManifest`, `downloadExtension`, `syncExtensions` |
| Create | `cmd/osctrld/extensions_test.go` | Tests for extension retrieval and deployment |
| Modify | `cmd/osctrld/service.go` | Add extension sync to `syncOnce` |
| Modify | `cmd/osctrld/main.go` | Add `ExtensionsDir` default path in `cliWrapper` |

## 11. Tests

- **Manifest retrieval test**: Mock HTTP server returns JSON manifest, verify parsing
- **Extension download test**: Mock HTTP server serves binary, verify written to disk
- **Empty manifest test**: Verify returns `(false, nil)` with no side effects
- **Server error test**: Verify logs error and returns gracefully
- **Change detection test**: Download same binary twice, verify second returns `false`
- **URL generation test**: Verify extensions URL is generated correctly

## 12. Scope Boundaries

**Not included:**
- Windows support
- Extension removal (cleanup of extensions not in manifest)
- Extension versioning/rollback
- Extension signature verification (beyond HTTPS transport security)
- One-shot CLI command for extension deployment
- Extension-specific restart (vs full osquery restart)
