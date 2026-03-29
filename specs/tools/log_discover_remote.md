# Tool: `log_discover_remote`

**Description (shown to LLM):**
> Discover log files and systemd journal units on remote hosts via SSH. Scans
> standard log locations by default, with optional custom search paths and
> commands. Use this to inventory what logs exist before gathering or analysing
> them.

---

## Input Schema

| Parameter          | Type       | Required | Default | Description                                                          |
| ------------------ | ---------- | -------- | ------- | -------------------------------------------------------------------- |
| `hosts`            | `[]string` | Yes      | —       | SSH targets in `[user@]host[:port]` format                           |
| `additional_paths` | `[]string` | No       | —       | Extra directories to scan (appended to default `/var/log` scan)      |
| `custom_command`   | `string`   | No       | —       | Custom shell command; output parsed as one path per line             |
| `timeout_seconds`  | `int`      | No       | `30`    | Max seconds per host                                                 |

### Go Input Struct

```go
type DiscoverRemoteLogsInput struct {
    Hosts           []string `json:"hosts"                      jsonschema:"required,description=SSH targets in [user@]host[:port] format"`
    AdditionalPaths []string `json:"additional_paths,omitempty"  jsonschema:"description=Extra directories to scan for log files"`
    CustomCommand   string   `json:"custom_command,omitempty"    jsonschema:"description=Custom shell command for log discovery (output: one path per line)"`
    TimeoutSeconds  int      `json:"timeout_seconds,omitempty"   jsonschema:"description=Max seconds per host (default 30),minimum=1"`
}
```

### Default Values (applied in handler)

| Field            | Default |
| ---------------- | ------- |
| `TimeoutSeconds` | `30`    |

---

## Output Format

### Go Output Structs

```go
type DiscoveredLog struct {
    Path         string   `json:"path"`
    Type         string   `json:"type"`                    // "file" or "journal"
    SizeBytes    int64    `json:"size_bytes,omitempty"`
    SizeHuman    string   `json:"size_human,omitempty"`
    ModifiedTime string   `json:"modified_time,omitempty"` // RFC 3339
    Variants     []string `json:"variants,omitempty"`      // rotated files: .log.1, .log.2.gz
}

type HostDiscoveryResult struct {
    Host  string          `json:"host"`
    Logs  []DiscoveredLog `json:"logs"`
    Error string          `json:"error,omitempty"`
}

type DiscoverRemoteLogsOutput struct {
    Results []HostDiscoveryResult `json:"results"`
}
```

### JSON Example

```json
{
  "results": [
    {
      "host": "web-01",
      "logs": [
        {
          "path": "/var/log/syslog",
          "type": "file",
          "size_bytes": 4521984,
          "size_human": "4.3 MB",
          "modified_time": "2025-01-15T10:45:23Z",
          "variants": ["/var/log/syslog.1", "/var/log/syslog.2.gz", "/var/log/syslog.3.gz"]
        },
        {
          "path": "/var/log/nginx/access.log",
          "type": "file",
          "size_bytes": 1048576,
          "size_human": "1.0 MB",
          "modified_time": "2025-01-15T11:00:00Z",
          "variants": ["/var/log/nginx/access.log.1"]
        },
        {
          "path": "nginx.service",
          "type": "journal",
          "size_bytes": 0,
          "size_human": ""
        }
      ],
      "error": ""
    },
    {
      "host": "db-01",
      "logs": [],
      "error": "ssh: connect to host db-01 port 22: Connection refused"
    }
  ]
}
```

---

## Default Scan Behaviour

1. **File scan:** Find files in `/var/log` (and each entry in `additional_paths`)
   matching `*.log`, `*.log.*`, `syslog*`, `messages*`, up to depth 3.
2. **Journal detection:** Check if `journalctl` is available on the host. If
   present, list running systemd service units and include each as a
   `"journal"` entry.
3. **Variant grouping:** Rotated files (e.g. `.log.1`, `.log.2.gz`) are grouped
   as `variants` of the base log file. Variants are sorted naturally
   (1, 2, 3… not 1, 10, 2).
4. **Custom command merge:** If `custom_command` is provided, execute it on each
   host, parse stdout as one path per line, and merge results into the `Logs`
   slice. Duplicates (same path) are deduplicated.

---

## Handler Signature

```go
func handleDiscoverRemoteLogs(ctx context.Context, req *mcp.CallToolRequest, input DiscoverRemoteLogsInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "log_discover_remote",
    Description: "Discover log files and systemd journal units on remote hosts via SSH. Scans standard log locations by default, with optional custom search paths and commands.",
}, handleDiscoverRemoteLogs)
```

---

## Error Handling

| Condition                        | Error Code       | Message                                                                 |
| -------------------------------- | ---------------- | ----------------------------------------------------------------------- |
| Empty `hosts` slice              | `INVALID_INPUT`  | `hosts is required and must contain at least one SSH target`            |
| Host connection failure          | *(per-host)*     | Error string in `HostDiscoveryResult.Error`; does not abort other hosts |
| Host command timeout             | *(per-host)*     | `"timeout: command exceeded <N> seconds on <host>"`                     |
| Custom command exits non-zero    | *(per-host)*     | `"custom_command failed (exit <N>): <stderr snippet>"`                  |
| Invalid host format              | `INVALID_INPUT`  | `"invalid host format: <value>"`                                        |

Per-host errors appear in the `Error` field of the corresponding
`HostDiscoveryResult`. They do not cause a tool-level error and do not prevent
other hosts from being scanned.

---

## Output Invariants

- `Results` slice has exactly one entry per host in `hosts`, in the same order.
- `Results` is never nil.
- `Logs` is a non-nil empty slice `[]` when no logs are found on a host.
- `Variants` is sorted naturally (1, 2, 3…) when present; nil when there are
  no rotated variants.
- `Type` is always `"file"` or `"journal"`.
- `ModifiedTime` is RFC 3339 when available, empty string when unavailable.
- `SizeBytes` is `0` for journal entries (size not knowable without export).

---

## Usage Scenario

An AI assistant is asked to investigate issues on a fleet of servers. It first
calls `log_discover_remote` to inventory what log files and journal units
exist on each host. Using the results, it decides which logs are relevant (e.g.
nginx access logs, application logs) and passes those paths to
`log_gather_remote` to download them for local analysis.