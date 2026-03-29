# Tool: `log_gather_remote`

**Description (shown to LLM):**
> Download log files and export systemd journal units from remote hosts to local
> temporary files. Returns local paths that can be passed directly to other
> analysis tools such as `log_read`, `log_search`, or `log_extract_errors`.

---

## Input Schema

| Parameter         | Type       | Required | Default     | Description                                      |
|-------------------|------------|----------|-------------|--------------------------------------------------|
| `hosts`           | `[]string` | Yes      | —           | SSH targets in `[user@]host[:port]` format       |
| `paths`           | `[]string` | No       | —           | Remote file paths to gather                      |
| `journal_units`   | `[]string` | No       | —           | Systemd journal units to export                  |
| `journal_since`   | `string`   | No       | —           | ISO 8601 start time for journal export           |
| `journal_until`   | `string`   | No       | —           | ISO 8601 end time for journal export             |
| `max_file_bytes`  | `int`      | No       | `104857600` | Max bytes per file (100 MB)                      |
| `timeout_seconds` | `int`      | No       | `300`       | Max seconds per file transfer (5 min)            |

### Go Input Struct

```go
type GatherRemoteLogsInput struct {
	Hosts          []string `json:"hosts"                      jsonschema:"required,description=SSH targets in [user@]host[:port] format"`
	Paths          []string `json:"paths,omitempty"            jsonschema:"description=Remote file paths to gather"`
	JournalUnits   []string `json:"journal_units,omitempty"    jsonschema:"description=Systemd journal units to export"`
	JournalSince   string   `json:"journal_since,omitempty"    jsonschema:"description=ISO 8601 start time for journal export"`
	JournalUntil   string   `json:"journal_until,omitempty"    jsonschema:"description=ISO 8601 end time for journal export"`
	MaxFileBytes   int      `json:"max_file_bytes,omitempty"   jsonschema:"description=Max bytes per file (default 100 MB),minimum=1"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"  jsonschema:"description=Max seconds per file transfer (default 300),minimum=1"`
}
```

### Default Values (applied in handler)

| Field            | Default     |
|------------------|-------------|
| `MaxFileBytes`   | `104857600` |
| `TimeoutSeconds` | `300`       |

---

## Output Format

### Go Output Structs

```go
type GatheredFile struct {
	Host       string `json:"host"`
	RemotePath string `json:"remote_path"`
	LocalPath  string `json:"local_path"`
	SizeBytes  int64  `json:"size_bytes"`
	Type       string `json:"type"`            // "file" or "journal"
	Error      string `json:"error,omitempty"`
}

type GatherRemoteLogsOutput struct {
	Files   []GatheredFile `json:"files"`
	TempDir string         `json:"temp_dir"`
}
```

### JSON Example

```json
{
  "files": [
    {
      "host": "web1.example.com",
      "remote_path": "/var/log/nginx/error.log",
      "local_path": "/tmp/log-analysis-gather-a1b2c3/web1.example.com/var-log-nginx-error.log",
      "size_bytes": 2481920,
      "type": "file"
    },
    {
      "host": "web1.example.com",
      "remote_path": "journalctl -u nginx",
      "local_path": "/tmp/log-analysis-gather-a1b2c3/web1.example.com/journal-nginx.log",
      "size_bytes": 1048576,
      "type": "journal"
    },
    {
      "host": "web2.example.com",
      "remote_path": "/var/log/nginx/error.log",
      "local_path": "",
      "size_bytes": 0,
      "type": "file",
      "error": "scp: /var/log/nginx/error.log: No such file or directory"
    }
  ],
  "temp_dir": "/tmp/log-analysis-gather-a1b2c3"
}
```

---

## Local File Organisation

Files are stored under a deterministic directory structure:

```
<temp_dir>/
  <hostname>/
    var-log-nginx-error.log      ← flattened path (/ replaced with -)
    var-log-app.log              ← another file from the same host
    journal-nginx.log            ← journal export for unit "nginx"
    journal-myapp.log            ← journal export for unit "myapp"
  <hostname2>/
    ...
```

- `<temp_dir>` is created with `os.MkdirTemp("", "log-analysis-gather-*")`.
- Remote paths are flattened by replacing `/` with `-` and stripping the
  leading `-` (e.g. `/var/log/app.log` → `var-log-app.log`).
- Journal exports use the pattern `journal-<unit>.log`.

---

## Handler Signature

```go
func handleGatherRemoteLogs(ctx context.Context, req *mcp.CallToolRequest, input GatherRemoteLogsInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
	Name:        "log_gather_remote",
	Description: "Download log files and export systemd journal units from remote hosts to local temporary files. Returns local paths that can be passed directly to other analysis tools.",
}, handleGatherRemoteLogs)
```

---

## Temp File Lifecycle

Temporary files created by this tool are registered in the same process-level
cleanup registry used by `log_decompress`. The `TempDir` and all its contents
are removed by `CleanupTempFiles()` during server shutdown.

If the server crashes, temp files remain in `os.TempDir()`. The
`log-analysis-gather-` prefix makes them identifiable for manual cleanup.

---

## Error Handling

| Condition                              | Error Code      | Message                                                                |
|----------------------------------------|-----------------|------------------------------------------------------------------------|
| Empty `hosts` slice                    | `INVALID_INPUT` | `hosts is required and must not be empty`                              |
| No `paths` and no `journal_units`      | `INVALID_INPUT` | `At least one of paths or journal_units must be provided`              |
| SSH connection failure                 | *(per-file)*    | `SSH connection failed for <host>: <underlying error>`                 |
| File not found on remote               | *(per-file)*    | `Remote file not found: <path> on <host>`                              |
| Max file size exceeded                 | *(per-file)*    | `File exceeds max_file_bytes (<limit>): <path> on <host>`             |
| Transfer timeout                       | *(per-file)*    | `Transfer timed out after <N>s: <path> on <host>`                     |
| Journal unit not found                 | *(per-file)*    | `Journal unit not found on <host>: <unit>`                             |

Per-file errors appear in the `Error` field of the corresponding `GatheredFile`
entry. A failure for one file or host does not abort transfers for others.

Tool-level errors (`INVALID_INPUT`) are returned only for validation failures
that prevent any work from starting.

---

## Output Invariants

- `Files` has one entry per host × path combination plus one entry per
  host × journal unit combination.
- `Files` is non-nil (empty slice `[]` when nothing was gathered).
- `TempDir` is the root temporary directory (absolute path).
- All `LocalPath` values are absolute paths under `TempDir`.
- `LocalPath` is empty string when `Error` is set.
- `SizeBytes` is `0` when `Error` is set.

---

## Usage Scenario

An AI assistant is asked to investigate errors across a web cluster. It first
calls `log_discover_remote` to find available log files, then calls
`log_gather_remote` to download the relevant files locally. The returned
`local_path` values are passed to `log_extract_errors`, `log_search`, and
`log_summarize` for analysis — all running against fast local files with no
repeated SSH overhead.