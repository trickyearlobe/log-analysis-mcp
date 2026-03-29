# Future Enhancements

### Live Log Tailing (Watch Mode)

Add a `watch_logs` tool that uses MCP notifications to stream new log entries as they are appended to a file. Would require the Streamable HTTP transport.

**Proposed approach:**
Use `os.File` polling or `fsnotify` to watch for appended lines and emit them as MCP notifications.

### Remote Log Sources

Support reading logs from remote sources:

- **SSH**: Read logs from remote servers via `golang.org/x/crypto/ssh`
- **S3/GCS**: Read log files from cloud object storage
- **CloudWatch**: Query AWS CloudWatch Logs groups and streams
- **Elasticsearch/OpenSearch**: Query log indices

Each remote source would be implemented as a pluggable "reader" behind the same tool interfaces using Go's `io.Reader` interface.

### Log Format Configuration File

Support a `.log-analysis.json` configuration file that defines:

- Custom log formats with named capture groups
- Default paths and glob patterns for log discovery
- Custom level mappings
- Timestamp format specifications

```json
{
  "formats": {
    "custom-app": {
      "pattern": "^\\[(?P<timestamp>[^\\]]+)\\] \\[(?P<level>\\w+)\\] (?P<message>.*)$",
      "timestamp_format": "2006-01-02 15:04:05.000",
      "level_map": { "DBG": "DEBUG", "INF": "INFO", "WRN": "WARN", "ERR": "ERROR" }
    }
  },
  "paths": ["/var/log/myapp/*.log"],
  "default_encoding": "utf-8"
}
```

Note: Go regex uses `(?P<name>...)` for named capture groups instead of `(?<name>...)`. The `timestamp_format` uses Go's reference time layout (`2006-01-02 15:04:05`).

### Custom Parser Plugins

Allow users to register custom parsers as Go plugins or as separate binaries communicating over a simple protocol:

**Go plugin approach:**
Custom parsers implement the same `Parser` interface (Parse, Detect, Name methods) and are loaded from a config-specified directory.

### Compressed File Support

Support reading compressed log files directly:

- `.gz` files via `compress/gzip` (standard library)
- `.bz2` files via `compress/bzip2` (standard library)
- `.zst` files via a streaming Zstandard decoder
- `.zip` archives via `archive/zip` (standard library)

Compressed file support would be transparent — all existing tools would work with compressed files by detecting the extension and wrapping the `io.Reader` with the appropriate decompressor:

Detect compression by file extension (.gz, .bz2, .zst) and wrap the reader with the appropriate decompressor before passing to the streaming line reader.

### Log Management Platform Integration

Integration with popular log management platforms:

- **Grafana Loki**: LogQL queries
- **Datadog**: Log search API
- **Splunk**: SPL queries via REST API
- **Elastic/Kibana**: KQL queries

These would be exposed as additional tools (e.g., `query_loki`, `search_datadog`) with platform-specific input schemas but normalized output formats consistent with the local file tools.

### Log Diff Tool

A `log_diff` tool that compares two log files or two time periods within the same file, highlighting:

- New error types that appear in the second set but not the first
- Changes in error rates or level distribution
- New or missing source components
- Changes in throughput patterns

### Report Generation

A `generate_report` prompt that creates a comprehensive Markdown report combining outputs from multiple tools, suitable for sharing with a team or attaching to an incident ticket.

## System SSH with ControlMaster

The current remote SSH infrastructure uses a two-tier approach:
1. `/usr/bin/ssh -W` for TCP transport (respects `~/.ssh/config`, passes macOS firewall)
2. Go `x/crypto/ssh` for protocol layer (connection pooling, multiplexed sessions)

This works but requires the Go layer to reimplement SSH config resolution for authentication (IdentityAgent, IdentityFile, User, Port). Every new SSH config directive users rely on requires more code in the Go layer.

**Alternative:** Replace both tiers with pure system SSH using `ControlMaster`/`ControlPath`:
- System SSH establishes a multiplexed master connection on first use
- Subsequent `ssh` invocations reuse it via the control socket — no re-auth, no new TCP
- All SSH config directives work automatically (ProxyJump, Match, CanonicalizeHostname, etc.)
- No `x/crypto/ssh` dependency needed for remote operations

**Trade-offs:**
- Subprocess per command (but with ControlMaster, only the first connection has real overhead)
- Less programmatic control over SSH channels
- Requires cleanup of control sockets on shutdown
- Would need to parse command output for exit codes and handle streaming differently

**When to consider:** If more SSH config directives need support, or if the Go SSH auth layer causes further compatibility issues, this approach eliminates the impedance mismatch entirely.