# Phase 2 Roadmap

## Goal

Extend the log analysis MCP server with five features: compressed file support,
log diff, report generation, live log tailing, and remote log sources (SSH + Windows).

## Status

| Phase | Feature              | Status      |
|-------|----------------------|-------------|
| 1     | Compressed files     | Not started |
| 2     | Log diff             | ✅ Complete  |
| 3     | Report prompt        | ✅ Complete  |
| 4     | Live tailing         | Blocked     |
| 5     | Remote sources (SSH) | ✅ Complete  |

Phase 5 includes: SSH infrastructure, 3 remote tools, macOS firewall proxy
fallback, `investigate_remote` prompt, and SSH-guarded integration tests.

## Policy: External Dependencies

CLAUDE.md keeps the strict "No external deps beyond the MCP SDK. Stdlib only." rule.
That rule remains the best practice and stays in Nuclia RAG unchanged.

Exceptions are declared in the **specs that need them**, not in CLAUDE.md. Each spec
that requires an external dep must include an `## External Dependencies` section with:
- The import path (e.g. `golang.org/x/crypto/ssh`).
- Why it is needed (what stdlib cannot provide).
- Vetting notes (maintainer, reputation, transitive deps, vulnerability status).

This keeps the general rule clean while making exceptions explicit, justified, and
scoped to the feature that requires them.

**No CLAUDE.md changes needed.** No Nuclia upload needed.

---

## Phase 1: Compressed File Support

**Deps:** None (stdlib only: `compress/gzip`, `compress/bzip2`, `archive/zip`)

**Goal:** All existing tools transparently handle `.gz`, `.bz2`, and `.zip` log files.

**Spec:** `specs/compression.md` (to be written)

**Design decisions:**
- Detection by file extension, not magic bytes (simpler, sufficient for log files).
- `fileutil` gets a new `OpenReader(path) (io.ReadCloser, error)` that wraps the
  raw file in the appropriate decompressor before returning.
- `ReadLines`, `TailLines`, and all tools call `OpenReader` instead of `os.Open`.
- `TailLines` backward-seek optimisation is unavailable for compressed files.
  Fall back to streaming from the beginning, collecting the last N lines in a ring
  buffer. Document this performance tradeoff in the spec.
- `.zip` archives: read only the first entry, or accept an optional `entry` parameter.
- Binary detection runs on the decompressed stream, not raw bytes.

**Steps:**
1. Write `specs/compression.md`.
2. Write `internal/fileutil/compression.go` + `compression_test.go` (TDD).
3. Modify `ReadLines` and `TailLines` to use `OpenReader`.
4. Run all existing tests — they must still pass (regression).
5. Add integration tests with compressed temp files.
6. `go test -race ./...` && `go vet ./...`.

**Acceptance:**
- `log_read /var/log/syslog.gz` works.
- `log_tail /var/log/app.log.bz2` works (slower than uncompressed, documented).
- `log_search` on a `.zip` containing a log file works.
- All 668+ existing tests still pass.

---

## Phase 2: Log Diff Tool ✅

Complete. `log_diff` tool with file-vs-file and time-range modes.
Spec: `specs/tools/log_diff.md`. 13 unit tests + 3 integration tests.

---

## Phase 3: Report Generation Prompt ✅

Complete. `generate_report` prompt with 8-step investigation workflow.
Conditional diff step and incident ID header. 3 integration tests.

---

## Phase 4: Live Log Tailing

**Deps:** None for local tailing. MCP Go SDK already supports progress notifications
and Streamable HTTP transport (`mcp.NewStreamableHTTPHandler`,
`ServerSession.NotifyProgress`).

**Goal:** New `watch_logs` tool that streams new lines as they are appended to a file.

**Spec:** `specs/tools/watch_logs.md` (to be written)

**Research (completed):**
- MCP Go SDK v1.4.1 has `StreamableHTTPHandler` with full SSE support.
- `ServerSession.NotifyProgress` sends progress notifications mid-request.
- `ServerSession.Log` sends log-level notifications to the client.
- For live tailing, the tool handler can block and send progress notifications
  as new lines appear, returning when cancelled or a timeout/max-lines limit is hit.
- Alternative: use `ServerSession.Log` to stream lines as log messages.
- The server entry point needs a `--transport` flag: `stdio` (default) or `http`.

**Research still needed:**
- [ ] Confirm whether Claude Desktop / MCP Inspector support progress notifications
      or log streaming during a tool call. If not, explore resource subscriptions
      (`ResourceUpdated` notifications) as an alternative delivery mechanism.
- [ ] Test `StreamableHTTPHandler` with a simple prototype before committing to
      the full implementation.

**Design decisions:**
- Poll-based: `os.Stat` + seek to last known offset. No `fsnotify` dep needed.
- Poll interval: 500ms default, configurable.
- The tool blocks until cancelled, timeout, or max_lines reached.
- Lines are delivered via progress notifications (message field = line text).
- Server gets `--transport stdio|http` flag and `--addr` for HTTP mode.

**Steps:**
1. Research spike: prototype StreamableHTTP + progress notifications.
2. Write `specs/tools/watch_logs.md`.
3. Add `--transport` and `--addr` flags to `cmd/log-analysis-mcp/main.go`.
4. Write `internal/tools/watch_logs.go` + `watch_logs_test.go` (TDD).
5. Register in `internal/server/server.go`.
6. Manual test with MCP Inspector over HTTP.
7. `go test -race ./...` && `go vet ./...`.

---

## Phase 5: Remote Log Sources (SSH) ✅

Complete. All sub-phases delivered:

- **5a** SSH infrastructure (`internal/remote/`): auth chain, connection pooling,
  host key verification, command exec, file download, journal export. 22 unit tests.
- **5b** `log_run_remote_command` tool. 5 unit tests.
- **5c** `log_discover_remote` tool. Rotated file grouping, journal detection. Unit tests.
- **5d** `log_gather_remote` tool. Size-guarded download, temp file management. Unit tests.
- **5e** `investigate_remote` prompt. 10-step multi-system workflow. 4 prompt tests +
  5 SSH-guarded integration tests (discover → gather → summarize → diff).
- **macOS firewall proxy fallback**: `dialTCP` tries `net.Dial` first, falls back to
  `/usr/bin/ssh -W` on darwin when blocked. Works on MDM-managed Macs without sudo.
  17 tests (proxy_conn + dialer).

Dep: `golang.org/x/crypto` v0.49.0 (SSH client). Exception in `specs/remote.md`.

---

## Remaining Work

| Phase | Feature          | Blocked by | Est. complexity |
|-------|------------------|------------|-----------------|
| 1     | Compressed files | —          | Medium          |
| 4     | Live tailing     | Research   | Large           |

Phase 1 is ready to start. Phase 4 needs a research spike to confirm
whether Claude Desktop / MCP Inspector surface progress notifications.

## Open Questions

1. **Windows remoting:** SSH-on-Windows covers modern hosts. Is WinRM support for
   legacy Windows needed? (Deferred until user feedback.)
2. **Live tailing delivery:** Progress notifications vs log messages vs resource
   subscriptions — which does Claude Desktop actually surface to the user?
3. **Compressed tail performance:** Is the ring-buffer fallback for `log_tail` on
   compressed files acceptable, or should we document it as unsupported?