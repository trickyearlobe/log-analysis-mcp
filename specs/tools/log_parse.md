# Tool: `log_parse`

**Description (shown to LLM):**
> Auto-detect the log format and parse log lines into structured records with fields like timestamp, level, source, and message. Supports syslog (RFC 3164/5424), Apache/Nginx access logs, and JSON structured logs.

---

## Input Schema

| Parameter     | Type     | Required | Default  | Description                                              |
| ------------- | -------- | -------- | -------- | -------------------------------------------------------- |
| `path`        | `string` | Yes      | —        | Path to the log file                                     |
| `start_line`  | `int`    | No       | `1`      | 1-based line number to start parsing from                |
| `num_lines`   | `int`    | No       | `50`     | Number of lines to parse (max 500)                       |
| `format_hint` | `string` | No       | `"auto"` | Hint for log format to skip auto-detection               |

### Go Input Struct

```go
type ParseLogsInput struct {
    Path       string `json:"path"        jsonschema:"required,description=Path to the log file"`
    StartLine  int    `json:"start_line"  jsonschema:"description=Line number to start parsing from (1-based),minimum=1"`
    NumLines   int    `json:"num_lines"   jsonschema:"description=Number of lines to parse (max 500),minimum=1,maximum=500"`
    FormatHint string `json:"format_hint" jsonschema:"description=Log format hint; auto attempts auto-detection,enum=syslog,enum=apache,enum=nginx,enum=json,enum=auto"`
}
```

### Default Values (applied in handler)

| Field        | Default  |
| ------------ | -------- |
| `StartLine`  | `1`      |
| `NumLines`   | `50`     |
| `FormatHint` | `"auto"` |

---

## Output Format

> **Note:** Types from `internal/types/` are referenced below. See `specs/types.md` for shared type definitions.

### Go Output Structs

```go
type ParsedRecord struct {
    LineNumber  int                    `json:"line_number"`
    Timestamp   *string                `json:"timestamp"`
    Level       *LogLevel              `json:"level"`                 // LogLevel from internal/types
    Source      *string                `json:"source"`
    Message     string                 `json:"message"`
    Raw         string                 `json:"raw"`
    ExtraFields map[string]interface{} `json:"extra_fields,omitempty"`
}

type ParseError struct {
    LineNumber int    `json:"line_number"`
    Raw        string `json:"raw"`
    Error      string `json:"error"`
}

type ParseLogsOutput struct {
    DetectedFormat string         `json:"detected_format"`
    Confidence     float64        `json:"confidence"`
    Records        []ParsedRecord `json:"records"`
    ParseErrors    []ParseError   `json:"parse_errors"`
    TotalParsed    int            `json:"total_parsed"`
    TotalErrors    int            `json:"total_errors"`
}
```

### JSON Example

```json
{
  "detected_format": "syslog-rfc3164",
  "confidence": 0.92,
  "records": [
    {
      "line_number": 1,
      "timestamp": "2025-01-15T10:30:00.000Z",
      "level": "INFO",
      "source": "app",
      "message": "Server started on port 3000",
      "raw": "Jan 15 10:30:00 webserver01 app[1234]: INFO Server started on port 3000",
      "extra_fields": {
        "hostname": "webserver01",
        "pid": "1234",
        "facility": "user"
      }
    }
  ],
  "parse_errors": [
    {
      "line_number": 17,
      "raw": "(unparseable line content)",
      "error": "Line does not match expected syslog format"
    }
  ],
  "total_parsed": 48,
  "total_errors": 2
}
```

---

## Handler Signature

```go
func handleParseLogs(ctx context.Context, req *mcp.CallToolRequest, input ParseLogsInput) (*mcp.CallToolResult, error)
```

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "log_parse",
    Description: "Auto-detect the log format and parse log lines into structured records with fields like timestamp, level, source, and message. Supports syslog (RFC 3164/5424), Apache/Nginx access logs, and JSON structured logs.",
}, handleParseLogs)
```

---

## Usage Scenario

An AI assistant is given an unfamiliar log file. It calls `log_parse` with `format_hint: "auto"` on the first 50 lines to understand the structure, then uses the detected format information to make better use of `log_filter` and other tools.