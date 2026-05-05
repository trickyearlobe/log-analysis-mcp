# Observability Specification

Instrumentation for measuring MCP server performance and tool effectiveness.
Design follows the nuclia-mcp pattern: persistent event log + queryable MCP tool.

## Goals

1. Measure per-tool latency, call frequency, and error rate.
2. Track response sizes to detect context-window pressure.
3. Provide an MCP tool for the LLM to self-diagnose performance issues.
4. Persist metrics across restarts for historical analysis.
5. Emit structured warnings for notable events (errors, slow calls).
6. Zero external dependencies. Minimal per-call overhead.

## Architecture

### Event Log

Every tool call writes a single-line JSON event to a log file:

```
~/.log-analysis-mcp/metrics/events.jsonl
```

Each event:
```json
{"ts":"2025-01-15T10:00:01Z","tool":"log_search","status":"ok","duration_ms":142,"response_bytes":3201,"warning":"","file_bytes":1048576}
```

Log rotation: new file per day (`events-YYYY-MM-DD.jsonl`). Files older than
30 days are gzip-compressed. Files older than 90 days are deleted.

### Event Fields

| Field | Type | Description |
|-------|------|-------------|
| `ts` | string | ISO 8601 timestamp |
| `tool` | string | Tool name |
| `status` | string | "ok" or "error" |
| `duration_ms` | int | Execution time in milliseconds |
| `response_bytes` | int | JSON response size |
| `warning` | string | Semantic warning category (empty if none) |
| `error_code` | string | Error category when status=error |
| `file_bytes` | int | Bytes of log file processed (when applicable) |

### Warning Categories

| Warning | Meaning |
|---------|---------|
| `SLOW_CALL` | Duration exceeded 2s |
| `LARGE_RESPONSE` | Response > 50KB |
| `NO_RESULTS` | Search/filter returned zero matches |
| `FILE_TOO_LARGE` | Input file > 100MB |
| `PARSE_FAILURE` | Format detection failed |

### Middleware Approach

Use `server.AddReceivingMiddleware()` from the Go MCP SDK. A single middleware
intercepts all `tools/call` requests and:

1. Records start time.
2. Calls next handler.
3. Measures duration, response size, success/failure.
4. Writes event to log file.
5. Emits `slog.Warn` for errors and slow calls.

### MCP Tool: `log_metrics`

A queryable tool (not just a resource) so the LLM can filter and group:

**Input:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| since | string | "24h" | Time window start (e.g., "24h", "7d") |
| group_by | string | "tool" | Aggregation key: "tool", "status", "warning" |
| tool | string | "" | Filter to one tool name |
| top_k | int | 10 | Max groups to return |

**Output:**

```json
{
  "total_events": 257,
  "window": {"from": "...", "to": "..."},
  "groups": [
    {
      "key": "log_search",
      "calls": 42,
      "errors": 1,
      "error_rate": 0.024,
      "duration_p50_ms": 85,
      "duration_p95_ms": 340,
      "avg_response_bytes": 2100,
      "warnings": {"SLOW_CALL": 1}
    }
  ]
}
```

### Stderr Logging

Always emitted (no flag):
- `slog.Error` for tool failures
- `slog.Warn` for slow calls (>2s) and large responses (>50KB)
- `slog.Info` at shutdown with per-tool summary

## Implementation Constraints

- NEVER write to stdout. All output to stderr via `slog` or served via MCP tool.
- No external dependencies. stdlib only (`sync`, `time`, `encoding/json`, `os`).
- Event log writes are buffered and flushed periodically (every 1s or 100 events).
- Thread-safe. Concurrent tool calls must not corrupt the log.
- File I/O must not block tool execution. Use a background writer goroutine.
- The `log_metrics` tool reads the event log on demand (no in-memory state required).

## Efficiency Indicators

Beyond raw performance, the LLM can use metrics to detect workflow issues:

| Signal | What it reveals |
|--------|----------------|
| Tool never called | Dead tool or poor discoverability |
| High error rate on one tool | Broken tool or bad parameter defaults |
| High latency on small files | Algorithm issue |
| Large response sizes | Context pressure — needs tighter pagination |
| `NO_RESULTS` warnings | Bad search patterns or wrong file targeted |
| file_info → count_by_level → search | Healthy triage workflow |
