# Install Flag

## Goal

Add a `--install` flag to the binary that registers it as an MCP server in
common IDEs. Running `log-analysis-mcp --install` writes or updates the
config files for each detected IDE, then exits.

## Specs to Read

- `specs/build_and_run.md` — current run instructions, Claude Desktop config

## Target IDEs

| IDE | Config path (macOS) | Top-level key | Format |
|-----|---------------------|---------------|--------|
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` | `mcpServers` | `{"command":"/abs/path"}` |
| VS Code | `~/.vscode/mcp.json` (global user) | `servers` | `{"command":"/abs/path"}` |
| Cursor | `~/.cursor/mcp.json` | `servers` | `{"command":"/abs/path"}` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | `mcpServers` | `{"command":"/abs/path"}` |
| Zed | `~/.config/zed/settings.json` | `context_servers` | `{"command":"/abs/path"}` |

Server name in all configs: `log-analysis-mcp`.

## Design

### New package: `internal/install`

**`install.go`:**
- `Run() error` — main entry point called from `main.go` when `--install` is set
- Resolves absolute path of the current binary via `os.Executable()` + `filepath.EvalSymlinks()`
- For each IDE, calls the appropriate installer
- Prints results to stderr (NEVER stdout — MCP owns it)
- Returns error only on fatal issues (config parse failure, write failure)
- Skips IDEs whose config directory does not exist (not installed)

**`ide.go`:**
- `type IDE struct { Name, ConfigPath, ServerKey string, TopLevelKey string }`
- `func AllIDEs() []IDE` — returns the list above with paths expanded
- Each IDE entry includes a `Format` field: `"mcpServers"` or `"servers"` or `"context_servers"`

**`config.go`:**
- `func upsertMCPServer(configPath, topKey, serverName, binaryPath string) (action string, err error)`
- Reads existing JSON (or starts with `{}` if file doesn't exist)
- Parses as `map[string]any` (preserves unknown fields)
- Creates or updates the server entry under the top-level key
- For Zed: `context_servers` is nested inside the existing `settings.json` — must merge, not overwrite
- Writes back with `json.MarshalIndent` (2-space indent)
- Creates parent directories if needed (`os.MkdirAll`)
- Returns action: `"installed"`, `"updated"`, `"already up to date"`, or `"skipped (not installed)"`

**`install_test.go`:**
- Tests with temp directories standing in for each IDE config location
- Verifies: new file creation, update existing, idempotent re-run, preserves other keys

### Modified: `cmd/log-analysis-mcp/main.go`

- Add `flag.Bool("install", false, "Register as MCP server in supported IDEs")`
- If set, call `install.Run()` and exit (do not start the MCP server)
- Print summary table to stderr: IDE name, action taken

### Also support: `--uninstall`

- `flag.Bool("uninstall", false, "Remove from supported IDE MCP configs")`
- Removes the `log-analysis-mcp` entry from each config file
- Skips files that don't exist or don't contain the entry

## Steps

1. Write `internal/install/ide.go` — IDE definitions
2. Write `internal/install/config.go` — JSON config read/merge/write
3. Write `internal/install/install.go` — Run() entry point
4. Write `internal/install/install_test.go` — tests
5. Modify `cmd/log-analysis-mcp/main.go` — flag parsing
6. Run `go test -race ./...` && `go vet ./...`
7. Update `specs/build_and_run.md` with install/uninstall docs
8. Commit

## Acceptance Criteria

- `./bin/log-analysis-mcp --install` registers in all detected IDEs
- `./bin/log-analysis-mcp --install` is idempotent (re-run changes nothing)
- `./bin/log-analysis-mcp --uninstall` removes from all detected IDEs
- Existing config keys in each file are preserved
- IDEs that aren't installed are skipped with a message
- Nothing is written to stdout
- All existing tests pass