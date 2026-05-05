# Phase 2 Roadmap — Remaining

Only one item remains. All others (compressed files, log diff, report prompt,
remote SSH) are complete and shipped.

## Live Log Tailing (Blocked)

**Goal:** New `watch_logs` tool that streams new lines as they are appended to a file.

**Blocked on:** Confirm whether Claude Desktop / MCP Inspector / Copilot surface
progress notifications or log streaming during a tool call. If not, explore
resource subscriptions (`ResourceUpdated`) as an alternative delivery mechanism.

**Research completed:**
- MCP Go SDK v1.4.1 has `StreamableHTTPHandler` with full SSE support.
- `ServerSession.NotifyProgress` sends progress notifications mid-request.
- `ServerSession.Log` sends log-level notifications to the client.
- Poll-based design: `os.Stat` + seek to last known offset. No `fsnotify` dep.
- Server would need `--transport stdio|http` flag and `--addr` for HTTP mode.

**Steps (once unblocked):**
1. Research spike: prototype StreamableHTTP + progress notifications.
2. Write `specs/tools/watch_logs.md`.
3. Add `--transport` and `--addr` flags to `cmd/log-analysis-mcp/main.go`.
4. TDD: `internal/tools/watch_logs.go` + test.
5. Manual test with MCP Inspector over HTTP.

## Open Questions

1. **Live tailing delivery:** Progress notifications vs log messages vs resource
   subscriptions — which do MCP clients actually surface to the user?
2. **Windows remoting:** SSH-on-Windows covers modern hosts. Is WinRM needed? (Deferred.)