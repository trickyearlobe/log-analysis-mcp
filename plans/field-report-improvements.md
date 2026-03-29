# Field Report Improvements

Source: Nuclia resource `afaade4be83e4ca392e89b0b77f43723` (Field Experience Report, 2026-03-18).
Context: Real-world analysis of Chef Automate HA cluster gather-logs bundle (30 nodes, 31 tarballs).
Rating: 8.5/10. Zero bugs found.

## Items

### 1. `list_archive` tool — Browse tar/gz/zip contents

**Problem:** No way to discover internal paths inside a tar.gz without falling back to `tar tzf` in terminal. Every analysis session started with manual archive browsing.

**Scope:** New tool. New spec at `specs/tools/list_archive.md`.

**Spec notes:**
- Input: `path` (required), `max_entries` (default 200, max 1000), `pattern` (glob filter).
- Output: list of entries with name, size, mod time, is_dir flag. Total count.
- Support `.tar.gz`, `.tar.bz2`, `.tar`, `.zip`. Use stdlib `archive/tar`, `archive/zip`, `compress/gzip`, `compress/bzip2`.
- Stream entries; do not load full archive into memory.

**Steps:**
1. Write spec.
2. TDD: `internal/tools/list_archive.go` + `list_archive_test.go`.
3. Register in server.
4. `go test -race ./...` && `go vet ./...`.

---

### 2. Time-based correlation mode for `log_correlate`

**Problem:** `log_correlate` requires shared field IDs (`request_id`, `trace_id`). Infrastructure logs rarely have cross-tier correlation IDs. The tool was used once and rated "Limited."

**Scope:** Extend existing tool. Update spec `specs/tools/log_correlate.md`.

**Spec notes:**
- New optional param `mode`: `"field"` (default, current behaviour) or `"time"`.
- In `"time"` mode, ignore `correlation_field`. Instead:
  - Parse all files, extract timestamped entries.
  - Merge-sort by timestamp across files.
  - Group entries from different files that fall within `time_window_seconds` of each other.
  - Return groups where 2+ files contribute entries.
- Add optional `level` filter (e.g. only correlate ERROR/FATAL entries) to reduce noise.
- Output struct unchanged; `correlation_id` becomes a synthetic group ID (e.g. `time-group-001`), `correlation_field` becomes `"timestamp_proximity"`.

**Steps:**
1. Update spec with `mode` param and time-based strategy.
2. TDD: add time-mode tests to `log_correlate_test.go`.
3. Implement time-based correlation path.
4. Existing field-mode tests must still pass.
5. `go test -race ./...` && `go vet ./...`.

---

### 3. "Top N most impactful" mode for `log_extract_errors`

**Problem:** With 500K+ line files, returned clusters can be large enough to blow LLM context windows. Current sorting is by count only. Reviewer wanted frequency × severity ranking.

**Scope:** Extend existing tool. Update spec `specs/tools/log_extract_errors.md`.

**Spec notes:**
- New optional param `sort_by`: `"count"` (default, current behaviour) or `"impact"`.
- Impact score: `count × severity_weight`. Severity weights: FATAL=10, CRITICAL=8, ERROR=5, WARN=2.
- When `sort_by=impact`, clusters are ranked by impact score descending.
- Add `impact_score` field to `ErrorCluster` output struct.
- Mixed-level clusters: use the highest severity level present in the cluster.

**Steps:**
1. Update spec with `sort_by` param and impact scoring.
2. TDD: add impact-sort tests to `log_extract_errors_test.go`.
3. Implement severity weight lookup and impact sort path.
4. Existing count-sort tests must still pass.
5. `go test -race ./...` && `go vet ./...`.

---

### 4. Expand `log_parse` format hints

**Problem:** Auto-detection failed on Erlang/OTP crash logs, Habitat supervisor output, and Chef Automate journalctl. `format_hint` didn't help because those formats aren't in the parser registry.

**Scope:** Extend parsers. Update `specs/parsers.md` and `specs/tools/log_parse.md`.

**Spec notes:**
- Add format hints: `"erlang"`, `"habitat"`, `"journalctl"`.
- Erlang crash log: multi-line `=CRASH REPORT====` / `=ERROR REPORT====` blocks with `{Module,Function,Arity}` tuples.
- Habitat supervisor: `hab-sup` prefix, custom timestamp `YYYY-MM-DD HH:MM:SS`.
- journalctl: `Mon DD HH:MM:SS hostname unit[PID]:` variant (similar to syslog but with systemd unit names).
- Each gets a new parser implementing the existing `Parser` interface.

**Steps:**
1. Gather sample log lines for each format (research task).
2. Update `specs/parsers.md` with regex patterns and field mappings.
3. Update `specs/tools/log_parse.md` enum with new hints.
4. TDD: one parser file + test file per format in `internal/parsers/`.
5. Register parsers in the detection chain.
6. `go test -race ./...` && `go vet ./...`.

---

### 5. `count_by_level` quick tool

**Problem:** `log_summarize` is the go-to orientation tool but does full statistical analysis. When triaging many files, a lightweight level-count-only tool would be faster and produce smaller output.

**Scope:** New tool. New spec at `specs/tools/count_by_level.md`.

**Spec notes:**
- Input: `path` (required).
- Output: map of level → count, total lines, unparsed count. No top sources, no top errors, no throughput metrics.
- Single streaming pass. Minimal output for minimal context window cost.
- Use existing parser chain for level detection.

**Steps:**
1. Write spec.
2. TDD: `internal/tools/count_by_level.go` + `count_by_level_test.go`.
3. Register in server.
4. `go test -race ./...` && `go vet ./...`.

---

### 6. Pagination for large tool results

**Problem:** Tools like `log_search`, `log_filter`, and `log_extract_errors` can return hundreds of matches. The `max_results`/`max_clusters` params help but there's no way to request "page 2."

**Scope:** Cross-cutting. Affects multiple tools. Needs design spec.

**Spec notes:**
- Add optional `offset` param to `log_search`, `log_filter`, `log_extract_errors`.
- Return `total_matches` (or estimate) and `has_more` boolean alongside results.
- For streaming tools: offset means "skip N matches before collecting."
- Do not break existing callers — offset defaults to 0.
- Consider whether MCP protocol has pagination conventions to align with.

**Steps:**
1. Write cross-cutting spec section in `specs/performance.md` or new `specs/pagination.md`.
2. Update individual tool specs with offset param.
3. TDD: add pagination tests to each affected tool.
4. Implement offset logic in each tool handler.
5. Existing tests must still pass.
6. `go test -race ./...` && `go vet ./...`.

## Priority Order

| Priority | Item | Rationale |
|----------|------|-----------|
| 1 | #3 Impact sort | Low effort, high value. Directly addresses context window pressure. |
| 2 | #1 list_archive | Fills a clear workflow gap. Stdlib only. |
| 3 | #5 count_by_level | Small tool, fast win. Reduces context usage for multi-file triage. |
| 4 | #2 Time correlation | High value but medium complexity. Unlocks cross-tier analysis. |
| 5 | #6 Pagination | Cross-cutting change, needs careful design. |
| 6 | #4 Parser formats | Requires format research. Useful but domain-specific. |

## Acceptance

All items done when: specs updated, tests pass, `go test -race ./...` clean, `go vet ./...` clean.
Delete this plan when all items are complete or moved to individual plans.