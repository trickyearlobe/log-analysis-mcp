# Overall Improvement Plan

Source: Nuclia field reports, tool feedback (2026-03-29), rubber duck critique,
repo state audit (2026-05-05).

## Current State (already shipped)

- ✅ Compression: `fileutil.OpenReader` handles .gz/.bz2/.zip; `log_decompress` tool
- ✅ CI: Release workflow builds linux/windows/darwin × amd64/arm64
- ✅ Install: 6 IDEs supported (Claude Desktop, VS Code, Cursor, Windsurf, Zed, Copilot CLI)
- ✅ JSONC handling for Zed, extra fields for Zed + Copilot CLI
- ✅ 15 tools, 30 install tests, full TDD coverage

## Items

### 1. Add missing IDE targets + schema compatibility fixes

**Status:** `internal/install/` supports 6 IDEs. Three are missing.

**Remaining work:**
- Add JetBrains: path `.junie/mcp/mcp.json` (project) or `~/.junie/mcp/mcp.json`
  (user). Key: `mcpServers`. Same format as Claude Desktop. Use user-global path
  for `--install`.
- Add Visual Studio: path `.vs/mcp.json`. Key: `servers` (same as VS Code).
  v17.14+. Use project-level by convention? Or skip (Windows only, project-local)?
- Add Claude Code: path `~/.claude.json`. Key: `mcpServers`. No `type` field
  needed for stdio. Simplest entry: `{"command": "/path/to/binary", "args": [], "env": {}}`.

**Schema compatibility fixes (critical):**
- **`outputSchema` bug**: Current handlers return typed output structs
  (e.g. `ReadLogsOutput`). Go SDK v1.4.1 generates `outputSchema` from these →
  breaks Copilot external agent (tools silently don't appear). Fix: change all 15
  handler return types to `any`. Keep internal `Run*` functions returning concrete
  types for testability.
- **Empty input schema**: All current tools have non-empty input structs, so this
  doesn't currently affect us. Add a regression test to catch future additions.

**Scope:** Extend `internal/install/ide.go` + fix `internal/tools/register.go`
handler signatures + add compatibility test.

---

### 2. `record_separator` — Generic multi-line log entry grouping

**Problem:** Tools split multi-line log entries (Java stack traces, Erlang SASL,
PostgreSQL continuations, Python tracebacks, Go panics) into separate results.

**Solution:** Add optional `record_separator` regex parameter to
`log_extract_errors`, `log_summarize`, `log_filter`, `log_search`. The regex
matches the **start** of a new record. Everything between matches is one logical
entry.

**Safety bounds (from rubber duck critique):**
- Max record size: 64KB or 500 lines, whichever comes first. Emit truncated
  record and start fresh if exceeded.
- Lines before first separator match: treat each as its own record (fallback).
- Separator never appears: entire file is one record → hit max size cap → stream
  as individual lines (graceful degradation, not OOM).
- Offsets/pagination operate on **records**, not lines, when separator is active.

**Scope:** New `specs/record_separator.md` + shared record reader in
`internal/fileutil/` + updates to 4 tool specs + implementation.

---

### 3. Pagination for large results

**Problem:** No way to request "page 2" of results. `max_results` caps but
discards remainder.

**Solution:** Add `offset` parameter (count of records/matches to skip). Return
`total_matches` (exact or estimate) and `has_more` boolean.

**Design contract (consistent across all tools):**
- `offset` counts: matches for search/filter, clusters for extract_errors,
  anomalies for detect_anomalies.
- Return `next_offset` (= offset + len(results)) for convenience.
- `has_more: true` means more results available at `next_offset`.
- `total_matches`: exact for small files, estimate for large ones (based on
  sample rate).

**Dependency:** Do record_separator spec first so pagination counts records
when separator is active.

**Scope:** Spec `specs/pagination.md` + updates to log_search, log_filter,
log_extract_errors, log_detect_anomalies.

---

### 4. Impact-ranked error extraction

**Problem:** Error clusters sorted by count only. Frequency × severity ranking
would surface the most important errors first.

**Solution:** Add `sort_by` param (`"count"` default, `"impact"`) to
`log_extract_errors`. Impact = count × severity_weight (FATAL=10, CRITICAL=8,
ERROR=5, WARN=2). Add `impact_score` to output struct.

**Scope:** Single tool update. Low effort.

---

### 5. Fix `correlate_logs` tool description

**Problem:** Description says "by a shared field like request_id or trace_id" —
hides the time-window mode that doesn't need shared fields.

**Fix:** Change to: "Correlate events across multiple log files. Supports two
modes: (1) correlation by a shared field like request_id or trace_id, and (2)
time-window correlation that groups events occurring within N seconds of each
other across files — useful when files have no shared fields."

Also update `correlation_field` param description: "Optional — if omitted, use
time_window_seconds for timestamp-based correlation."

**Scope:** One-line change in `internal/tools/register.go`. Trivial.

---

### 6. `list_archive` tool

**Problem:** No way to discover paths inside tar.gz/zip without terminal.

**Solution:** New tool. Input: path, max_entries (default 200, max 1000), pattern
(glob filter). Output: entries with name, size, mod_time, is_dir. Stream entries.

**Design note:** Decide now whether other tools will accept `archive:entry` paths
or if the user must `log_decompress` first. Current approach: user decompresses
or uses `log_decompress`, then passes extracted path. Document this workflow.

**Scope:** New tool + new spec.

---

### 7. `count_by_level` quick tool

**Problem:** `log_summarize` is heavyweight for multi-file triage.

**Solution:** New tool. Single streaming pass. Returns: level→count map, total
lines, unparsed count. Minimal output for minimal context cost.

**Scope:** New tool + new spec.

---

### 8. `file_info` lightweight metadata tool

**Problem:** Agents need size/linecount before committing to expensive parsing.

**Solution:** Returns: path, size_bytes, line_count, first_timestamp,
last_timestamp (best-effort), compression_type, is_binary. Single pass.

**Scope:** New tool + new spec.

---

### 9. Expand parser format hints

**Problem:** Auto-detection fails on Erlang/OTP, Habitat, journalctl formats.

**Solution:** Add format hints and parsers. Depends on record_separator being
available first (Erlang especially needs multi-line grouping).

**Scope:** New parsers + spec updates. Requires format research.

---

## Priority Order

| Pri | Item | Effort | Value | Rationale |
|-----|------|--------|-------|-----------|
| 1 | #1 IDE targets + schema fixes | Low-Med | Very High | Unblocks 3 IDEs + fixes silent Copilot agent bug |
| 2 | #5 correlate_logs description | Trivial | Medium | 1-line fix, ship immediately |
| 3 | #4 Impact sort | Low | High | Quick win, context pressure |
| 4 | #2 record_separator | Medium | Very High | Core analysis improvement; spec first |
| 5 | #3 Pagination | Medium | High | Depends on record_separator spec |
| 6 | #6 list_archive | Low-Med | Medium | Workflow gap |
| 7 | #7 count_by_level | Low | Medium | Fast triage |
| 8 | #8 file_info | Low | Medium | Pre-analysis decisions |
| 9 | #9 Parser formats | Medium | Low-Med | Depends on record_separator |

## Dependencies

```
record_separator → pagination (pagination counts records when active)
record_separator → parser_formats (Erlang needs multi-line grouping)
```

## Acceptance

All items done when: specs written/updated, tests pass, `go test -race ./...`
clean, `go vet ./...` clean. Delete this plan when complete.
