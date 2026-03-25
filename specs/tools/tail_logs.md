# Tool: `tail_logs`

## Description

> Read the last N lines of a log file (most recent entries). Equivalent to the Unix `tail` command. Useful for checking the latest activity in a log file.

---

## Input Schema

| Parameter   | Type     | Required | Default | Description                                      |
| ----------- | -------- | -------- | ------- | ------------------------------------------------ |
| `path`      | `string` | Yes      | —       | Path to the log file                             |
| `num_lines` | `int`    | No       | `50`    | Number of lines to read from the end (max 1000)  |

### Go Input Struct

```go
type TailLogsInput struct {
    Path     string `json:"path"      jsonschema:"required,description=Path to the log file"`
    NumLines int    `json:"num_lines" jsonschema:"description=Number of lines to read from the end of the file (max 1000),minimum=1,maximum=1000"`
}
```

### Default Values (applied in handler)

| Field      | Default |
| ---------- | ------- |
| `NumLines` | `50`    |

---

## Output Types

### Go Output Structs

```go
// LogLine represents a single line from the log file.
type LogLine struct {
    LineNumber int    `json:"line_number"`
    Content    string `json:"content"`
}

// TailLogsOutput is the top-level result returned by the tail_logs tool.
type TailLogsOutput struct {
    Lines           []LogLine `json:"lines"`
    TotalLines      int       `json:"total_lines"`
    FileSizeBytes   int64     `json:"file_size_bytes"`
    ShowingFromLine int       `json:"showing_from_line"`
}
```

| Field             | Type        | Description                                                  |
| ----------------- | ----------- | ------------------------------------------------------------ |
| `lines`           | `[]LogLine` | The last N lines from the file, each with its line number    |
| `total_lines`     | `int`       | Total number of lines in the file                            |
| `file_size_bytes` | `int64`     | Size of the file in bytes                                    |
| `showing_from_line` | `int`     | The line number of the first line in the returned set        |

---

## Handler Signature

```go
func handleTailLogs(ctx context.Context, req *mcp.CallToolRequest, input TailLogsInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "tail_logs",
    Description: "Read the last N lines of a log file (most recent entries). Equivalent to the Unix tail command. Useful for checking the latest activity in a log file.",
}, handleTailLogs)
```

---

## Output Format

```json
{
  "lines": [
    { "line_number": 54783, "content": "2025-01-15T23:59:56Z INFO [scheduler] Cron job completed: cleanup_temp" },
    { "line_number": 54784, "content": "2025-01-15T23:59:57Z DEBUG [cache] Evicted 23 expired entries" }
  ],
  "total_lines": 54832,
  "file_size_bytes": 4521984,
  "showing_from_line": 54783
}
```

---

## Example Usage Scenario

An AI assistant is asked "what's happening in the logs right now?" and uses `tail_logs` to quickly see the most recent entries without scanning the entire file.