# Tool: `log_correlate`

**Description (shown to LLM):**
> Correlate events across multiple log files using shared identifiers like request IDs or trace IDs. Builds a unified timeline showing how a single request or operation flows through different components. Useful for distributed system debugging.

---

## Input Schema

| Parameter              | Type       | Required | Default        | Description                                                  |
| ---------------------- | ---------- | -------- | -------------- | ------------------------------------------------------------ |
| `paths`                | `[]string` | Yes      | —              | Array of log file paths to correlate (2–10 files)            |
| `correlation_field`    | `string`   | No       | `"request_id"` | Field name to use for correlation                            |
| `time_window_seconds`  | `int`      | No       | `60`           | Max time window in seconds for grouping correlated events    |

### Go Input Struct

```go
type CorrelateLogsInput struct {
    Paths              []string `json:"paths"                jsonschema:"required,description=Array of log file paths to correlate (2-10 files),minItems=2,maxItems=10"`
    CorrelationField   string   `json:"correlation_field"    jsonschema:"description=Field name used for correlation (e.g. request_id\\, trace_id)"`
    TimeWindowSeconds  int      `json:"time_window_seconds"  jsonschema:"description=Maximum time window in seconds for grouping correlated events,minimum=1,maximum=3600"`
}
```

### Default Values (applied in handler)

| Field               | Default        |
| ------------------- | -------------- |
| `CorrelationField`  | `"request_id"` |
| `TimeWindowSeconds` | `60`           |

---

## Correlation Strategy

1. Parse each log file to extract structured records.
2. For each record, look for the `correlation_field` in:
   - Parsed `extra_fields` (from structured logs)
   - Message content via regex: `correlation_field[=: ]["']?(\S+)`
3. Group all entries sharing the same correlation value.
4. Within each group, sort entries chronologically across all files.
5. Only include groups where entries span at least 2 different files.
6. Filter out groups where the time span exceeds `time_window_seconds`.

---

## Output Types

> **Note:** Types prefixed with `types.` are defined in `internal/types/` — see `specs/types.md` for canonical shared type definitions.

### Go Output Structs

```go
type CorrelatedEvent struct {
    Timestamp  string  `json:"timestamp"`
    File       string  `json:"file"`
    LineNumber int     `json:"line_number"`
    Level      *LogLevel `json:"level"`    // types.LogLevel — see specs/types.md
    Source     *string `json:"source"`
    Message    string  `json:"message"`
}

type CorrelatedGroup struct {
    CorrelationID    string            `json:"correlation_id"`
    CorrelationField string            `json:"correlation_field"`
    FilesInvolved    []string          `json:"files_involved"`
    TimeSpanMs       int64             `json:"time_span_ms"`
    Events           []CorrelatedEvent `json:"events"`
}

type FileAnalysis struct {
    Path          string `json:"path"`
    EntriesParsed int    `json:"entries_parsed"`
}

type CorrelateLogsOutput struct {
    CorrelatedGroups []CorrelatedGroup `json:"correlated_groups"`
    TotalGroups      int               `json:"total_groups"`
    GroupsReturned   int               `json:"groups_returned"`
    FilesAnalyzed    []FileAnalysis    `json:"files_analyzed"`
}
```

---

## Handler Signature

```go
func handleCorrelateLogs(ctx context.Context, req *mcp.CallToolRequest, input CorrelateLogsInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "log_correlate",
    Description: "Correlate events across multiple log files using shared identifiers like request IDs or trace IDs. Builds a unified timeline showing how a single request or operation flows through different components. Useful for distributed system debugging.",
}, handleCorrelateLogs)
```

---

## Output Format

```json
{
  "correlated_groups": [
    {
      "correlation_id": "req-abc-123",
      "correlation_field": "request_id",
      "files_involved": ["api-gateway.log", "auth-service.log", "user-db.log"],
      "time_span_ms": 342,
      "events": [
        {
          "timestamp": "2025-01-15T14:30:00.100Z",
          "file": "api-gateway.log",
          "line_number": 12034,
          "level": "INFO",
          "source": "gateway",
          "message": "Incoming request: GET /api/users/42 request_id=req-abc-123"
        },
        {
          "timestamp": "2025-01-15T14:30:00.215Z",
          "file": "auth-service.log",
          "line_number": 8921,
          "level": "INFO",
          "source": "auth",
          "message": "Token validated for request_id=req-abc-123"
        },
        {
          "timestamp": "2025-01-15T14:30:00.442Z",
          "file": "user-db.log",
          "line_number": 6712,
          "level": "ERROR",
          "source": "db",
          "message": "Query timeout for request_id=req-abc-123: SELECT * FROM users WHERE id=42"
        }
      ]
    }
  ],
  "total_groups": 1243,
  "groups_returned": 50,
  "files_analyzed": [
    { "path": "api-gateway.log", "entries_parsed": 18234 },
    { "path": "auth-service.log", "entries_parsed": 12001 },
    { "path": "user-db.log", "entries_parsed": 9876 }
  ]
}
```

---

## Example Usage Scenario

An AI assistant is debugging a failed API request. It has log files from three microservices and uses `log_correlate` with `correlation_field: "request_id"` to trace the request's journey through the system and find where it failed.