## Resource Definitions

### `log:///{path}` — Log File Resource Template

**URI Template:** `log:///{path}`

The resource template allows AI assistants to access metadata about a log file as an MCP resource. The `{path}` parameter is the absolute or relative file path.

**Registration:**

```go
mcp.AddResourceTemplate(server, &mcp.ResourceTemplate{
    URITemplate: "log:///{path}",
    Name:        "log-file",
    Description: "Log file metadata and preview",
    MIMEType:    "text/plain",
}, handleLogFileResource)
```

**Resource Metadata Returned:**

```json
{
  "uri": "log:///var/log/application.log",
  "name": "application.log",
  "description": "Log file: application.log (4.3 MB, 54832 lines, JSON format)",
  "mimeType": "text/plain",
  "metadata": {
    "path": "/var/log/application.log",
    "size_bytes": 4521984,
    "size_human": "4.3 MB",
    "total_lines": 54832,
    "last_modified": "2025-01-15T23:59:58.000Z",
    "detected_format": "json",
    "time_range": {
      "start": "2025-01-15T00:00:01.000Z",
      "end": "2025-01-15T23:59:58.000Z"
    },
    "permissions": {
      "readable": true,
      "writable": false
    }
  }
}
```

**Content Returned (read):**

When the resource is read, it returns the first 100 lines of the file as text content, along with a note indicating whether there are more lines.

---

## Prompt Definitions

### `investigate_error`

**Purpose:** Guide the AI through a structured error investigation workflow.

**Arguments:**

| Argument        | Type     | Required | Description                                     |
| --------------- | -------- | -------- | ----------------------------------------------- |
| `log_path`      | `string` | Yes      | Path to the log file to investigate              |
| `error_pattern` | `string` | No       | Specific error pattern to look for               |

**Registration:**

```go
mcp.AddPrompt(server, &mcp.Prompt{
    Name:        "investigate_error",
    Description: "Guided error investigation workflow for a log file",
    Arguments: []mcp.PromptArgument{
        {Name: "log_path", Description: "Path to the log file to investigate", Required: true},
        {Name: "error_pattern", Description: "Specific error pattern to look for", Required: false},
    },
}, handleInvestigateErrorPrompt)
```

**Prompt Text:**

The prompt returns a single `user` role message. The text is constructed from `log_path` and the optional `error_pattern` argument:

> I need you to investigate errors in the log file at "{log_path}". Specifically, look for errors matching: "{error_pattern}".
>
> Please follow this structured investigation process:
>
> 1. **Overview**: First, use the summarize_logs tool to get an overview of the log file. Report the total error count, error rate, and time range.
>
> 2. **Error Extraction**: Use the extract_errors tool to identify and cluster all unique error types. List the top 10 error clusters by frequency.
>
> 3. **Timeline Analysis**: Use the timeline tool to build a timeline of error events matching "{error_pattern}". Identify when errors started, whether they correlate with other events (deployments, restarts), and if there are patterns in timing.
>
> 4. **Context Gathering**: For the most significant error(s), use search_logs with context_lines=5 to see what happened immediately before and after each occurrence.
>
> 5. **Anomaly Detection**: Use detect_anomalies to check for error spikes or unusual patterns that might indicate a trigger event.
>
> 6. **Root Cause Analysis**: Based on all the evidence gathered, provide:
>    - A summary of findings
>    - The most likely root cause
>    - The sequence of events that led to the errors
>    - Recommended next steps for remediation
>
> Format your analysis as a structured report with clear headings.

When `error_pattern` is omitted, the sentence "Specifically, look for errors matching: ..." is removed, and "matching ..." in step 3 is removed.

**Example Usage Scenario:**
A developer tells the AI "investigate the errors in /var/log/app.log" and the AI uses this prompt to follow a systematic investigation process rather than ad-hoc exploration.

---

### `log_health_check`

**Purpose:** System health assessment from log analysis.

**Arguments:**

| Argument   | Type     | Required | Description                        |
| ---------- | -------- | -------- | ---------------------------------- |
| `log_path` | `string` | Yes      | Path to the log file to assess     |

**Registration:**

```go
mcp.AddPrompt(server, &mcp.Prompt{
    Name:        "log_health_check",
    Description: "System health assessment from log analysis",
    Arguments: []mcp.PromptArgument{
        {Name: "log_path", Description: "Path to the log file to assess", Required: true},
    },
}, handleLogHealthCheckPrompt)
```

**Prompt Text:**

The prompt returns a single `user` role message. The text is constructed from `log_path`:

> Please perform a health check on the system by analyzing the log file at "{log_path}".
>
> Use the following tools to conduct a comprehensive assessment:
>
> 1. **File Overview**: Use summarize_logs to get a high-level picture:
>    - How large is the file and what time period does it cover?
>    - What is the overall log volume and throughput?
>
> 2. **Error Assessment**:
>    - What percentage of log entries are errors or warnings?
>    - Use extract_errors to identify the most common error types.
>    - Is the error rate acceptable (< 1% is healthy, 1-5% is concerning, > 5% is critical)?
>
> 3. **Anomaly Scan**: Use detect_anomalies to check for:
>    - Error spikes that might indicate instability
>    - Gaps in logging that might indicate outages
>    - Rate changes that might indicate load issues
>
> 4. **Recent Activity**: Use tail_logs to check the most recent entries:
>    - Is the system currently logging normally?
>    - Are there any active errors or warnings?
>
> 5. **Health Report**: Provide a structured health report with:
>    - **Overall Status**: 🟢 Healthy / 🟡 Warning / 🔴 Critical
>    - **Error Rate**: Current error rate with assessment
>    - **Top Issues**: List of the most important issues found
>    - **Anomalies**: Any detected anomalies
>    - **Recommendations**: Prioritized list of recommended actions
>    - **Summary**: A one-paragraph executive summary
>
> Be specific with numbers and evidence from the logs.

**Example Usage Scenario:**
An ops engineer asks "how healthy is our system?" and the AI uses this prompt to conduct a thorough health assessment using multiple tools in a structured sequence.