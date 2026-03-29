# Tool: `log_read`

**Description (shown to LLM):**
> Read a log file with pagination support. Returns lines from the specified file along with metadata about file size and total line count. Use start_line and num_lines to paginate through large files.

---

## Input Schema

| Parameter    | Type     | Required | Default   | Description                                           |
| ------------ | -------- | -------- | --------- | ----------------------------------------------------- |
| `path`       | `string` | Yes      | —         | Absolute or relative path to the log file             |
| `start_line` | `int`    | No       | `1`       | 1-based line number to start reading from             |
| `num_lines`  | `int`    | No       | `100`     | Number of lines to return (max 1000)                  |
| `encoding`   | `string` | No       | `"utf-8"` | File encoding (utf-8, ascii, latin1)                  |

### Go Input Struct

```go
type ReadLogsInput struct {
    Path      string `json:"path"       jsonschema:"required,description=Path to the log file"`
    StartLine int    `json:"start_line" jsonschema:"description=Line number to start reading from (1-based),minimum=1"`
    NumLines  int    `json:"num_lines"  jsonschema:"description=Number of lines to return (max 1000),minimum=1,maximum=1000"`
    Encoding  string `json:"encoding"   jsonschema:"description=File encoding (utf-8\\, ascii\\, latin1),enum=utf-8,enum=ascii,enum=latin1"`
}
```

### Default Values (applied in handler)

| Field       | Default   |
| ----------- | --------- |
| `StartLine` | `1`       |
| `NumLines`  | `100`     |
| `Encoding`  | `"utf-8"` |

---

## Output Format

### Go Output Structs

```go
type LogLine struct {
    LineNumber int    `json:"line_number"`
    Content    string `json:"content"`
}

type LineRange struct {
    Start int `json:"start"`
    End   int `json:"end"`
}

type ReadLogsOutput struct {
    Lines         []LogLine `json:"lines"`
    TotalLines    int       `json:"total_lines"`
    HasMore       bool      `json:"has_more"`
    FileSizeBytes int64     `json:"file_size_bytes"`
    CurrentRange  LineRange `json:"current_range"`
}
```

### JSON Example

```json
{
  "lines": [
    { "line_number": 1, "content": "2025-01-15T10:30:00Z INFO [app] Server started on port 3000" },
    { "line_number": 2, "content": "2025-01-15T10:30:01Z DEBUG [db] Connection pool initialized" }
  ],
  "total_lines": 54832,
  "has_more": true,
  "file_size_bytes": 4521984,
  "current_range": { "start": 1, "end": 100 }
}
```

---

## Handler Signature

```go
func handleReadLogs(ctx context.Context, req *mcp.CallToolRequest, input ReadLogsInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "log_read",
    Description: "Read a log file with pagination support. Returns lines from the specified file along with metadata about file size and total line count. Use start_line and num_lines to paginate through large files.",
}, handleReadLogs)
```

---

## Usage Scenario

An AI assistant is asked to look at a log file. It first calls `log_read` with default parameters to see the beginning of the file, then uses `start_line` to page forward to areas of interest identified from the initial read.