# Tool Rename: Add `log_` Namespace Prefix

## Goal

Rename all 15 tools with a `log_` prefix per MCP best practices for LLM discoverability. Drop redundant `_logs` suffix where the prefix makes it unnecessary.

## Naming Map

| Current | New |
|---|---|
| `log_read` | `log_read` |
| `log_tail` | `log_tail` |
| `log_search` | `log_search` |
| `log_parse` | `log_parse` |
| `log_filter` | `log_filter` |
| `log_extract_errors` | `log_extract_errors` |
| `log_summarize` | `log_summarize` |
| `log_detect_anomalies` | `log_detect_anomalies` |
| `timeline` | `log_timeline` |
| `log_correlate` | `log_correlate` |
| `log_decompress` | `log_decompress` |
| `log_diff` | `log_diff` |
| `log_run_remote_command` | `log_run_remote_command` |
| `log_discover_remote` | `log_discover_remote` |
| `log_gather_remote` | `log_gather_remote` |

## Scope

### Go source (name strings + error prefixes + GoDoc)
- `internal/tools/register.go` — all 15 Name strings
- `internal/tools/*.go` — `fmt.Errorf` prefixes and GoDoc comments (15 files)
- `internal/tools/timeline_test.go` — test fixture string
- `internal/prompts/prompts.go` — prompt template text
- `internal/types/types.go` — GoDoc comments on ErrorCluster, TimelineEvent
- `internal/integration/integration_test.go` — tool name map + callTool strings
- `internal/integration/prompt_test.go` — prompt assertion strings
- `internal/integration/remote_test.go` — callToolRemote strings

### Go source files to rename
- 15 tool files: e.g. `log_read.go` → `log_read.go`
- 15 test files: e.g. `log_read_test.go` → `log_read_test.go`

### Spec files to rename
- 15 files in `specs/tools/`: e.g. `log_read.md` → `log_read.md`

### Spec content to update
- `specs/server_entry.md` — tool table + filename mentions
- `specs/performance.md` — pagination table + perf sections
- `specs/error_handling.md` — error message examples
- `specs/fileutil.md` — concurrent processing mention
- `specs/compression.md` — log_decompress tool section
- `specs/remote.md` — timeout/output policy tables
- `specs/build_and_run.md` — macOS firewall section
- `specs/future.md` — log_diff mention
- `specs/types.md` — GoDoc on shared types
- `specs/resources_and_prompts.md` — all 4 prompt definitions
- All 15 tool spec files — headings, registration blocks, cross-refs

### Plans to update
- `plans/phase2-roadmap.md`
- `plans/field-report-improvements.md`

## Steps

1. Create plan (this file).
2. Rename Go source files (tool files + test files).
3. Rename spec files.
4. Update all Name strings, error prefixes, GoDoc in Go source.
5. Update prompts, types, integration tests.
6. Update all spec content.
7. Update plans.
8. `go test -race ./...` && `go vet ./...`.
9. Delete this plan.

## Acceptance

- All 15 tools registered with `log_` prefix.
- All tests pass.
- All specs reference new names.
- `go vet ./...` clean.
