# Phase 2 Roadmap

## Goal

Extend the log analysis MCP server with five features: compressed file support,
log diff, report generation, live log tailing, and remote log sources (SSH + Windows).

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
- `read_logs /var/log/syslog.gz` works.
- `tail_logs /var/log/app.log.bz2` works (slower than uncompressed, documented).
- `search_logs` on a `.zip` containing a log file works.
- All 668+ existing tests still pass.

---

## Phase 2: Log Diff Tool

**Deps:** None

**Goal:** New `diff_logs` tool that compares two log files or two time periods,
highlighting new error types, rate changes, missing/new sources, and throughput shifts.

**Spec:** `specs/tools/diff_logs.md` (to be written)

**Design decisions:**
- Two modes: file-vs-file, and time-range-vs-time-range within one file.
- Reuses existing parsers and error clustering (extract_errors normalisation).
- Single-pass streaming per file/period, accumulates summary stats, then diffs.
- Output: structured JSON with `new_errors`, `resolved_errors`, `rate_changes`,
  `source_changes`, `throughput_comparison`.

**Steps:**
1. Write `specs/tools/diff_logs.md`.
2. Write `internal/tools/diff_logs.go` + `diff_logs_test.go` (TDD).
3. Register in `internal/server/server.go`.
4. Add integration test.
5. `go test -race ./...` && `go vet ./...`.

---

## Phase 3: Report Generation Prompt

**Deps:** None

**Goal:** New `generate_report` prompt that guides the AI through a multi-tool
investigation and produces a structured Markdown incident report.

**Spec:** `specs/resources_and_prompts.md` (append new prompt section)

**Design decisions:**
- Prompt only — no new tool. The AI calls existing tools following the prompt.
- Prompt text references: `summarize_logs`, `extract_errors`, `detect_anomalies`,
  `diff_logs` (if available), `timeline`, `search_logs`.
- Arguments: `log_path` (required), `comparison_path` (optional), `incident_id` (optional).

**Steps:**
1. Update `specs/resources_and_prompts.md` with `generate_report` prompt spec.
2. Add handler in `internal/prompts/prompts.go`.
3. Register in `internal/server/server.go`.
4. Add integration test.
5. `go test -race ./...` && `go vet ./...`.

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

## Phase 5: Remote Log Sources (SSH)

**Deps:** `golang.org/x/crypto/ssh` — declared as exception in `specs/remote.md`

**Goal:** All existing tools can read logs from remote hosts over SSH.

**Spec:** `specs/remote.md` (to be written)

**Research needed:**
- [ ] Confirm `golang.org/x/crypto/ssh` API for session exec + stdout streaming.
- [ ] Evaluate SSH agent forwarding vs key file vs password auth.
- [ ] Determine if `golang.org/x/crypto` pulls in transitive deps that conflict
      with our module.
- [ ] Investigate WinRM libraries for Windows remote. Modern Windows (2019+/Win10+)
      ships OpenSSH server, so SSH may cover Windows too. Decide whether WinRM is
      needed or if SSH-on-Windows is sufficient.

**Design decisions (tentative, pending research):**
- Path syntax: `ssh://user@host:port/path/to/log` parsed from the `path` field.
- Spec must include `## External Dependencies` section vetting `golang.org/x/crypto/ssh`.
- New `internal/remote/` package with a `Reader` interface matching `io.ReadCloser`.
- `fileutil.OpenReader` checks for `ssh://` prefix and delegates to remote reader.
- Auth: SSH agent > key file (`~/.ssh/id_*`) > password (prompted via MCP elicitation).
- Connection pooling: one SSH connection per host, reused across tool calls within
  a session. Connections closed when the MCP session ends.
- Windows remoting: prefer SSH. WinRM only if there's a strong user need for
  legacy Windows hosts without OpenSSH.

**Steps:**
1. Research spike: prototype SSH exec + streaming stdout.
2. Write `specs/remote.md`.
3. `go get golang.org/x/crypto/ssh` and vet the dependency.
4. Write `internal/remote/ssh.go` + `ssh_test.go` (TDD, mock SSH server).
5. Integrate with `fileutil.OpenReader`.
6. Test with real remote host (manual).
7. `go test -race ./...` && `go vet ./...`.

---

## Execution Order

| Phase | Feature              | Blocked by | Est. complexity |
|-------|----------------------|------------|-----------------|
| 1     | Compressed files     | —          | Medium          |
| 2     | Log diff             | —          | Medium          |
| 3     | Report prompt        | Phase 2*   | Small           |
| 4     | Live tailing         | Research   | Large           |
| 5     | Remote sources (SSH) | Research   | Large           |

*Phase 3 benefits from Phase 2 (diff_logs referenced in report prompt) but can
proceed without it by making that section conditional.

Phases 1-3 can start immediately. Phases 4-5 need research spikes first.

## Open Questions

1. **Windows remoting:** SSH-on-Windows covers modern hosts. Is WinRM support for
   legacy Windows needed? (Deferred until user feedback.)
2. **Live tailing delivery:** Progress notifications vs log messages vs resource
   subscriptions — which does Claude Desktop actually surface to the user?
3. **Compressed tail performance:** Is the ring-buffer fallback for `tail_logs` on
   compressed files acceptable, or should we document it as unsupported?