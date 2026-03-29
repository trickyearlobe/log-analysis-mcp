# Tool: `log_run_remote_command`

**Description (shown to LLM):**
> Execute a command on one or more remote hosts via SSH. Returns stdout, stderr, and exit code per host. Useful for custom log discovery, quick system checks, and flexible remote operations.

---

## Input Schema

| Parameter          | Type       | Required | Default   | Description                                      |
| ------------------ | ---------- | -------- | --------- | ------------------------------------------------ |
| `hosts`            | `[]string` | Yes      | —         | SSH targets in `[user@]host[:port]` format        |
| `command`          | `string`   | Yes      | —         | Shell command to execute on each host             |
| `timeout_seconds`  | `int`      | No       | `30`      | Max seconds per host                              |
| `max_output_bytes` | `int`      | No       | `1048576` | Max bytes of stdout/stderr per host (1 MB)        |

### Go Input Struct

```go
type RunRemoteCommandInput struct {
    Hosts          []string `json:"hosts"                      jsonschema:"required,description=SSH targets in [user@]host[:port] format"`
    Command        string   `json:"command"                    jsonschema:"required,description=Shell command to execute on each host"`
    TimeoutSeconds int      `json:"timeout_seconds,omitempty"  jsonschema:"description=Max seconds per host (default 30),minimum=1"`
    MaxOutputBytes int      `json:"max_output_bytes,omitempty" jsonschema:"description=Max bytes of stdout/stderr per host (default 1MB),minimum=1"`
}
```

### Default Values (applied in handler)

| Field            | Default   |
| ---------------- | --------- |
| `TimeoutSeconds` | `30`      |
| `MaxOutputBytes` | `1048576` |

---

## Output Format

### Go Output Structs

```go
type HostCommandResult struct {
    Host     string `json:"host"`
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    ExitCode int    `json:"exit_code"`
    Error    string `json:"error,omitempty"`
}

type RunRemoteCommandOutput struct {
    Results []HostCommandResult `json:"results"`
}
```

### JSON Example

```json
{
  "results": [
    {
      "host": "web01.example.com",
      "stdout": "Linux web01 5.15.0-91-generic #101-Ubuntu SMP x86_64\n",
      "stderr": "",
      "exit_code": 0
    },
    {
      "host": "db01.example.com",
      "stdout": "",
      "stderr": "",
      "exit_code": 0,
      "error": "ssh: connect to host db01.example.com port 22: Connection refused"
    }
  ]
}
```

---

## Handler Signature

```go
func handleRunRemoteCommand(ctx context.Context, req *mcp.CallToolRequest, input RunRemoteCommandInput) (*mcp.CallToolResult, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "log_run_remote_command",
    Description: "Execute a command on one or more remote hosts via SSH. Returns stdout, stderr, and exit code per host. Useful for custom log discovery, quick system checks, and flexible remote operations.",
}, handleRunRemoteCommand)
```

---

## Output Invariants

- `Results` slice is non-nil and has exactly one entry per element in `hosts`.
- Order of `Results` matches order of `hosts`.
- Per-host failures (connection refused, timeout, auth failure) populate the `Error` field on that host's result; they do **not** abort execution on other hosts and do **not** produce a tool-level error.
- `Stdout` and `Stderr` are truncated to `max_output_bytes` independently. When truncated, the value ends with `\n...[truncated at <N> bytes]`.

---

## Error Handling

| Condition                    | Error Code      | Message                                                  |
| ---------------------------- | --------------- | -------------------------------------------------------- |
| `hosts` is empty             | `INVALID_INPUT` | `hosts is required and must not be empty`                |
| `command` is empty           | `INVALID_INPUT` | `command is required and must not be empty`              |
| Connection failure           | —               | Per-host `error` field (see Output Invariants)           |
| Timeout exceeded             | —               | Per-host `error` field: `command timed out after Ns`     |
| Output exceeds max bytes     | —               | Output truncated with marker (see Output Invariants)     |

---

## Usage Scenario

An AI assistant is asked to check disk space across a fleet of servers. It calls `log_run_remote_command` with `hosts: ["web01", "web02", "db01"]` and `command: "df -h /var/log"` to get a quick snapshot without needing a specialised tool.