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
> 1. **Overview**: First, use the log_summarize tool to get an overview of the log file. Report the total error count, error rate, and time range.
>
> 2. **Error Extraction**: Use the log_extract_errors tool to identify and cluster all unique error types. List the top 10 error clusters by frequency.
>
> 3. **Timeline Analysis**: Use the timeline tool to build a timeline of error events matching "{error_pattern}". Identify when errors started, whether they correlate with other events (deployments, restarts), and if there are patterns in timing.
>
> 4. **Context Gathering**: For the most significant error(s), use log_search with context_lines=5 to see what happened immediately before and after each occurrence.
>
> 5. **Anomaly Detection**: Use log_detect_anomalies to check for error spikes or unusual patterns that might indicate a trigger event.
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
> 1. **File Overview**: Use log_summarize to get a high-level picture:
>    - How large is the file and what time period does it cover?
>    - What is the overall log volume and throughput?
>
> 2. **Error Assessment**:
>    - What percentage of log entries are errors or warnings?
>    - Use log_extract_errors to identify the most common error types.
>    - Is the error rate acceptable (< 1% is healthy, 1-5% is concerning, > 5% is critical)?
>
> 3. **Anomaly Scan**: Use log_detect_anomalies to check for:
>    - Error spikes that might indicate instability
>    - Gaps in logging that might indicate outages
>    - Rate changes that might indicate load issues
>
> 4. **Recent Activity**: Use log_tail to check the most recent entries:
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

---

### `generate_report`

**Purpose:** Guide the AI through a comprehensive multi-tool investigation to produce a structured Markdown incident report.

**Arguments:**

| Argument          | Type     | Required | Description                                              |
| ----------------- | -------- | -------- | -------------------------------------------------------- |
| `log_path`        | `string` | Yes      | Path to the primary log file to investigate               |
| `comparison_path` | `string` | No       | Path to a baseline log file for before/after comparison   |
| `incident_id`     | `string` | No       | Incident or ticket ID to include in the report header     |

**Registration:**

```go
srv.AddPrompt(&mcp.Prompt{
    Name:        "generate_report",
    Description: "Generate a structured incident report from log analysis",
    Arguments: []*mcp.PromptArgument{
        {Name: "log_path", Description: "Path to the primary log file to investigate", Required: true},
        {Name: "comparison_path", Description: "Path to a baseline log file for before/after comparison", Required: false},
        {Name: "incident_id", Description: "Incident or ticket ID to include in the report header", Required: false},
    },
}, handleGenerateReport)
```

**Prompt Text:**

The prompt returns a single `user` role message. The text is constructed from `log_path` and the optional arguments.

When `incident_id` is provided, the report header instruction includes it:
> Include the incident ID "{incident_id}" in the report header.

When `comparison_path` is provided, a diff step is included:
> **Comparison Analysis**: Use the log_diff tool to compare "{log_path}" (target) against "{comparison_path}" (baseline). Report new errors, resolved errors, rate changes, source changes, and throughput shifts.

When `comparison_path` is omitted, the diff step is replaced with:
> **Comparison Analysis**: No baseline file was provided, so skip the comparison step.

The full prompt text (with all optional sections present):

> Generate a comprehensive incident report by analyzing the log file at "{log_path}".
> Include the incident ID "{incident_id}" in the report header.
>
> Follow this structured investigation process, using the tools listed for each step:
>
> 1. **Executive Summary** (after completing all steps below):
>    Summarize the incident in 2-3 sentences: what happened, when, impact, and current status.
>
> 2. **System Overview**: Use log_summarize to establish baseline metrics:
>    - File size, time range, and total line count
>    - Log volume and throughput (lines/minute)
>    - Detected log format
>
> 3. **Error Analysis**: Use log_extract_errors to identify and cluster all error types:
>    - List the top 10 error clusters by frequency
>    - Note error rate (errors/hour and percentage of all lines)
>    - Identify any error patterns that suggest a root cause
>
> 4. **Anomaly Detection**: Use log_detect_anomalies to find unusual patterns:
>    - Error spikes (sudden increases in error rate)
>    - Gaps in logging (possible outages or restarts)
>    - Rate changes (load shifts)
>    - New error types not seen before
>
> 5. **Comparison Analysis**: Use the log_diff tool to compare "{log_path}" (target) against "{comparison_path}" (baseline). Report new errors, resolved errors, rate changes, source changes, and throughput shifts.
>
> 6. **Timeline**: Use the timeline tool to build a chronological sequence of significant events:
>    - When did the incident start?
>    - What were the key events leading up to the incident?
>    - When was it resolved (if applicable)?
>
> 7. **Deep Dive**: For the top 3 most significant errors, use log_search with context_lines=5 to examine surrounding context. Look for:
>    - What triggered each error
>    - Whether errors cascade (one causing another)
>    - Any recovery attempts visible in the logs
>
> 8. **Report**: Compile all findings into a structured Markdown report with these sections:
>    - **Incident Report: {incident_id}** (header)
>    - **Executive Summary**
>    - **Timeline of Events** (chronological table)
>    - **Error Analysis** (clusters, rates, patterns)
>    - **Anomalies Detected**
>    - **Comparison with Baseline** (if comparison_path was provided)
>    - **Root Cause Analysis** (your assessment based on evidence)
>    - **Impact Assessment** (what was affected, duration, severity)
>    - **Recommendations** (prioritized remediation steps)
>    - **Appendix: Raw Data** (key metrics, tool outputs referenced)
>
> Be specific with numbers, timestamps, and evidence from the logs. Every claim in the report should be backed by data from one of the tools.

**Example Usage Scenarios:**

1. An SRE says "generate a report for the outage in /var/log/app.log, incident INC-2025-042" — the AI follows the full workflow with the incident ID in the header.
2. A developer says "compare today's logs against yesterday's and write a report" providing both paths — the AI includes the log_diff comparison section.
3. A team lead says "analyze /var/log/nginx/error.log and write up what happened" — the AI skips the comparison step and omits the incident ID.

---

### `investigate_remote`

**Purpose:** Guide the AI through a multi-system remote log investigation workflow, from discovery through analysis to a structured report.

**Arguments:**

| Argument      | Type     | Required | Description                                              |
| ------------- | -------- | -------- | -------------------------------------------------------- |
| `hosts`       | `string` | Yes      | Comma-separated list of SSH targets in [user@]host[:port] format |
| `log_paths`   | `string` | No       | Comma-separated specific remote log paths to gather (skips discovery) |
| `incident_id` | `string` | No       | Incident or ticket ID to include in the report header     |

**Registration:**

```go
srv.AddPrompt(&mcp.Prompt{
    Name:        "investigate_remote",
    Description: "Multi-system remote log investigation via SSH",
    Arguments: []*mcp.PromptArgument{
        {Name: "hosts", Description: "Comma-separated SSH targets in [user@]host[:port] format", Required: true},
        {Name: "log_paths", Description: "Specific remote log paths to gather (comma-separated, skips discovery)", Required: false},
        {Name: "incident_id", Description: "Incident or ticket ID for the report header", Required: false},
    },
}, handleInvestigateRemote)
```

**Prompt Text:**

The prompt returns a single `user` role message. The text is constructed from `hosts` and the optional arguments.

When `incident_id` is provided:
> Include the incident ID "{incident_id}" in the report header.

When `log_paths` is provided, the discovery step is replaced:
> **Discovery**: The following specific log paths have been provided: {log_paths}. Skip discovery and proceed to gathering these files.

When `log_paths` is omitted:
> **Discovery**: Use log_discover_remote to find available log files and journal units on all target hosts. Review the results and select the most relevant logs for the investigation.

When there is only one host (no comma in `hosts`), the cross-host steps are omitted:
> **Cross-Host Correlation**: Only one host is being investigated — skip cross-host correlation.
> **Cross-Host Comparison**: Only one host is being investigated — skip cross-host comparison.

The full prompt text (with all optional sections present, multiple hosts, no log_paths):

> Investigate a potential incident across the following remote hosts: {hosts}.
> Include the incident ID "{incident_id}" in the report header.
>
> Follow this structured investigation process:
>
> 1. **Discovery**: Use log_discover_remote to find available log files and journal units on all target hosts. Review the results and select the most relevant logs for the investigation.
>
> 2. **Gathering**: Use log_gather_remote to download the selected log files and journal exports to local temporary files. Note the local paths returned — all subsequent tools operate on these local copies.
>
> 3. **System Health**: Use log_run_remote_command to check each host's current state:
>    - `uptime` — how long has the system been running? Recent reboots?
>    - `df -h` — disk space issues?
>    - `free -h` — memory pressure?
>    - `dmesg | tail -20` — kernel-level issues?
>
> 4. **Log Summary**: Use log_summarize on each gathered log file to establish baseline metrics: line counts, time ranges, error rates, and throughput.
>
> 5. **Error Analysis**: Use log_extract_errors on each gathered log file to identify and cluster error types. Compare error profiles across hosts.
>
> 6. **Anomaly Detection**: Use log_detect_anomalies on each gathered log file to find error spikes, gaps, rate changes, and new error types.
>
> 7. **Cross-Host Correlation**: Use log_correlate across gathered files from different hosts to find events that span multiple systems (shared request IDs, trace IDs, or timestamps).
>
> 8. **Cross-Host Comparison**: Use log_diff to compare log files between hosts to identify divergent behaviour — errors on one host but not another, different error rates, etc.
>
> 9. **Deep Dive**: For the most significant issues found, use log_search with context_lines=5 to examine surrounding context.
>
> 10. **Report**: Compile all findings into a structured Markdown report:
>     - **Incident Report: {incident_id}** (header, or generic if no ID provided)
>     - **Executive Summary** (2-3 sentences: what, when, impact)
>     - **Systems Investigated** (table of hosts with uptime, OS, key metrics)
>     - **Timeline of Events** (chronological, cross-host)
>     - **Error Analysis by Host** (clusters, rates, patterns per host)
>     - **Anomalies Detected** (per host)
>     - **Cross-Host Correlation** (events spanning systems)
>     - **Root Cause Analysis** (assessment based on evidence)
>     - **Impact Assessment** (affected systems, duration, severity)
>     - **Recommendations** (prioritized remediation steps)

**Example Usage Scenarios:**

1. An SRE says "investigate the outage on web1 and web2, incident INC-2025-100" — the AI discovers logs, gathers them, analyzes across both hosts.
2. A developer says "check the logs on db-server, specifically /var/log/postgresql/postgresql-15-main.log" — the AI skips discovery and goes straight to gathering.
3. A team lead says "what's happening on all three app servers?" — the AI discovers, selects, and compares across hosts.