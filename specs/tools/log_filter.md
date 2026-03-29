# Tool: `log_filter`

**Description (shown to LLM):**
> Filter log entries by severity level, time range, source component, or message content. Parses the log file and returns only entries matching all specified criteria. Multiple filters are combined with AND logic.

---

## Input Schema

| Parameter         | Type       | Required | Default | Description                                                    |
| ----------------- | ---------- | -------- | ------- | -------------------------------------------------------------- |
| `path`            | `string`   | Yes      | —       | Path to the log file                                           |
| `level`           | `[]string` | No       | —       | Log levels to include (e.g., `["ERROR", "WARN"]`)              |
| `after`           | `string`   | No       | —       | ISO 8601 timestamp — only include entries after this time      |
| `before`          | `string`   | No       | —       | ISO 8601 timestamp — only include entries before this time     |
| `source`          | `string`   | No       | —       | Regex pattern to match the source/component field              |
| `message_pattern` | `string`   | No       | —       | Regex pattern to match the message content                     |
| `max_results`     | `int`      | No       | `100`   | Maximum number of entries to return (max 1000)                 |

### Go Input Struct

```go
type FilterLogsInput struct {
    Path           string   `json:"path"            jsonschema:"required,description=Path to the log file"`
    Level          []string `json:"level"           jsonschema:"description=Log levels to include (e.g. ERROR\\, WARN)"`
    After          string   `json:"after"           jsonschema:"description=ISO 8601 timestamp — include entries after this time"`
    Before         string   `json:"before"          jsonschema:"description=ISO 8601 timestamp — include entries before this time"`
    Source         string   `json:"source"          jsonschema:"description=Regex pattern to match the source/component field"`
    MessagePattern string   `json:"message_pattern" jsonschema:"description=Regex pattern to match the message content"`
    MaxResults     int      `json:"max_results"     jsonschema:"description=Maximum entries to return (max 1000),minimum=1,maximum=1000"`
}
```

### Default Values (applied in handler)

| Field        | Default |
| ------------ | ------- |
| `MaxResults` | `100`   |

---

## Output Format

> **Note:** Types from `internal/types/` are referenced below. See `specs/types.md` for shared type definitions.

### Go Output Structs

```go
type FilteredEntry struct {
    LineNumber int       `json:"line_number"`
    Timestamp  *string   `json:"timestamp"`
    Level      *LogLevel `json:"level"`          // LogLevel from internal/types
    Source     *string   `json:"source"`
    Message    string    `json:"message"`
    Raw        string    `json:"raw"`
}

type AppliedFilters struct {
    Level  []string `json:"level,omitempty"`
    After  string   `json:"after,omitempty"`
    Before string   `json:"before,omitempty"`
}

type FilterLogsOutput struct {
    Entries        []FilteredEntry `json:"entries"`
    TotalMatched   int             `json:"total_matched"`
    TotalScanned   int             `json:"total_scanned"`
    AppliedFilters AppliedFilters  `json:"applied_filters"`
    Truncated      bool            `json:"truncated"`
}
```

### JSON Example

```json
{
  "entries": [
    {
      "line_number": 1523,
      "timestamp": "2025-01-15T10:45:23.000Z",
      "level": "ERROR",
      "source": "auth",
      "message": "Failed login attempt for user admin@example.com",
      "raw": "2025-01-15T10:45:23Z ERROR [auth] Failed login attempt for user admin@example.com"
    }
  ],
  "total_matched": 47,
  "total_scanned": 54832,
  "applied_filters": {
    "level": ["ERROR", "WARN"],
    "after": "2025-01-15T10:00:00.000Z",
    "before": "2025-01-15T11:00:00.000Z"
  },
  "truncated": false
}
```

---

## Handler Signature

```go
func handleFilterLogs(ctx context.Context, req *mcp.CallToolRequest, input FilterLogsInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "log_filter",
    Description: "Filter log entries by severity level, time range, source component, or message content. Parses the log file and returns only entries matching all specified criteria. Multiple filters are combined with AND logic.",
}, handleFilterLogs)
```

---

## Usage Scenario

An AI assistant investigating a production incident filters a large log file to show only ERROR and WARN entries within a 1-hour window around the reported incident time.