# log-analysis-mcp

An MCP server that gives AI assistants powerful tools to parse, search, analyse, and summarise log files. Single binary, zero runtime dependencies.

## Features

- **15 tools** covering the full log investigation lifecycle
- **Transparent compressed file handling** — all tools work directly with `.gz`, `.bz2`, and `.zip` files
- **Remote log collection** via SSH with automatic macOS firewall handling
- **Auto-install** into Claude Desktop, VS Code, Cursor, Windsurf, Zed, and Copilot CLI
- **Structured JSON output** designed for LLM consumption

## Tools

| Tool | Description |
|------|-------------|
| `log_read` | Read a log file with line-range pagination |
| `log_tail` | Read the last N lines (most recent entries) |
| `log_search` | Regex/text search with optional context lines |
| `log_parse` | Auto-detect format and extract structured fields |
| `log_filter` | Filter by level, time range, source, or message pattern |
| `log_extract_errors` | Cluster errors by similarity to identify distinct error types |
| `log_summarize` | Statistical summary: level distribution, top errors, throughput |
| `log_detect_anomalies` | Find error spikes, rate changes, gaps, and new error types |
| `log_timeline` | Build a chronological timeline of significant events |
| `log_correlate` | Correlate events across files by request/trace ID |
| `log_diff` | Compare two log files or time periods for changes |
| `log_decompress` | Decompress a file to disk for repeated access |
| `log_run_remote_command` | Execute a command on remote hosts via SSH |
| `log_discover_remote` | Discover log files and journal units on remote hosts |
| `log_gather_remote` | Download remote logs to local temp files for analysis |

## Prompts

| Prompt | Description |
|--------|-------------|
| `investigate_error` | Guided error investigation workflow |
| `log_health_check` | System health assessment from logs |
| `generate_report` | Comprehensive Markdown report from multiple tools |
| `investigate_remote` | Multi-host remote log investigation workflow |

## Installation

### From Release Binaries

Download the latest binary for your platform from the [Releases](https://github.com/trickyearlobe/log-analysis-mcp/releases) page and run:

```sh
chmod +x log-analysis-mcp-*    # not needed on Windows
./log-analysis-mcp --install
```

This registers the server in all detected IDEs automatically.

### From Source

```sh
git clone https://github.com/trickyearlobe/log-analysis-mcp.git
cd log-analysis-mcp
make build
./bin/log-analysis-mcp --install
```

### Manual Configuration

If you prefer to configure manually, add the server to your IDE's MCP config:

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "log-analysis-mcp": {
      "command": "/absolute/path/to/log-analysis-mcp"
    }
  }
}
```

**VS Code** (`~/.vscode/mcp.json`):

```json
{
  "servers": {
    "log-analysis-mcp": {
      "command": "/absolute/path/to/log-analysis-mcp"
    }
  }
}
```

**Copilot CLI** (`~/.copilot/mcp-config.json`):

```json
{
  "mcpServers": {
    "log-analysis-mcp": {
      "type": "local",
      "command": "/absolute/path/to/log-analysis-mcp",
      "args": [],
      "env": {},
      "tools": ["*"]
    }
  }
}
```

### Uninstall

```sh
./log-analysis-mcp --uninstall
```

## Building

```sh
make           # show available targets
make build     # build the binary
make test      # run tests
make test-race # run tests with race detector
make lint      # go vet + staticcheck
```

## Releasing

Version is derived from git tags. Create a tag and push to trigger the release pipeline:

```sh
make release-patch   # v1.0.0 -> v1.0.1
make release-minor   # v1.0.0 -> v1.1.0
make release-major   # v1.0.0 -> v2.0.0
git push origin <tag>
```

The GitHub Actions pipeline builds binaries for linux, macOS, and Windows on both amd64 and arm64, then creates a GitHub Release with checksums.

## Requirements

- Go 1.23+ (build only)
- No runtime dependencies — single static binary

## License

Apache 2.0