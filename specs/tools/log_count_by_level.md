# Tool: `log_count_by_level`

**Description (shown to LLM):**
> Count log entries by severity level. A fast single-pass tool for multi-file triage — returns level counts with minimal output to save context tokens.

---

## Input Schema

| Parameter | Type     | Required | Default | Description                    |
| --------- | -------- | -------- | ------- | ------------------------------ |
| `path`    | `string` | Yes      | —       | Path to the log file           |

### Go Input Struct

```go
type CountByLevelInput struct {
    Path string `json:"path" jsonschema:"Path to the log file"`
}
```

---

## Output Format

### Go Output Structs

```go
type CountByLevelOutput struct {
    Counts      map[string]int `json:"counts"`
    TotalLines  int            `json:"total_lines"`
    ParsedLines int            `json:"parsed_lines"`
}
```

### JSON Example

```json
{
  "counts": {
    "ERROR": 47,
    "WARN": 152,
    "INFO": 8432,
    "DEBUG": 2103
  },
  "total_lines": 10734,
  "parsed_lines": 10734
}
```

---

## Behaviour

1. Auto-detect log format from first 10 lines.
2. Stream entire file line-by-line counting levels.
3. If no parser matches, use keyword-level inference (`inferLevelFromText`).
4. Lines with no detectable level are counted in `total_lines` but not `parsed_lines`.

---

## Handler Signature

```go
func handleCountByLevel(ctx context.Context, req *mcp.CallToolRequest, input CountByLevelInput) (*mcp.CallToolResult, error)
```
