# Plan: integration tests

## Goal

Add integration tests that exercise the full MCP stack: client → server → tool/resource/prompt handlers → parsers → fileutil → real log files on disk.

## Specs to read

- `specs/server_entry.md` (server wiring — already read)
- `specs/resources_and_prompts.md` (resource + prompt contracts — already read)
- `specs/tools/*.md` (tool I/O contracts — already read)
- MCP Go SDK in-memory transport API (already reviewed)

## Test infrastructure

- Use `mcp.NewInMemoryTransports()` to connect a real client ↔ server pair in-process.
- Use `t.TempDir()` for all test log files — no fixtures checked in.
- Helper: `setupTestServer(t) *mcp.ClientSession` — wires server, client, connects, returns session.
- Helper: `writeLogFile(t, dir, name, lines []string) string` — writes temp log, returns path.
- Helper: `callTool[T any](t, session, toolName string, args any) T` — calls tool, unmarshals JSON result.

## Ordered steps

1. **Create `internal/integration/integration_test.go`** — test infrastructure + tool tests.
2. **Create `internal/integration/resource_test.go`** — resource template tests.
3. **Create `internal/integration/prompt_test.go`** — prompt template tests.
4. Run `go test -race ./internal/integration/`.
5. Commit.
6. Delete this plan.

## Test cases

### Tools (integration_test.go)

| Test | What it exercises |
|------|-------------------|
| ListTools returns all 10 | Server registration wiring |
| read_logs round-trip | Client → server → fileutil.ReadLines → JSON response |
| tail_logs round-trip | Client → server → fileutil.TailLines → JSON response |
| search_logs round-trip | Client → server → regex compile → stream → matches |
| parse_logs auto-detect JSON | Client → server → parsers.AutoDetect → JSON parser → records |
| parse_logs auto-detect syslog | Client → server → parsers.AutoDetect → syslog parser → records |
| filter_logs by level | Client → server → parser → level filter → entries |
| extract_errors clustering | Client → server → parser → normalize → clusters |
| summarize_logs statistics | Client → server → parser → single-pass stats |
| detect_anomalies error spike | Client → server → parser → window analysis → anomalies |
| timeline event classification | Client → server → parser → classify → events |
| correlate_logs cross-file | Client → server → multi-file parse → correlation groups |
| tool error propagation | File not found → IsError:true in CallToolResult |
| binary file rejection | Null bytes → BINARY_FILE error through MCP |

### Resources (resource_test.go)

| Test | What it exercises |
|------|-------------------|
| ListResourceTemplates | Returns log:///{path} template |
| ReadResource valid file | Returns first 100 lines as text content |
| ReadResource missing file | Returns error |

### Prompts (prompt_test.go)

| Test | What it exercises |
|------|-------------------|
| ListPrompts returns both | investigate_error + log_health_check registered |
| GetPrompt investigate_error with pattern | Full prompt text with error_pattern interpolated |
| GetPrompt investigate_error without pattern | Prompt text without "matching" clause |
| GetPrompt log_health_check | Full prompt text with log_path interpolated |

## Acceptance criteria

- [ ] All 10 tools callable via MCP client session and return valid JSON.
- [ ] Error cases propagate correctly through the MCP protocol (IsError:true).
- [ ] Resource template resolves and returns file content.
- [ ] Both prompts return correctly templated messages.
- [ ] `go test -race ./internal/integration/` passes.
- [ ] `go vet ./...` clean.
- [ ] No external dependencies added.
- [ ] Tests use real temp files, not mocks.