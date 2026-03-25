# Phase 2 & 3: Log Diff Tool + Report Generation Prompt

## Goal

Implement `diff_logs` tool (Phase 2) and `generate_report` prompt (Phase 3) from
the roadmap in `plans/phase2-roadmap.md`.

## Specs to Read

- `specs/tools/extract_errors.md` — error clustering / normalizeMessage reuse
- `specs/tools/summarize_logs.md` — summary stat types (LevelStats, SourceCount, ThroughputInfo)
- `specs/types.md` — shared types (ErrorCluster, TimeRange, SeenAt)
- `specs/resources_and_prompts.md` — existing prompt patterns
- `specs/fileutil.md` — ReadLines streaming pattern
- `specs/performance.md` — streaming constraints

## Phase 2: `diff_logs` Tool

### Steps

1. Write spec `specs/tools/diff_logs.md` — input/output contracts, two modes, behaviour.
2. Write `internal/tools/diff_logs_test.go` — table-driven tests covering:
   - File-vs-file comparison (different error profiles)
   - File-vs-file comparison (identical files → no changes)
   - Time-range-vs-time-range within one file
   - New errors detected in "after" not present in "before"
   - Resolved errors present in "before" but absent in "after"
   - Rate changes (throughput increase/decrease)
   - Source changes (new sources, disappeared sources)
   - Level distribution shift
   - Error: missing file
   - Error: invalid time range (after > before)
   - Error: no parseable entries
   - Compressed file support (.gz)
3. Write `internal/tools/diff_logs.go` — implement `RunDiffLogs` to pass tests:
   - Reuse `normalizeMessage` from extract_errors.go
   - Reuse `parseTimestamp` from extract_errors.go
   - Reuse `isErrorLevel` pattern
   - Stream each file/period once via `fileutil.ReadLines` loop
   - Accumulate per-side stats in a `diffAccumulator` struct
   - Compute diff output from the two accumulators
4. Register `diff_logs` in `internal/tools/register.go` (tool #12).
5. Add integration test in `internal/integration/integration_test.go`.
6. Run `go test -race ./...` && `go vet ./...`.
7. Commit: `tools: add diff_logs tool`

### Acceptance Criteria

- `diff_logs` with two files returns structured JSON with all diff sections.
- `diff_logs` with one file + two time ranges returns the same structure.
- Identical inputs produce empty diff sections (no false positives).
- Compressed files work transparently.
- All existing 724+ tests still pass.

## Phase 3: `generate_report` Prompt

### Steps

1. Update spec `specs/resources_and_prompts.md` — append `generate_report` prompt definition.
2. Write `handleGenerateReport` in `internal/prompts/prompts.go`.
3. Register prompt in `Register()` function.
4. Add integration test.
5. Run `go test -race ./...` && `go vet ./...`.
6. Commit: `prompts: add generate_report prompt`

### Acceptance Criteria

- Prompt registered and discoverable via `prompts/list`.
- Arguments: `log_path` (required), `comparison_path` (optional), `incident_id` (optional).
- When `comparison_path` provided, prompt includes `diff_logs` step.
- When `comparison_path` omitted, diff step is skipped.
- `incident_id` appears in report header when provided.
- Prompt references all relevant tools by name.

## Cleanup

- Delete this plan file when both phases are complete.
- Delete `plans/phase2-roadmap.md` Phase 2/3 sections are done (or mark complete).