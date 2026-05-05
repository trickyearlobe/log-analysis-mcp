# Tool: `log_detect_anomalies`

**Description (shown to LLM):**
> Detect anomalous patterns in log files such as sudden error spikes, new error types that haven't appeared before, gaps in logging (periods with no entries), and significant changes in log volume. Analyzes the temporal distribution of log entries to find unusual behavior.

---

## Input Schema

| Parameter        | Type     | Required | Default    | Description                                              |
| ---------------- | -------- | -------- | ---------- | -------------------------------------------------------- |
| `path`           | `string` | Yes      | —          | Path to the log file                                     |
| `window_minutes` | `int`    | No       | `5`        | Time window size in minutes for rate analysis            |
| `sensitivity`    | `string` | No       | `"medium"` | Detection sensitivity: low, medium, or high              |
| `max_results`    | `int`    | No       | `50`       | Maximum number of anomalies to return (max 200)          |
| `offset`         | `int`    | No       | `0`        | Number of anomalies to skip for pagination               |

### Go Input Struct

```go
type DetectAnomaliesInput struct {
    Path          string `json:"path"           jsonschema:"required,description=Path to the log file"`
    WindowMinutes int    `json:"window_minutes" jsonschema:"description=Time window in minutes for rate analysis,minimum=1,maximum=60"`
    Sensitivity   string `json:"sensitivity"    jsonschema:"description=Detection sensitivity level,enum=low,enum=medium,enum=high"`
    MaxResults    int    `json:"max_results"    jsonschema:"description=Maximum number of anomalies to return (max 200),minimum=1,maximum=200"`
    Offset        int    `json:"offset"         jsonschema:"description=Number of anomalies to skip for pagination"`
}
```

### Default Values (applied in handler)

| Field           | Default    |
| --------------- | ---------- |
| `WindowMinutes` | `5`        |
| `Sensitivity`   | `"medium"` |
| `MaxResults`    | `50`       |
| `Offset`        | `0`        |

### Sensitivity Thresholds

| Sensitivity | Error Spike      | Rate Change       | Gap Detection        |
| ----------- | ---------------- | ----------------- | -------------------- |
| `low`       | > 5× baseline    | > 5× avg rate     | > 10× avg interval   |
| `medium`    | > 3× baseline    | > 3× avg rate     | > 5× avg interval    |
| `high`      | > 2× baseline    | > 2× avg rate     | > 3× avg interval    |

---

## Output Types

> Types from `internal/types/` are referenced. See `specs/types.md` for shared type definitions.

### Go Output Structs

```go
// EvidenceLine is defined in internal/types (see specs/types.md).
type EvidenceLine struct {
    LineNumber int    `json:"line_number"`
    Content    string `json:"content"`
}

// TimeRange is defined in internal/types (see specs/types.md).
type TimeRange struct {
    Start string `json:"start"`
    End   string `json:"end"`
}

type Anomaly struct {
    Type          string                 `json:"type"`
    Severity      string                 `json:"severity"`
    Description   string                 `json:"description"`
    TimeRange     TimeRange              `json:"time_range"`
    Details       map[string]interface{} `json:"details"`
    EvidenceLines []EvidenceLine         `json:"evidence_lines"`
}

type AnalysisMetadata struct {
    TotalLinesAnalyzed int     `json:"total_lines_analyzed"`
    TimeSpanHours      float64 `json:"time_span_hours"`
    WindowMinutes      int     `json:"window_minutes"`
    Sensitivity        string  `json:"sensitivity"`
    WindowsAnalyzed    int     `json:"windows_analyzed"`
}

type DetectAnomaliesOutput struct {
    Anomalies        []Anomaly        `json:"anomalies"`
    TotalAnomalies   int              `json:"total_anomalies"`
    AnalysisMetadata AnalysisMetadata `json:"analysis_metadata"`
    HasMore          bool             `json:"has_more"`
    NextOffset       int              `json:"next_offset"`
}
```

---

## Handler Signature

```go
func handleDetectAnomalies(ctx context.Context, req *mcp.CallToolRequest, input DetectAnomaliesInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "log_detect_anomalies",
    Description: "Detect anomalous patterns in log files such as sudden error spikes, new error types that haven't appeared before, gaps in logging (periods with no entries), and significant changes in log volume. Analyzes the temporal distribution of log entries to find unusual behavior.",
}, handleDetectAnomalies)
```

---

## Output Format

```json
{
  "anomalies": [
    {
      "type": "error_spike",
      "severity": "high",
      "description": "Error rate increased 8.2x compared to baseline in 5-minute window",
      "time_range": {
        "start": "2025-01-15T14:30:00.000Z",
        "end": "2025-01-15T14:35:00.000Z"
      },
      "details": {
        "baseline_error_rate": 2.1,
        "spike_error_rate": 17.2,
        "multiplier": 8.2
      },
      "evidence_lines": [
        { "line_number": 32451, "content": "2025-01-15T14:31:02Z ERROR [db] Connection refused..." },
        { "line_number": 32455, "content": "2025-01-15T14:31:03Z ERROR [db] Connection refused..." }
      ]
    },
    {
      "type": "gap",
      "severity": "medium",
      "description": "No log entries for 12 minutes (expected interval: ~0.5 seconds)",
      "time_range": {
        "start": "2025-01-15T03:14:00.000Z",
        "end": "2025-01-15T03:26:00.000Z"
      },
      "details": {
        "gap_duration_seconds": 720,
        "avg_interval_seconds": 0.5
      },
      "evidence_lines": []
    },
    {
      "type": "new_error_type",
      "severity": "medium",
      "description": "New error pattern appeared that was not seen in the first 80% of the file",
      "time_range": {
        "start": "2025-01-15T18:22:00.000Z",
        "end": "2025-01-15T18:22:00.000Z"
      },
      "details": {
        "pattern": "SSL handshake failed",
        "occurrences": 14,
        "first_seen_line": 41023
      },
      "evidence_lines": [
        { "line_number": 41023, "content": "2025-01-15T18:22:01Z ERROR [tls] SSL handshake failed: certificate expired" }
      ]
    }
  ],
  "analysis_metadata": {
    "total_lines_analyzed": 54832,
    "time_span_hours": 23.99,
    "window_minutes": 5,
    "sensitivity": "medium",
    "windows_analyzed": 288
  }
}
```

---

## Algorithm Details

The anomaly detection handler performs the following steps:

1. **Stream the log file** line-by-line, parsing timestamps and log levels from each entry.
2. **Bucket entries into time windows** of `window_minutes` duration for rate analysis.
3. **Compute baselines** — calculate average error rate, average log volume, and average inter-entry interval across all windows.
4. **Detect error spikes** — compare each window's error count against the baseline using the sensitivity multiplier thresholds. Windows exceeding the threshold are flagged as `error_spike` anomalies.
5. **Detect rate changes** — compare each window's total log volume against the average rate using the sensitivity multiplier thresholds. Significant increases or decreases are flagged.
6. **Detect gaps** — identify consecutive entries where the time interval exceeds the average interval multiplied by the gap detection threshold. These are flagged as `gap` anomalies.
7. **Detect new error types** — collect error message patterns from the first 80% of the file, then identify any new patterns that appear only in the last 20%. These are flagged as `new_error_type` anomalies.
8. **Collect evidence lines** — for each anomaly, attach a small number of representative log lines (with line numbers) as evidence.
9. **Assemble metadata** — record total lines analyzed, time span, window configuration, sensitivity level, and number of windows analyzed.
10. **Return results** sorted by severity (high → medium → low), then by time range start.

---

## Example Usage Scenario

An AI assistant is investigating why a service degraded at a particular time. It calls `log_detect_anomalies` to automatically find error spikes and rate changes without manually scanning through the file.