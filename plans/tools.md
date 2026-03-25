# Plan: tools package

## Goal

Implement `internal/tools/` — 10 MCP tools per `specs/tools/*.md`. One file per tool. One test file beside each.

## Specs to read

- `specs/tools/*.md` (10 tool specs — summarised below)
- `specs/error_handling.md` (error codes, binary detection, partial results — already read)
- `specs/performance.md` (pagination defaults, output caps — already read)

## Tool dependency tiers

**Tier 1 — raw text tools (fileutil + types only):**
- `read_logs` — paginated line reader
- `tail_logs` — last N lines via seek-from-end
- `search_logs` — regex/text search with context lines

**Tier 2 — single-file parsed tools (+ parsers):**
- `parse_logs` — auto-detect + parse with pagination
- `filter_logs` — parse then filter by level/time/source/message
- `extract_errors` — cluster ERROR/FATAL/CRITICAL entries
- `summarize_logs` — single-pass statistics
- `detect_anomalies` — sliding-window rate analysis
- `timeline` — classify events, filter by time/type

**Tier 3 — multi-file tool (+ fileutil.ProcessFiles):**
- `correlate_logs` — cross-file correlation by field

## Ordered steps

1. **Create `internal/tools/toolutil.go`** — shared helpers.
   - `CheckFileAccess(path string) error` — stat + binary check.
   - `SampleLines(path string, n int) ([]string, error)` — read first N non-empty lines.
   - `ClampInt(val, min, max int) int`.
   - Common error wrapping with ToolError codes.

2. **Tier 1 tools (parallel):**
   - `read_logs.go` + `read_logs_test.go`
   - `tail_logs.go` + `tail_logs_test.go`
   - `search_logs.go` + `search_logs_test.go`

3. **Tier 2 tools (parallel):**
   - `parse_logs.go` + `parse_logs_test.go`
   - `filter_logs.go` + `filter_logs_test.go`
   - `extract_errors.go` + `extract_errors_test.go`
   - `summarize_logs.go` + `summarize_logs_test.go`
   - `detect_anomalies.go` + `detect_anomalies_test.go`
   - `timeline.go` + `timeline_test.go`

4. **Tier 3:**
   - `correlate_logs.go` + `correlate_logs_test.go`

5. **Run `go test -race ./internal/tools/` and `go vet ./...`**.

6. **Commit**: `tools: implement all 10 MCP tools`.

7. **Delete this plan**.

## Per-tool summary

| Tool | Input (key params) | Output struct | Core logic |
|------|--------------------|---------------|------------|
| `read_logs` | path, start_line=1, num_lines=100 | Lines, TotalLines, HasMore, FileSize, Range | fileutil.ReadLines + file stat |
| `tail_logs` | path, num_lines=50 | Lines, TotalLines, FileSize, ShowingFromLine | fileutil.TailLines |
| `search_logs` | path, pattern, is_regex=false, case_sensitive=false, context_lines=0, max_results=50 | Matches, TotalMatches, SearchedLines, Truncated | stream + regex/text match + context ring buffer |
| `parse_logs` | path, start_line=1, num_lines=50, format_hint="auto" | Format, Confidence, Records, ParseErrors | AutoDetectWithHint + parse page |
| `filter_logs` | path, level[], after, before, source, message_pattern, max_results=100 | Entries, TotalMatched, TotalScanned, Filters, Truncated | parse + AND filter chain |
| `extract_errors` | path, include_stack_traces=true, max_clusters=20 | Clusters, TotalErrors, ErrorRate | parse + normalize + cluster by pattern |
| `summarize_logs` | path, sample_size=0 | FileInfo, Format, LevelDist, TopSources, TopErrors, Throughput | single-pass streaming stats |
| `detect_anomalies` | path, window_minutes=5, sensitivity="medium" | Anomalies, Metadata | time-bucket + baseline + threshold |
| `timeline` | path, after, before, event_types[], max_events=100 | Events, TimeSpan, EventCount, Truncated | parse + classify + filter + sort |
| `correlate_logs` | paths[], correlation_field="request_id", time_window_seconds=60 | Groups, TotalGroups, FilesAnalyzed | multi-file parse + group by field + filter cross-file |

## Acceptance criteria

- [ ] All 10 tools implemented, one file each.
- [ ] Each tool has table-driven tests covering happy path + edge cases.
- [ ] Binary file detection before processing.
- [ ] Pagination respected per specs/performance.md defaults.
- [ ] All tool outputs are Go structs with JSON tags.
- [ ] No full-file buffering — streaming throughout.
- [ ] `go test -race ./internal/tools/` passes.
- [ ] `go vet ./...` clean.
- [ ] No external dependencies added.
- [ ] No panics — all errors returned.
- [ ] No writes to stdout.