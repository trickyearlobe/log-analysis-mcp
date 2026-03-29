# Tool: `log_extract_errors`

## Description (shown to LLM)

> Extract all error messages and exceptions from a log file, then cluster them by similarity to identify distinct error types. Includes stack traces when available. Useful for understanding the error landscape of a system.

## Input Schema

| Parameter              | Type     | Required | Default | Description                                          |
| ---------------------- | -------- | -------- | ------- | ---------------------------------------------------- |
| `path`                 | `string` | Yes      | —       | Path to the log file                                 |
| `include_stack_traces` | `bool`   | No       | `true`  | Whether to capture multiline stack traces            |
| `max_clusters`         | `int`    | No       | `20`    | Maximum number of error clusters to return (max 100) |

## Go Input Struct

```go
type ExtractErrorsInput struct {
    Path               string `json:"path"                 jsonschema:"required,description=Path to the log file"`
    IncludeStackTraces bool   `json:"include_stack_traces" jsonschema:"description=Capture multiline stack traces with errors"`
    MaxClusters        int    `json:"max_clusters"         jsonschema:"description=Maximum number of error clusters to return (max 100),minimum=1,maximum=100"`
}
```

## Default Values (applied in handler)

| Field                | Default |
| -------------------- | ------- |
| `IncludeStackTraces` | `true`  |
| `MaxClusters`        | `20`    |

## Clustering Algorithm

Errors are clustered using a simplified text-similarity approach:

1. Normalize each error message by replacing variable parts (IDs, timestamps, IP addresses, file paths, numbers) with placeholders.
2. Group messages with identical normalized forms into clusters.
3. Sort clusters by count (descending).
4. Return top N clusters with representative samples.

## Go Output Structs

> **Note:** `SeenAt` and `ErrorCluster` are canonical shared types defined in `specs/types.md` (`internal/types`). They are reproduced here for readability.

```go
// Canonical definition: internal/types/types.go (see specs/types.md)
type SeenAt struct {
    Timestamp  *string `json:"timestamp"`
    LineNumber int     `json:"line_number"`
}

// Canonical definition: internal/types/types.go (see specs/types.md)
type ErrorCluster struct {
    Pattern        string   `json:"pattern"`
    Count          int      `json:"count"`
    Percentage     float64  `json:"percentage"`
    FirstSeen      SeenAt   `json:"first_seen"`
    LastSeen       SeenAt   `json:"last_seen"`
    SampleMessages []string `json:"sample_messages"`
    StackTrace     *string  `json:"stack_trace"`
}

type ErrorRate struct {
    ErrorsPerHour        float64 `json:"errors_per_hour"`
    PercentageOfAllLines float64 `json:"percentage_of_all_lines"`
}

type ExtractErrorsOutput struct {
    Clusters       []ErrorCluster `json:"clusters"`
    TotalErrors    int            `json:"total_errors"`
    ErrorRate      ErrorRate      `json:"error_rate"`
    LevelsIncluded []string       `json:"levels_included"`
}
```

## Handler Signature

```go
func handleExtractErrors(ctx context.Context, req *mcp.CallToolRequest, input ExtractErrorsInput) (*mcp.CallToolResult, error)
```

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "log_extract_errors",
    Description: "Extract all error messages and exceptions from a log file, then cluster them by similarity to identify distinct error types. Includes stack traces when available. Useful for understanding the error landscape of a system.",
}, handleExtractErrors)
```

## Output Format

```json
{
  "clusters": [
    {
      "pattern": "Connection timeout to database (host: <IP>, port: <NUM>)",
      "count": 412,
      "percentage": 34.7,
      "first_seen": {
        "timestamp": "2025-01-15T02:14:33.000Z",
        "line_number": 4521
      },
      "last_seen": {
        "timestamp": "2025-01-15T22:58:01.000Z",
        "line_number": 53201
      },
      "sample_messages": [
        "Connection timeout to database (host: 10.0.1.5, port: 5432)",
        "Connection timeout to database (host: 10.0.1.6, port: 5432)"
      ],
      "stack_trace": null
    },
    {
      "pattern": "NullPointerException in <CLASS>.<METHOD>",
      "count": 87,
      "percentage": 7.3,
      "first_seen": {
        "timestamp": "2025-01-15T08:10:12.000Z",
        "line_number": 12034
      },
      "last_seen": {
        "timestamp": "2025-01-15T21:45:55.000Z",
        "line_number": 51002
      },
      "sample_messages": [
        "NullPointerException in UserService.getProfile"
      ],
      "stack_trace": "java.lang.NullPointerException\n\tat com.example.UserService.getProfile(UserService.java:142)\n\tat com.example.ApiController.handleRequest(ApiController.java:89)"
    }
  ],
  "total_errors": 1188,
  "error_rate": {
    "errors_per_hour": 49.5,
    "percentage_of_all_lines": 2.2
  },
  "levels_included": ["ERROR", "FATAL", "CRITICAL"]
}
```

## Example Usage Scenario

An AI assistant needs to understand the error landscape of a production system. It calls `log_extract_errors` to see all unique error types, their frequencies, and representative stack traces, enabling quick prioritization.