# Tool: `timeline`

**Description (shown to LLM):**
> Build a chronological event timeline from log entries. Filters for significant events (errors, warnings, startups, shutdowns, deployments) and presents them in time order. Useful for understanding the sequence of events during an incident.

---

## Input Schema

| Parameter     | Type       | Required | Default | Description                                                  |
| ------------- | ---------- | -------- | ------- | ------------------------------------------------------------ |
| `path`        | `string`   | Yes      | —       | Path to the log file                                         |
| `after`       | `string`   | No       | —       | ISO 8601 timestamp — only include events after this time     |
| `before`      | `string`   | No       | —       | ISO 8601 timestamp — only include events before this time    |
| `event_types` | `[]string` | No       | —       | Types of events to include (e.g., `["ERROR", "WARN", "startup", "shutdown"]`). If omitted, includes ERROR, WARN, FATAL, and lifecycle events. |
| `max_events`  | `int`      | No       | `100`   | Maximum number of events to return (max 500)                 |

### Go Input Struct

```go
type TimelineInput struct {
    Path       string   `json:"path"        jsonschema:"required,description=Path to the log file"`
    After      string   `json:"after"       jsonschema:"description=ISO 8601 timestamp — include events after this time"`
    Before     string   `json:"before"      jsonschema:"description=ISO 8601 timestamp — include events before this time"`
    EventTypes []string `json:"event_types" jsonschema:"description=Event types to include (log levels or keywords like startup\\, shutdown\\, deploy)"`
    MaxEvents  int      `json:"max_events"  jsonschema:"description=Maximum number of events to return (max 500),minimum=1,maximum=500"`
}
```

### Default Values (applied in handler)

| Field       | Default |
| ----------- | ------- |
| `MaxEvents` | `100`   |

---

## Event Type Detection

Beyond log levels, the timeline tool detects lifecycle events by scanning message content for keywords:

| Event Type   | Keywords / Patterns                                          |
| ------------ | ------------------------------------------------------------ |
| `startup`    | "started", "listening on", "server ready", "boot complete"  |
| `shutdown`   | "shutting down", "stopped", "graceful shutdown", "SIGTERM"   |
| `deploy`     | "deployed", "deployment", "release", "version"               |
| `restart`    | "restarting", "restarted", "respawn"                         |
| `crash`      | "crash", "panic", "fatal", "core dump", "segfault"          |
| `connection` | "connected", "disconnected", "connection lost", "reconnect" |

---

## Output Types

> **Note:** `TimelineEvent` is the canonical definition from `internal/types/` (see `specs/types.md`).
> `TimeSpan` is tool-specific — it extends `types.TimeRange` with a `DurationMinutes` field and is distinct from it.

### Go Output Structs

```go
// TimelineEvent is defined in internal/types/types.go — repeated here for readability.
type TimelineEvent struct {
    Timestamp  string  `json:"timestamp"`
    Type       string  `json:"type"`
    Source     *string `json:"source"`
    Message    string  `json:"message"`
    LineNumber int     `json:"line_number"`
}

// TimeSpan is tool-specific (distinct from types.TimeRange; adds DurationMinutes).
type TimeSpan struct {
    Start           string  `json:"start"`
    End             string  `json:"end"`
    DurationMinutes float64 `json:"duration_minutes"`
}

type TimelineOutput struct {
    Events     []TimelineEvent `json:"events"`
    TimeSpan   TimeSpan        `json:"time_span"`
    EventCount int             `json:"event_count"`
    Truncated  bool            `json:"truncated"`
}
```

---

## Handler Signature

```go
func handleTimeline(ctx context.Context, req *mcp.CallToolRequest, input TimelineInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "timeline",
    Description: "Build a chronological event timeline from log entries. Filters for significant events (errors, warnings, startups, shutdowns, deployments) and presents them in time order. Useful for understanding the sequence of events during an incident.",
}, handleTimeline)
```

---

## Output Format

```json
{
  "events": [
    {
      "timestamp": "2025-01-15T14:28:00.000Z",
      "type": "INFO",
      "source": "deploy",
      "message": "Deployment started: version 2.4.1",
      "line_number": 31200
    },
    {
      "timestamp": "2025-01-15T14:30:05.000Z",
      "type": "startup",
      "source": "app",
      "message": "Server started on port 3000",
      "line_number": 31245
    },
    {
      "timestamp": "2025-01-15T14:31:02.000Z",
      "type": "ERROR",
      "source": "db",
      "message": "Connection refused to primary database",
      "line_number": 31302
    }
  ],
  "time_span": {
    "start": "2025-01-15T14:28:00.000Z",
    "end": "2025-01-15T14:35:00.000Z",
    "duration_minutes": 7
  },
  "event_count": 42,
  "truncated": false
}
```

---

## Algorithm / Strategy

1. **Parse the log file** using the auto-detect parser to extract structured records with timestamps.
2. **Apply time-range filters** — if `after` and/or `before` are provided, discard entries outside the window.
3. **Classify each entry by event type:**
   - First, check the parsed log level (ERROR, WARN, FATAL, INFO, DEBUG, etc.).
   - Then, scan the message content against the lifecycle keyword table above to detect `startup`, `shutdown`, `deploy`, `restart`, `crash`, and `connection` events.
   - A single entry may match both a log level and a lifecycle keyword; prefer the lifecycle classification when it matches (it is more specific).
4. **Filter by `event_types`** — if the caller provided an explicit list, only keep entries whose classified type appears in the list. If `event_types` is omitted, keep entries with types: `ERROR`, `WARN`, `FATAL`, `startup`, `shutdown`, `deploy`, `restart`, `crash`, and `connection`.
5. **Sort all surviving entries chronologically** by timestamp.
6. **Truncate** to `max_events` entries. If truncation occurs, set `truncated: true` in the output.
7. **Compute the time span** from the first event's timestamp to the last event's timestamp and calculate `duration_minutes`.
8. **Return** the structured `TimelineOutput`.

---

## Example Usage Scenario

An AI assistant is building an incident timeline. It calls `timeline` with `after` and `before` timestamps bracketing the incident, filtering for ERROR, WARN, and lifecycle events, to produce a clear chronological narrative.