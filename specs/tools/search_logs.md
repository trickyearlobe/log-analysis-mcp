# Tool: `search_logs`

**Description (shown to LLM):**
> Search a log file using regex or text patterns. Returns matching lines with optional surrounding context. Useful for finding specific errors, request IDs, or patterns in log files.

---

## Input Schema

| Parameter        | Type     | Required | Default | Description                                                |
| ---------------- | -------- | -------- | ------- | ---------------------------------------------------------- |
| `path`           | `string` | Yes      | —       | Path to the log file to search                             |
| `pattern`        | `string` | Yes      | —       | Search pattern (plain text or regex)                       |
| `is_regex`       | `bool`   | No       | `false` | Whether to interpret pattern as a regular expression       |
| `case_sensitive` | `bool`   | No       | `false` | Whether the search should be case-sensitive                |
| `context_lines`  | `int`    | No       | `0`     | Number of lines to include before and after each match     |
| `max_results`    | `int`    | No       | `50`    | Maximum number of matches to return (max 500)              |

### Go Input Struct

```go
type SearchLogsInput struct {
    Path          string `json:"path"           jsonschema:"required,description=Path to the log file to search"`
    Pattern       string `json:"pattern"        jsonschema:"required,description=Search pattern (plain text or regex)"`
    IsRegex       bool   `json:"is_regex"       jsonschema:"description=Treat pattern as a regular expression"`
    CaseSensitive bool   `json:"case_sensitive" jsonschema:"description=Case-sensitive search"`
    ContextLines  int    `json:"context_lines"  jsonschema:"description=Lines of context before and after each match,minimum=0,maximum=10"`
    MaxResults    int    `json:"max_results"    jsonschema:"description=Maximum number of matches to return (max 500),minimum=1,maximum=500"`
}
```

### Default Values (applied in handler)

| Field           | Default |
| --------------- | ------- |
| `IsRegex`       | `false` |
| `CaseSensitive` | `false` |
| `ContextLines`  | `0`     |
| `MaxResults`    | `50`    |

---

## Output Format

### Go Output Structs

> **Note:** `SearchMatch` is canonically defined in `specs/types.md` (`types.SearchMatch`). The definition is repeated here for readability.

```go
type SearchMatch struct {
    LineNumber    int      `json:"line_number"`
    Line          string   `json:"line"`
    BeforeContext []string `json:"before_context"`
    AfterContext  []string `json:"after_context"`
}

type SearchLogsOutput struct {
    Matches       []SearchMatch `json:"matches"`
    TotalMatches  int           `json:"total_matches"`
    SearchedLines int           `json:"searched_lines"`
    PatternUsed   string        `json:"pattern_used"`
    Truncated     bool          `json:"truncated"`
}
```

### JSON Example

```json
{
  "matches": [
    {
      "line_number": 1523,
      "line": "2025-01-15T10:45:23Z ERROR [auth] Failed login attempt for user admin@example.com",
      "before_context": [
        "2025-01-15T10:45:22Z INFO [auth] Login attempt initiated for user admin@example.com"
      ],
      "after_context": [
        "2025-01-15T10:45:23Z WARN [auth] Account lockout threshold approaching for admin@example.com"
      ]
    }
  ],
  "total_matches": 47,
  "searched_lines": 54832,
  "pattern_used": "Failed login",
  "truncated": false
}
```

---

## Handler Signature

```go
func handleSearchLogs(ctx context.Context, req *mcp.CallToolRequest, input SearchLogsInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "search_logs",
    Description: "Search a log file using regex or text patterns. Returns matching lines with optional surrounding context. Useful for finding specific errors, request IDs, or patterns in log files.",
}, handleSearchLogs)
```

---

## Usage Scenario

An AI assistant investigating a reported authentication issue searches for `"Failed login"` with 2 context lines to understand the sequence of events around each failure.