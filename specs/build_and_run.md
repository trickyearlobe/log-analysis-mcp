## Build & Run

### Build

```bash
# Build the binary
go build -o bin/log-analysis-mcp ./cmd/log-analysis-mcp

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Run race detector
go test -race ./...
```

### Makefile

```makefile
BINARY_NAME := log-analysis-mcp
VERSION := 1.0.0
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build test install clean lint docker run

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/log-analysis-mcp

test:
	go test ./...

test-cover:
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-race:
	go test -race ./...

install:
	go install $(LDFLAGS) ./cmd/log-analysis-mcp

clean:
	rm -f $(BINARY_NAME) coverage.out coverage.html

lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed"

docker:
	docker build -t $(BINARY_NAME):$(VERSION) .

run:
	go run ./cmd/log-analysis-mcp
```

### `go.mod`

```
module github.com/trickyearlobe/log-analysis-mcp

go 1.23

require (
    github.com/modelcontextprotocol/go-sdk v1.4.1
)
```

### macOS Firewall (Remote SSH Tools)

The remote SSH tools (`run_remote_command`, `discover_remote_logs`, `gather_remote_logs`)
open TCP connections from the Go binary. macOS Application Firewall blocks outbound
connections from locally-built binaries while allowing signed system binaries like
`/usr/bin/ssh`. The symptom is `connect: no route to host` even though `ssh <host>`
works fine from the terminal.

**This is handled automatically.** On the first connection attempt, the server probes
with `net.Dial`. If that fails on macOS, it falls back to spawning `/usr/bin/ssh -W`
as a TCP proxy — the system SSH binary is pre-authorized by the firewall. The chosen
strategy is cached for the lifetime of the process. See `specs/remote.md` §Connection
Strategy for details.

No firewall configuration or sudo access is required. This works transparently on
corporate MDM-managed Macs.

> **Optional optimisation:** Adding the binary to the firewall allowlist avoids the
> ~20ms probe overhead on the first connection:
> ```bash
> sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add "$(pwd)/bin/log-analysis-mcp"
> sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp "$(pwd)/bin/log-analysis-mcp"
> ```
> This must be re-run after every rebuild. Linux hosts are not affected.

### Run Standalone

```bash
# Start the MCP server with stdio transport
./bin/log-analysis-mcp
```

The server communicates via stdin/stdout using the MCP JSON-RPC protocol. It does not produce any output to stdout on its own — it waits for MCP client messages. Diagnostic logging goes to stderr via `log/slog`.

### Configure in Claude Desktop

Add the following to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or equivalent:

```json
{
  "mcpServers": {
    "log-analysis": {
      "command": "/absolute/path/to/bin/log-analysis-mcp"
    }
  }
}
```

Note: Since the server is a single binary, no `args` are needed — just point `command` directly at the binary. Remote SSH tools work automatically on macOS — the server falls back to the system SSH binary if the firewall blocks direct connections.

### Test with MCP Inspector

```bash
# Interactive testing of all tools, resources, and prompts
npx @modelcontextprotocol/inspector ./bin/log-analysis-mcp
```

The MCP Inspector provides a web UI at `http://localhost:5173` where you can:

- List all registered tools and their schemas
- Call tools with custom arguments and see responses
- Browse resources and read their content
- Execute prompts and see the generated messages

### Dockerfile

```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /log-analysis-mcp ./cmd/log-analysis-mcp

# Runtime stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates
COPY --from=builder /log-analysis-mcp /usr/local/bin/log-analysis-mcp

ENTRYPOINT ["/usr/local/bin/log-analysis-mcp"]
```

### `.gitignore`

```
# Binary
log-analysis-mcp

# Coverage
coverage.out
coverage.html

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
```

---

## Dependencies

### Production Dependencies

| Package                                      | Version   | Purpose                                          |
| -------------------------------------------- | --------- | ------------------------------------------------ |
| `github.com/modelcontextprotocol/go-sdk`     | `v1.4.1`  | MCP server SDK — server, transports, types       |

### Standard Library Packages Used

| Package            | Purpose                                                  |
| ------------------ | -------------------------------------------------------- |
| `os`               | File operations, file info, signals                       |
| `io`               | Reader/Writer interfaces, `io.EOF`, `io.SeekStart/End`   |
| `bufio`            | Streaming line-by-line file reading                       |
| `regexp`           | RE2 regular expressions for log parsing                   |
| `encoding/json`    | JSON marshaling/unmarshaling for tool I/O                |
| `log/slog`         | Structured diagnostic logging to stderr                   |
| `time`             | Timestamp parsing and duration calculations               |
| `path/filepath`    | Cross-platform file path manipulation                     |
| `strings`          | String searching, splitting, trimming                     |
| `strconv`          | String-to-number conversions                              |
| `sort`             | Sorting slices (for top-N, chronological ordering)        |
| `context`          | Context propagation and cancellation                      |
| `fmt`              | String formatting and error wrapping                      |
| `math`             | Mathematical operations for statistics                    |
| `sync`             | Mutexes and WaitGroups for concurrent file processing     |
| `unicode/utf8`     | UTF-8 validation                                          |

### Development Dependencies

None. Go's built-in `go test` framework and standard library `testing` package are used for all tests. No external test frameworks are required.

### Why Minimal Dependencies?

This server intentionally keeps its dependency footprint to a single external module:

- **No log parsing libraries**: Custom parsers give full control over format detection, error handling, and output structure. Third-party parsers often don't expose the raw data or line numbers needed for LLM-friendly output.
- **No file watching libraries**: File watching (future enhancement) can use `os` package or platform-specific APIs.
- **No CLI frameworks**: The server uses stdio transport only — no CLI argument parsing needed.
- **Go standard library**: All file I/O, regex, JSON, time parsing, and HTTP operations use the Go standard library exclusively.
- **Single binary**: Zero runtime dependencies — the binary runs on any compatible OS/arch with no Go runtime, no `node_modules`, no interpreters.