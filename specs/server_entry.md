# Server Entry Point & Wiring

This spec describes the three files that bootstrap the MCP server. No application logic lives here — only configuration, wiring, and lifecycle management.

---

## 1. Entry Point — `cmd/log-analysis-mcp/main.go`

Responsibilities:

- Configure `log/slog` with a text handler writing to **stderr** (stdout is reserved for MCP JSON-RPC transport and must never be written to directly).
- Set the default log level to `Info`.
- Create a root `context.Context` that cancels on `SIGINT` or `SIGTERM` using `signal.NotifyContext`.
- Instantiate the server via `server.New(version)` (version may be a package-level variable or injected at build time via `-ldflags`).
- Call `server.Run(ctx)` and block until it returns.
- If `Run` returns a non-nil error, log it to stderr and exit with code 1.
- No other logic belongs in this file — it is purely orchestration.

---

## 2. Server — `internal/server/server.go`

Responsibilities:

- Define a `Server` struct that wraps the SDK's `MCPServer`.
- Provide a `New(version string) *Server` constructor that:
  - Creates an `MCPServer` with implementation name `"log-analysis-mcp"` and the provided version string.
  - Enables tool, resource (with subscribe + list-changed), and prompt (with list-changed) capabilities via SDK options.
  - Calls `tools.Register(srv)` to register all 10 tool handlers.
  - Calls `resources.Register(srv)` to register resource definitions.
  - Calls `prompts.Register(srv)` to register prompt templates.
- Provide a `Run(ctx context.Context) error` method that:
  - Creates a new `StdioTransport` from the SDK.
  - Starts the MCP server on that transport, blocking until the context is cancelled or the transport closes.
  - Returns any error from the transport/server lifecycle.

---

## 3. Tool Registration — `internal/tools/register.go`

Responsibilities:

- Export a single `Register(srv *server.MCPServer)` function.
- Call `mcp.AddTool` once for each of the 10 tools listed below.
- Each `AddTool` call provides the tool's name, description, and a handler function reference.
- Each handler lives in its own file under `internal/tools/` (e.g. `log_read.go`, `log_search.go`). The register file only wires them — it contains no handler logic.
- Input schemas (via jsonschema struct tags) are defined alongside each handler, not in the register file.

### Tools to register

| Tool | Handler reference | Purpose (used as the tool description basis) |
|------|-------------------|----------------------------------------------|
| `log_read` | `handleReadLogs` | Read a log file with line-range pagination |
| `log_search` | `handleSearchLogs` | Regex/text search with optional context lines |
| `log_parse` | `handleParseLogs` | Auto-detect format and extract structured fields |
| `log_filter` | `handleFilterLogs` | Filter by level, time range, source, or message pattern |
| `log_summarize` | `handleSummarizeLogs` | Statistical summary: level distribution, top errors, rates |
| `log_tail` | `handleTailLogs` | Read last N lines (most recent entries) |
| `log_detect_anomalies` | `handleDetectAnomalies` | Find error spikes, new error types, logging gaps |
| `log_extract_errors` | `handleExtractErrors` | Cluster errors and exceptions by similarity |
| `log_correlate` | `handleCorrelateLogs` | Correlate events across files by request/trace ID |
| `log_timeline` | `handleTimeline` | Build chronological event timeline |

---

## Design Constraints

- **No stdout writes** outside the MCP transport. All diagnostic/debug output goes to stderr via `slog`.
- **Read-only file access** — the server never modifies log files.
- **Minimal dependencies** — only the MCP Go SDK and standard library. Avoid third-party packages for functionality Go provides natively.
- **Clean separation** — `main.go` owns the process lifecycle, `server.go` owns MCP configuration, `register.go` owns the tool wiring. No layer reaches into another's concerns.