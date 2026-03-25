# Plan: filter_logs tool

## Goal
Implement the `filter_logs` tool that filters parsed log entries by level, time range, source, and message pattern using AND logic.

## Specs to read
- `specs/tools/filter_logs.md` — tool contract
- `specs/types.md` — shared types (ParsedLogEntry, LogLevel)
- `specs/parsers.md` — AutoDetectWithHint, Parser interface
- `specs/error_handling.md` — error codes (INVALID_TIMESTAMP, FILE_NOT_FOUND)

## Steps
1. Create `internal/tools/filter_logs.go` with:
   - FilterLogsInput, FilteredEntry, AppliedFilters, FilterLogsOutput structs
   - RunFilterLogs function implementing the streaming filter algorithm
2. Create `internal/tools/filter_logs_test.go` with table-driven tests covering:
   - Filter by level (single, multiple)
   - Filter by time range (after, before, both)
   - Filter by source regex
   - Filter by message_pattern regex
   - Combined AND filters
   - No filters → all parsed entries
   - No matches
   - MaxResults truncation
   - File not found error
   - Invalid timestamp format error
   - Empty file
3. Run `go test -race ./internal/tools/` to verify
4. Run `go vet ./...` to lint

## Acceptance criteria
- All tests pass with `-race`
- `go vet` clean
- Streaming page-by-page (never whole file in memory)
- No stdout writes
- No panics — all errors returned
- Uses existing toolutil helpers (CheckFileAccess, SampleLines, DefaultInt, ClampInt, CompilePattern)
- Does not redefine types from other files in the package