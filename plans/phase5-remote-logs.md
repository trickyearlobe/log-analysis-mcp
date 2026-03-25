# Phase 5: Remote Log Tools

## Goal

Add SSH-based remote log discovery, gathering, and command execution.
Three new tools: `discover_remote_logs`, `gather_remote_logs`, `run_remote_command`.
Shared SSH infrastructure in `internal/remote/`.

## External Dependency

`golang.org/x/crypto` — provides `ssh`, `ssh/agent`, `ssh/knownhosts`.
Official Go sub-repository, BSD-3-Clause, 22k+ importers.
Exception documented in `specs/remote.md` per policy in `plans/phase2-roadmap.md`.

## Specs to Read

- `specs/remote.md` (to be written first — SSH infra, auth, pooling, host key verification)
- `specs/tools/run_remote_command.md` (to be written)
- `specs/tools/discover_remote_logs.md` (to be written)
- `specs/tools/gather_remote_logs.md` (to be written)
- `specs/compression.md` — gathered files may be compressed
- `specs/fileutil.md` — temp file patterns from `decompress_file`
- `specs/performance.md` — streaming constraints, max output sizes

## Phase 5a: SSH Infrastructure

### Steps

1. Write `specs/remote.md` — shared SSH contracts, auth chain, connection pooling, host key verification, timeout policy, dependency vetting.
2. `go get golang.org/x/crypto` and run `govulncheck ./...`.
3. Write `internal/remote/ssh_test.go` — unit tests for:
   - Host string parsing (`user@host`, `user@host:port`, defaults)
   - Auth method resolution (agent → key files → error)
   - Known hosts verification (accept known, reject unknown, clear error message)
   - Connection reuse (same host returns same client)
   - Session exec + stdout/stderr capture
   - Timeout enforcement
4. Write `internal/remote/ssh.go` — implement to pass tests:
   - `ParseTarget(s string) (user, host string, port int, err error)`
   - `type ClientPool` — `sync.Mutex`-protected `map[string]*ssh.Client`
   - `func (p *ClientPool) Get(target string) (*ssh.Client, error)`
   - `func (p *ClientPool) CloseAll()`
   - Auth chain: `ssh/agent` socket → `~/.ssh/id_ed25519` → `~/.ssh/id_rsa` → error
   - Host key callback via `knownhosts.New("~/.ssh/known_hosts")`
   - `func Exec(client *ssh.Client, cmd string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)`
5. Wire `ClientPool.CloseAll()` into `main.go` shutdown alongside `CleanupTempFiles()`.
6. `go test -race ./internal/remote/` && `go vet ./...`.
7. Commit: `remote: add SSH client infrastructure`

### Testing Note

Unit tests for SSH require either:
- A real SSH server (skip with `testing.Short()` or env var guard)
- A mock using `ssh.NewServerConn` from the same package (preferred for CI)

Design the test helpers so both modes work.

## Phase 5b: `run_remote_command`

### Steps

1. Write `specs/tools/run_remote_command.md`.
2. Write `internal/tools/run_remote_command_test.go` — tests covering:
   - Successful command execution with stdout capture
   - Command with stderr output
   - Non-zero exit code returned in output (not as error)
   - Timeout enforcement
   - Max output size truncation (default 1MB)
   - Error: empty host
   - Error: empty command
   - Error: connection failure (bad host)
   - Multiple hosts: command run on each, results per host
3. Write `internal/tools/run_remote_command.go` — implement `RunRemoteCommand`:
   - Input: `hosts []string`, `command string`, `timeout_seconds int`, `max_output_bytes int`
   - Output: per-host results with `stdout`, `stderr`, `exit_code`, `error`
   - Uses `remote.ClientPool` and `remote.Exec`
4. Register in `register.go` (tool #13).
5. `go test -race ./...` && `go vet ./...`.
6. Commit: `tools: add run_remote_command tool`

### Acceptance Criteria

- `run_remote_command` with `["localhost"]` and `"echo hello"` returns `{"stdout": "hello\n"}`.
- Timeout kills long-running commands.
- Output > max_output_bytes is truncated with indicator.

## Phase 5c: `discover_remote_logs`

### Steps

1. Write `specs/tools/discover_remote_logs.md`.
2. Write `internal/tools/discover_remote_logs_test.go` — tests covering:
   - Default discovery finds files in `/var/log`
   - Journald units detected when `journalctl` is available
   - Rotated files grouped with parent (`.log.1`, `.log.2.gz`)
   - Custom search paths appended to default scan
   - Custom command executed and results merged
   - File metadata: size, modified time, type (file/journal)
   - Multiple hosts: results structured per host
   - Error: connection failure for one host doesn't block others
   - Error: empty hosts list
3. Write `internal/tools/discover_remote_logs.go` — implement `RunDiscoverRemoteLogs`:
   - Input: `hosts []string`, `additional_paths []string`, `custom_command string`
   - Default scan commands:
     - `find /var/log -maxdepth 3 \( -name '*.log' -o -name '*.log.*' -o -name 'syslog*' -o -name 'messages*' \) -printf '%p\t%s\t%T@\n' 2>/dev/null`
     - `journalctl --list-boots --no-pager 2>/dev/null` (detect journald availability)
     - `systemctl list-units --type=service --state=running --no-pager --no-legend 2>/dev/null` (list active services with journals)
   - Additional paths: appended to the `find` command
   - Custom command: executed raw, output parsed as one-path-per-line
   - Output: per-host list of `DiscoveredLog` entries
4. Register in `register.go` (tool #14).
5. `go test -race ./...` && `go vet ./...`.
6. Commit: `tools: add discover_remote_logs tool`

### Acceptance Criteria

- On a Linux host with `/var/log`, discovers syslog/auth/kern logs.
- On a systemd host, detects available journal units.
- Custom command (`locate '*.log'`) results are included.
- Rotated files grouped: `app.log` shows variants `[app.log.1, app.log.2.gz]`.

## Phase 5d: `gather_remote_logs`

### Steps

1. Write `specs/tools/gather_remote_logs.md`.
2. Write `internal/tools/gather_remote_logs_test.go` — tests covering:
   - Gather file by path: creates local temp file with correct content
   - Gather journal unit: exports via `journalctl -u <unit>` to local file
   - Multiple hosts × multiple paths: all combinations gathered
   - Local file organization: `<tmpdir>/<host>/<flattened-path>`
   - Max file size enforced (skip with warning if exceeded)
   - Compressed remote file gathered as-is (not decompressed)
   - Output includes mapping: host → original → local path
   - Temp files registered for cleanup
   - Error: file not found on remote (per-file error, doesn't abort others)
   - Error: empty hosts or empty targets
   - Journal time range filtering (`--since`, `--until`)
3. Write `internal/tools/gather_remote_logs.go` — implement `RunGatherRemoteLogs`:
   - Input: `hosts []string`, `paths []string`, `journal_units []string`,
     `journal_since string`, `journal_until string`, `max_file_bytes int`
   - For files: `cat <path>` piped to local temp file with size check
   - For journals: `journalctl -u <unit> --no-pager -o short-iso [--since ...] [--until ...]`
   - Temp dir: `/tmp/log-analysis-mcp-<random>/<hostname>/<flattened-path>`
   - Register all temp paths in the same registry as `decompress_file`
   - Output: per-host per-target result with `local_path`, `size_bytes`, `error`
4. Register in `register.go` (tool #15).
5. `go test -race ./...` && `go vet ./...`.
6. Commit: `tools: add gather_remote_logs tool`

### Acceptance Criteria

- Gathered files are readable by all existing tools (`read_logs`, `search_logs`, etc.).
- `correlate_logs` works across files gathered from different hosts.
- Journal exports parse correctly with existing log parsers.
- Files cleaned up on server shutdown.
- Per-file size limit prevents downloading multi-GB files accidentally.

## Phase 5e: Integration & Prompt Update

### Steps

1. Add integration tests exercising the multi-tool workflow:
   - discover → gather → summarize (if SSH available, else skip)
   - gather → correlate across hosts
   - gather → diff between hosts
2. Update `specs/resources_and_prompts.md` — add `investigate_remote` prompt.
3. Implement `handleInvestigateRemote` in `internal/prompts/prompts.go`:
   - Arguments: `hosts` (required), `log_paths` (optional), `incident_id` (optional)
   - Workflow: discover → select → gather → summarize → errors → anomalies → correlate → report
4. Register prompt.
5. Update tool count in integration test (`TestListToolsReturnsAllN`).
6. `go test -race ./...` && `go vet ./...`.
7. Commit: `prompts: add investigate_remote prompt`

## Execution Order

| Sub-phase | Deliverable | Blocked by | Complexity |
|-----------|-------------|------------|------------|
| 5a | SSH infrastructure | — | Medium |
| 5b | `run_remote_command` | 5a | Small |
| 5c | `discover_remote_logs` | 5a | Medium |
| 5d | `gather_remote_logs` | 5a | Medium |
| 5e | Integration + prompt | 5b, 5c, 5d | Small |

5b, 5c, 5d can proceed in parallel after 5a.

## Future: Live Remote Tailing

Once Phase 4 research spike validates a streaming delivery mechanism:
- `watch_remote_logs` tool using `tail -f <files>` or `journalctl -f -u <units>` over SSH
- Reuses `internal/remote/` connection pooling
- Streams lines via progress notifications or validated alternative
- Blocked on: confirming Claude Desktop surfaces progress notifications

## Cleanup

- Delete this plan file when all sub-phases are complete.
- Mark Phase 5 complete in `plans/phase2-roadmap.md`.