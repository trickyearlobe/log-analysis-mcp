# Error Handling Strategy

All tools follow a consistent error handling pattern. Errors are returned as MCP tool results with `IsError: true`, never panics that would crash the server.

### Error Response Format

There are two categories of errors:

**Tool-level errors** (file not found, invalid input, binary file, etc.) — tool handlers return a Go `error` for tool-level failures. The SDK automatically packs these into a `*mcp.CallToolResult` with `IsError: true` and a JSON-serialized `ToolError` in the content. The `ToolError` includes a code (e.g. `FILE_NOT_FOUND`), a human-readable message, and an optional details map for structured context like the file path.

**Protocol-level errors** (malformed request, unknown tool name, missing required parameters) — return a `*jsonrpc.Error` with a standard JSON-RPC error code (e.g. `jsonrpc.InvalidParams`). These are reserved for situations where the request itself is invalid at the protocol level, not for domain-level failures in tool logic.

### Error Codes & Handling

| Error Code            | Condition                           | User-Facing Message                                                              |
| --------------------- | ----------------------------------- | -------------------------------------------------------------------------------- |
| `FILE_NOT_FOUND`      | File does not exist at given path   | `File not found: /path/to/file.log — verify the path is correct and accessible`  |
| `PERMISSION_DENIED`   | Insufficient read permissions       | `Permission denied: /path/to/file.log — check file permissions (current user: X)` |
| `BINARY_FILE`         | File detected as binary (non-text)  | `Binary file detected: /path/to/file — this tool only supports text log files`    |
| `FILE_TOO_LARGE`      | File exceeds processing limits      | `File is very large (X GB). Use pagination (log_read with start_line/num_lines) or sampling (log_summarize with sample_size)` |
| `INVALID_REGEX`       | Invalid regex pattern provided      | `Invalid regular expression: "pattern" — error details`                           |
| `INVALID_TIMESTAMP`   | Unparseable timestamp in filter     | `Invalid timestamp format: "value" — use ISO 8601 format (e.g., 2025-01-15T10:30:00Z)` |
| `PARSE_ERROR`         | Log format could not be determined  | `Could not detect log format. Try specifying format_hint parameter.`              |
| `ENCODING_ERROR`      | File encoding mismatch              | `Encoding error reading file with {encoding}. Try a different encoding (utf-8, ascii, latin1).` |
| `MAX_OUTPUT_EXCEEDED` | Output would exceed size limit      | `Results truncated to stay within output limits. Use more specific filters or pagination.` |

### Binary File Detection

Before processing, check the first 8192 bytes of the file for null bytes (`0x00`). If any null byte is found, treat the file as binary and return a `BINARY_FILE` error immediately. This prevents garbled output from accidentally processing compiled files, compressed archives, or media files.

### Partial Results

When a tool encounters errors on some lines but succeeds on others (e.g., parse errors in a log file), it returns partial results with a warnings array:

```json
{
  "records": [ ],
  "warnings": [
    { "line_number": 17, "message": "Line does not match expected format", "raw": "..." },
    { "line_number": 234, "message": "Timestamp could not be parsed", "raw": "..." }
  ],
  "total_parsed": 48,
  "total_warnings": 2
}
```
