# Phase 5e: Integration Tests + investigate_remote Prompt

## Goal

Complete the remote SSH story with an `investigate_remote` prompt and integration
tests that exercise the multi-tool remote workflow.

## Specs to Read

- `specs/resources_and_prompts.md` — existing prompt patterns and registration
- `specs/tools/run_remote_command.md` — remote command tool contract
- `specs/tools/discover_remote_logs.md` — log discovery tool contract
- `specs/tools/gather_remote_logs.md` — log gathering tool contract
- `specs/remote.md` — SSH infrastructure, connection strategy, prereqs

## Deliverables

### 1. `investigate_remote` Prompt

**File:** `internal/prompts/prompts.go` (append handler + registration)

**Arguments:**
- `hosts` (required) — comma-separated list of SSH targets
- `log_paths` (optional) — specific remote log paths to gather
- `incident_id` (optional) — incident ID for report header

**Workflow steps in prompt text:**
1. Discovery — `discover_remote_logs` on all hosts
2. Selection — AI picks relevant logs from discovery output
3. Gathering — `gather_remote_logs` to download selected logs
4. System check — `run_remote_command` for uptime, disk, memory
5. Summary — `summarize_logs` on each gathered file
6. Errors — `extract_errors` on each gathered file
7. Anomalies — `detect_anomalies` on each gathered file
8. Correlation — `correlate_logs` across gathered files
9. Comparison — `diff_logs` between hosts if multiple
10. Report — compile findings into structured Markdown

**Conditional sections:**
- `log_paths` provided → skip discovery, go straight to gather
- `log_paths` omitted → run discovery first
- `incident_id` provided → include in report header
- Single host → skip cross-host correlation and diff steps

### 2. Prompt Spec Update

**File:** `specs/resources_and_prompts.md` (append `investigate_remote` section)

### 3. Prompt Integration Tests

**File:** `internal/integration/prompt_test.go` (append new tests)

Tests:
- `TestListPrompts` — update count from 3 to 4, add name check
- `TestGetPromptInvestigateRemoteFullArgs` — all args, verify all tools referenced
- `TestGetPromptInvestigateRemoteMinimalArgs` — hosts only, verify discovery step
- `TestGetPromptInvestigateRemoteWithPaths` — hosts + paths, verify skip discovery
- `TestGetPromptInvestigateRemoteMissingHosts` — error for missing required arg

### 4. Remote Tool Integration Tests (SSH-guarded)

**File:** `internal/integration/remote_test.go` (new file)

All tests guarded by `SSH_TEST_HOST` env var. Skipped when not set.

Tests:
- `TestRemoteRunCommand` — run `echo hello` on test host
- `TestRemoteDiscoverLogs` — discover logs on test host, verify structured output
- `TestRemoteGatherAndSummarize` — gather a log → summarize it (multi-tool chain)
- `TestRemoteGatherAndDiff` — gather from two paths → diff them (if possible)

### 5. Tool Count Update

**File:** `internal/integration/integration_test.go`

- No tool count change (still 15 tools, we're adding a prompt not a tool)

## Steps

1. Write spec section for `investigate_remote` in `specs/resources_and_prompts.md`
2. Write prompt tests in `internal/integration/prompt_test.go`
3. Implement `handleInvestigateRemote` in `internal/prompts/prompts.go`
4. Register prompt in `Register()`, update `TestListPrompts` count to 4
5. Run `go test -race ./internal/integration/`
6. Create `internal/integration/remote_test.go` with SSH-guarded tests
7. Run `go test -race ./...` && `go vet ./...`
8. Commit

## Acceptance Criteria

- `investigate_remote` prompt registered and returns correct text for all arg combos
- Prompt references all remote tools + analysis tools by name
- `TestListPrompts` expects 4 prompts
- SSH-guarded tests pass when `SSH_TEST_HOST` is set, skip cleanly when not
- All existing 840+ tests still pass