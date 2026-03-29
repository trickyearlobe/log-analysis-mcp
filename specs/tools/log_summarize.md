# Tool: `log_summarize`

**Description (shown to LLM):**
> Generate a statistical summary of a log file including line counts, severity level distribution, top error messages, most active sources, and throughput metrics. Provides a quick overview of the file's contents without reading every line.

---

## Input Schema

| Parameter     | Type     | Required | Default | Description                                       |
| ------------- | -------- | -------- | ------- | ------------------------------------------------- |
| `path`        | `string` | Yes      | —       | Path to the log file                              |
| `sample_size` | `int`    | No       | `0`     | Number of lines to sample (0 = analyze all lines) |

### Go Input Struct

```go
type SummarizeLogsInput struct {
    Path       string `json:"path"        jsonschema:"required,description=Path to the log file"`
    SampleSize int    `json:"sample_size" jsonschema:"description=Number of lines to sample; 0 means analyze all lines,minimum=0"`
}
```

### Default Values (applied in handler)

| Field        | Default |
| ------------ | ------- |
| `SampleSize` | `0`     |

---

## Output Types

### Go Output Structs

```go
type FileInfoSummary struct {
    Name       string         `json:"name"`
    Path       string         `json:"path"`
    SizeBytes  int64          `json:"size_bytes"`
    SizeHuman  string         `json:"size_human"`
    TotalLines int            `json:"total_lines"`
    TimeRange  *TimeRangeInfo `json:"time_range"`
}

type TimeRangeInfo struct {
    Earliest      string  `json:"earliest"`
    Latest        string  `json:"latest"`
    DurationHours float64 `json:"duration_hours"`
}

type LevelStats struct {
    Count      int     `json:"count"`
    Percentage float64 `json:"percentage"`
}

type SourceCount struct {
    Source string `json:"source"`
    Count  int    `json:"count"`
}

type ErrorCount struct {
    Message string `json:"message"`
    Count   int    `json:"count"`
}

type MinuteStats struct {
    Timestamp string `json:"timestamp"`
    Count     int    `json:"count"`
}

type ThroughputInfo struct {
    LinesPerMinute float64     `json:"lines_per_minute"`
    PeakMinute     MinuteStats `json:"peak_minute"`
    QuietestMinute MinuteStats `json:"quietest_minute"`
}

type SummarizeLogsOutput struct {
    FileInfo          FileInfoSummary       `json:"file_info"`
    DetectedFormat    string                `json:"detected_format"`
    LevelDistribution map[string]LevelStats `json:"level_distribution"`
    TopSources        []SourceCount         `json:"top_sources"`
    TopErrors         []ErrorCount          `json:"top_errors"`
    Throughput        ThroughputInfo        `json:"throughput"`
    Sampled           bool                  `json:"sampled"`
    LinesAnalyzed     int                   `json:"lines_analyzed"`
}
```

---

## Handler Signature

```go
func handleSummarizeLogs(ctx context.Context, req *mcp.CallToolRequest, input SummarizeLogsInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "log_summarize",
    Description: "Generate a statistical summary of a log file including line counts, severity level distribution, top error messages, most active sources, and throughput metrics. Provides a quick overview of the file's contents without reading every line.",
}, handleSummarizeLogs)
```

---

## Output Format

```json
{
  "file_info": {
    "name": "application.log",
    "path": "/var/log/application.log",
    "size_bytes": 4521984,
    "size_human": "4.3 MB",
    "total_lines": 54832,
    "time_range": {
      "earliest": "2025-01-15T00:00:01.000Z",
      "latest": "2025-01-15T23:59:58.000Z",
      "duration_hours": 23.99
    }
  },
  "detected_format": "json",
  "level_distribution": {
    "DEBUG": { "count": 28410, "percentage": 51.8 },
    "INFO":  { "count": 21933, "percentage": 40.0 },
    "WARN":  { "count": 3201,  "percentage": 5.8 },
    "ERROR": { "count": 1188,  "percentage": 2.2 },
    "FATAL": { "count": 100,   "percentage": 0.2 }
  },
  "top_sources": [
    { "source": "api-gateway", "count": 18234 },
    { "source": "auth-service", "count": 12001 },
    { "source": "db-pool", "count": 9876 },
    { "source": "cache", "count": 8721 },
    { "source": "scheduler", "count": 6000 }
  ],
  "top_errors": [
    { "message": "Connection timeout to database", "count": 412 },
    { "message": "Rate limit exceeded", "count": 298 },
    { "message": "Invalid authentication token", "count": 187 },
    { "message": "File not found: /api/v2/users", "count": 156 },
    { "message": "Out of memory in worker process", "count": 135 }
  ],
  "throughput": {
    "lines_per_minute": 38.1,
    "peak_minute": { "timestamp": "2025-01-15T14:32:00.000Z", "count": 312 },
    "quietest_minute": { "timestamp": "2025-01-15T03:17:00.000Z", "count": 2 }
  },
  "sampled": false,
  "lines_analyzed": 54832
}
```

---

## Example Usage Scenario

An AI assistant receives a log file and wants to quickly understand what's in it before diving deeper. It calls `log_summarize` to get an overview, then uses the top errors and level distribution to decide where to investigate next.