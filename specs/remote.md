# Remote SSH Infrastructure

Shared SSH client infrastructure used by `run_remote_command`, `discover_remote_logs`,
and `gather_remote_logs` tools.

## External Dependencies

**Import path:** `golang.org/x/crypto`

**Sub-packages used:**
- `golang.org/x/crypto/ssh` — SSH client, session exec, auth methods
- `golang.org/x/crypto/ssh/agent` — SSH agent forwarding for key-based auth
- `golang.org/x/crypto/ssh/knownhosts` — Host key verification against `~/.ssh/known_hosts`

**Justification:** Go stdlib has no SSH client. `x/crypto` is the official Go
sub-repository maintained by the Go team. There is no lighter alternative.

**Vetting notes:**
- Maintainer: Go team (`golang.org/x`)
- License: BSD-3-Clause
- Importers: 22,000+
- Transitive deps: only `golang.org/x/sys` (also official Go sub-repo)
- Vulnerability history: actively patched, covered by Go security policy
- Run `govulncheck ./...` after adding the dependency to confirm no known issues

This exception is scoped to the remote log tools. It does not change the
general "stdlib only" rule in CLAUDE.md.

---

## Package: `internal/remote`

### Target Parsing

Hosts are specified as strings in the format `[user@]host[:port]`.

```go
// Target represents a parsed SSH target.
type Target struct {
    User string
    Host string
    Port int
}
```

**Parsing rules:**
- `user@host:port` → all three fields set
- `user@host` → port defaults to 22
- `host:port` → user defaults to current OS user (`os.Getenv("USER")` or `user.Current()`)
- `host` → user defaults to OS user, port defaults to 22
- If user is empty after all fallbacks, return error

```go
// ParseTarget parses an SSH target string into its components.
func ParseTarget(s string) (Target, error)
```

**Validation:**
- Host must not be empty
- Port must be 1–65535
- User must not be empty after defaults applied

### Authentication

Auth methods are tried in order. The first successful method wins.

**Auth chain:**

1. **SSH agent** — connect to `SSH_AUTH_SOCK` via `agent.NewClient(net.Dial("unix", sock))`.
   Skip silently if `SSH_AUTH_SOCK` is not set or the socket is unreachable.

2. **Key files** — try each in order, skip files that don't exist:
   - `~/.ssh/id_ed25519`
   - `~/.ssh/id_ecdsa`
   - `~/.ssh/id_rsa`
   Parse with `ssh.ParsePrivateKey`. If the key is encrypted
   (`PassphraseMissingError`), skip it with a log warning — we cannot prompt
   for passphrases in MCP (stdout is owned by the protocol).

3. **Error** — if no auth method succeeded, return a clear error listing what
   was tried and why each failed.

```go
// BuildAuthMethods returns SSH auth methods in priority order.
// It logs (to slog) which methods are available and which are skipped.
func BuildAuthMethods() ([]ssh.AuthMethod, error)
```

**No password auth.** Passwords cannot be prompted for over MCP stdio.
If password auth is needed in the future, it could use MCP elicitation
(once supported) or environment variables, but that is out of scope.

### Host Key Verification

```go
// BuildHostKeyCallback returns a host key callback using ~/.ssh/known_hosts.
func BuildHostKeyCallback() (ssh.HostKeyCallback, error)
```

**Behaviour:**
- Parse `~/.ssh/known_hosts` using `knownhosts.New()`.
- If the file doesn't exist, return an error with a clear message:
  `"no known_hosts file found at ~/.ssh/known_hosts — run 'ssh-keyscan <host> >> ~/.ssh/known_hosts' to add the host key"`
- If the host key doesn't match, the SSH library returns a `knownhosts.KeyError`
  which produces a clear error message about the mismatch.
- No `InsecureIgnoreHostKey` option. Security is not optional.

### Connection Pooling

```go
// ClientPool manages reusable SSH client connections.
type ClientPool struct {
    mu      sync.Mutex
    clients map[string]*ssh.Client // key: "user@host:port"
    timeout time.Duration          // dial timeout, default 30s
}

// NewClientPool creates a pool with the given dial timeout.
func NewClientPool(timeout time.Duration) *ClientPool

// Get returns an existing client or dials a new connection.
// The pool key is the canonical "user@host:port" string.
func (p *ClientPool) Get(target Target) (*ssh.Client, error)

// CloseAll closes all pooled connections. Called on server shutdown.
func (p *ClientPool) CloseAll()
```

**Pool behaviour:**
- `Get` checks the pool first. If the cached client's underlying connection
  is dead (detected by sending a keepalive request), it is removed and a new
  connection is dialed.
- Connections are **not** removed after use — they stay in the pool until
  `CloseAll` or until detected dead.
- `CloseAll` is called from `main.go` in a `defer`, same pattern as
  `tools.CleanupTempFiles()`.

### Command Execution

```go
// ExecResult holds the output of a remote command.
type ExecResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
}

// Exec runs a command on the remote host with the given timeout.
// It returns the captured stdout, stderr, and exit code.
// If the command times out, the session is closed and an error is returned.
func Exec(client *ssh.Client, cmd string, timeout time.Duration) (ExecResult, error)
```

**Implementation:**
- Create a new `Session` per `Exec` call (sessions are one-shot in SSH).
- Set `session.Stdout` and `session.Stderr` to `bytes.Buffer`.
- Use `session.Start(cmd)` + goroutine with `session.Wait()`.
- Use `context.WithTimeout` or `time.AfterFunc` to enforce the timeout.
  On timeout, call `session.Close()` to kill the remote process.
- Extract exit code from `*ssh.ExitError` if present. If `ExitMissingError`,
  set exit code to -1.
- If stdout or stderr exceeds a caller-specified max size, truncate and
  append `\n[truncated: output exceeded <N> bytes]`.

### File Download

```go
// DownloadFile copies a remote file to a local path using cat over SSH exec.
// It enforces maxBytes — if the remote file exceeds this, the download is
// aborted and an error is returned. Returns the number of bytes written.
func DownloadFile(client *ssh.Client, remotePath, localPath string, maxBytes int64, timeout time.Duration) (int64, error)
```

**Implementation:**
- `session.Start("cat " + shellescape(remotePath))`
- Pipe `session.StdoutPipe()` through an `io.LimitedReader` to enforce max size.
- Write to local file with `bufio.Writer` for efficiency.
- If the limit is hit, close the session (kills `cat`), remove partial local file,
  return error.
- `shellescape` is a simple function that wraps the path in single quotes with
  proper escaping for single quotes inside the path. No external dependency needed.

### Journal Export

```go
// ExportJournal runs journalctl on the remote host and writes output to a local file.
// unit is the systemd service name (e.g., "nginx.service").
// since/until are optional ISO 8601 timestamps for time range filtering.
func ExportJournal(client *ssh.Client, unit, since, until, localPath string, maxBytes int64, timeout time.Duration) (int64, error)
```

**Implementation:**
- Build command: `journalctl -u <unit> --no-pager -o short-iso [--since "..."] [--until "..."]`
- Same streaming-to-file approach as `DownloadFile`.
- If `journalctl` is not found (exit code 127 or command not found in stderr),
  return a clear error: `"journalctl not available on <host>"`

---

## Shell Escaping

```go
// ShellEscape wraps a string in single quotes for safe use in sh commands.
// Single quotes within the string are escaped as '\'' (end quote, escaped quote, start quote).
func ShellEscape(s string) string
```

This is the only escaping needed. All remote commands use `sh -c` implicitly
via the SSH exec channel, so single-quote wrapping is sufficient.

---

## Timeout Policy

| Operation | Default | Configurable via |
|-----------|---------|------------------|
| SSH dial | 30s | `ClientPool` constructor |
| `run_remote_command` | 30s | `timeout_seconds` input field |
| `discover_remote_logs` per host | 30s | `timeout_seconds` input field |
| `gather_remote_logs` per file | 300s (5min) | `timeout_seconds` input field |
| Journal export | 300s (5min) | same as gather |

---

## Max Output / File Size Policy

| Operation | Default | Configurable via |
|-----------|---------|------------------|
| `run_remote_command` stdout | 1 MB | `max_output_bytes` input field |
| `gather_remote_logs` per file | 100 MB | `max_file_bytes` input field |
| `discover_remote_logs` output | 1 MB | not configurable (discovery output is metadata, always small) |

---

## Error Handling

All errors are returned, never panicked. Errors are wrapped with context:

- `"remote: dial user@host:port: <reason>"` — connection failures
- `"remote: auth: no methods available — SSH_AUTH_SOCK not set, no key files found in ~/.ssh/"` — auth failures
- `"remote: known_hosts: <reason>"` — host key issues
- `"remote: exec on user@host:port: <reason>"` — command execution failures
- `"remote: download <path> from user@host:port: <reason>"` — file transfer failures
- `"remote: journal <unit> from user@host:port: <reason>"` — journal export failures

Per-host errors in multi-host operations do NOT abort other hosts. Each host's
result includes its own error field. The tool returns partial results.

---

## Lifecycle

```
main.go:
    pool := remote.NewClientPool(30 * time.Second)
    defer pool.CloseAll()
    // pass pool to tools via registration or package-level var
```

The `ClientPool` is created once at startup and shared across all remote tools.
It is closed on shutdown alongside temp file cleanup.

**Package-level pool:** For simplicity, `internal/remote` exposes a default pool
initialised on first use (via `sync.Once`). Tools call `remote.DefaultPool()`.
`remote.CloseDefaultPool()` is called from `main.go`.

---

## Testing Strategy

### Unit tests with mock SSH server

The `x/crypto/ssh` package supports creating in-process SSH servers via
`ssh.NewServerConn`. Tests create a local TCP listener, accept connections with
a test server config, and handle exec requests by running simple test commands.

```go
// testSSHServer starts a local SSH server for testing.
// Returns the address (host:port) and a cleanup function.
func testSSHServer(t *testing.T, handler func(ch ssh.Channel, req *ssh.Request)) (string, func())
```

This avoids requiring a real SSH server for CI.

### Integration tests with real SSH

Guarded by `SSH_TEST_HOST` environment variable. When set, integration tests
connect to a real host and run discovery/gather operations. When not set,
these tests are skipped with `t.Skip("set SSH_TEST_HOST to enable")`.

### What to test

| Component | Test approach |
|-----------|---------------|
| `ParseTarget` | Pure unit tests, no SSH needed |
| `BuildAuthMethods` | Unit test with mocked env vars and temp key files |
| `BuildHostKeyCallback` | Unit test with temp known_hosts file |
| `ClientPool` | Mock SSH server |
| `Exec` | Mock SSH server that echoes, sleeps (timeout), or exits non-zero |
| `DownloadFile` | Mock SSH server that serves file content via cat |
| `ExportJournal` | Mock SSH server that returns fake journal output |
| `ShellEscape` | Pure unit tests |

---

## Invariants

- NEVER write to stdout. All logging via `slog` to stderr.
- NEVER store credentials in memory longer than needed. Auth methods are built
  once per connection attempt.
- NEVER modify remote hosts. All operations are read-only.
- NEVER use `InsecureIgnoreHostKey`. Always verify against known_hosts.
- Connection pool is goroutine-safe (`sync.Mutex`).
- All temp files from gather operations are registered for cleanup.
- Partial results are always returned for multi-host operations.