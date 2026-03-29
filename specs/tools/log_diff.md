# Tool: `log_diff`

## Description (shown to LLM)

> Compare two log files or two time periods within a single file and highlight differences: new error types, resolved errors, rate changes, source changes, and throughput shifts. Useful for before/after deployment comparisons, incident investigation, and trend analysis.

## Input Schema

| Parameter    | Type     | Required | Default | Description                                                                 |
| ------------ | -------- | -------- | ------- | --------------------------------------------------------------------------- |
| `base_path`  | `string` | Yes      | —       | Path to the baseline ("before") log file                                    |
| `target_path`| `string` | No       | —       | Path to the target ("after") log file. Omit for single-file time range mode |
| `base_after` | `string` | No       | —       | ISO 8601 timestamp — start of baseline period                               |
| `base_before`| `string` | No       | —       | ISO 8601 timestamp — end of baseline period                                 |
| `target_after`| `string`| No       | —       | ISO 8601 timestamp — start of target period                                 |
| `target_before`|`string`| No       | —       | ISO 8601 timestamp — end of target period                                   |

## Go Input Struct

```go
type DiffLogsInput struct {
    BasePath     string `json:"base_path"                jsonschema:"Path to the baseline (before) log file"`
    TargetPath   string `json:"target_path,omitempty"     jsonschema:"Path to the target (after) log file; omit for single-file time range mode"`
    BaseAfter    string `json:"base_after,omitempty"      jsonschema:"ISO 8601 timestamp — start of baseline period"`
    BaseBefore   string `json:"base_before,omitempty"     jsonschema:"ISO 8601 timestamp — end of baseline period"`
    TargetAfter  string `json:"target_after,omitempty"    jsonschema:"ISO 8601 timestamp — start of target period"`
    TargetBefore string `json:"target_before,omitempty"   jsonschema:"ISO 8601 timestamp — end of target period"`
}
```

> **Note:** The SDK infers `required` from the absence of `omitempty` in the `json` tag.
> The `jsonschema` tag contains only the description text — no `required` or `description=` prefixes.

## Modes

### File-vs-file mode

`base_path` and `target_path` are both set. Each file is scanned in full (or within
its respective time range if `base_after`/`base_before`/`target_after`/`target_before`
are provided).

### Time-range-vs-time-range mode (single file)

Only `base_path` is set. Both `base_after`+`base_before` and `target_after`+`target_before`
must be provided to define two periods within the same file. The file is scanned once;
each entry is assigned to the base period, target period, or neither based on its timestamp.

## Validation Rules

- `base_path` is always required.
- If `target_path` is empty, all four time range parameters must be provided.
- If any time range parameter is provided, it must parse as a valid timestamp.
- In single-file mode, `base_before` must be ≤ `target_after` (periods must not overlap).
  If they overlap, return an error.

## Algorithm

1. **Detect format** — sample lines from `base_path` (and `target_path` if set) via
   `parsers.AutoDetectWithHint`. Use the detected parser for both sides.

2. **Accumulate base stats** — stream `base_path` via `fileutil.ReadLines` in pages.
   For each parsed entry within the time window (if any):
   - Count total lines and parsed lines.
   - Bucket by level (map[string]int).
   - Bucket by source (map[string]int).
   - Track error messages: normalize with `normalizeMessage` (reused from
     `log_extract_errors.go`), accumulate into a `map[string]*clusterAccumulator`.
   - Track earliest/latest timestamp.
   - Bucket by minute for throughput.

3. **Accumulate target stats** — same as step 2 but for the target file/period.
   In single-file mode, this happens in the same scan pass as step 2.

4. **Compute diff** — compare the two accumulators:
   - **New errors**: patterns in target but not in base.
   - **Resolved errors**: patterns in base but not in target.
   - **Common errors with count change**: patterns in both, sorted by absolute count change.
   - **Level distribution change**: per-level count and percentage delta.
   - **Source changes**: new sources, disappeared sources, top movers by count delta.
   - **Throughput comparison**: lines/minute average, peak minute for each side.

5. **Return** — structured `DiffLogsOutput`.

## Go Output Structs

```go
type ErrorDiff struct {
    Pattern    string `json:"pattern"`
    BaseCount  int    `json:"base_count"`
    TargetCount int   `json:"target_count"`
    Change     int    `json:"change"`
}

type LevelDiff struct {
    Level          string  `json:"level"`
    BaseCount      int     `json:"base_count"`
    BasePercentage float64 `json:"base_percentage"`
    TargetCount    int     `json:"target_count"`
    TargetPercentage float64 `json:"target_percentage"`
}

type SourceDiff struct {
    Source      string `json:"source"`
    BaseCount   int    `json:"base_count"`
    TargetCount int    `json:"target_count"`
    Change      int    `json:"change"`
}

type PeriodSummary struct {
    Path           string  `json:"path"`
    TotalLines     int     `json:"total_lines"`
    ParsedLines    int     `json:"parsed_lines"`
    ErrorCount     int     `json:"error_count"`
    Earliest       string  `json:"earliest,omitempty"`
    Latest         string  `json:"latest,omitempty"`
    LinesPerMinute float64 `json:"lines_per_minute"`
}

type DiffLogsOutput struct {
    BaseSummary      PeriodSummary `json:"base_summary"`
    TargetSummary    PeriodSummary `json:"target_summary"`
    NewErrors        []ErrorDiff   `json:"new_errors"`
    ResolvedErrors   []ErrorDiff   `json:"resolved_errors"`
    ChangedErrors    []ErrorDiff   `json:"changed_errors"`
    LevelChanges     []LevelDiff   `json:"level_changes"`
    NewSources       []SourceDiff  `json:"new_sources"`
    DisappearedSources []SourceDiff `json:"disappeared_sources"`
    ChangedSources   []SourceDiff  `json:"changed_sources"`
}
```

## Default Values

None — all optional fields default to empty string (no time filtering).

## Output Invariants

- `NewErrors` contains only patterns where `BaseCount == 0 && TargetCount > 0`.
- `ResolvedErrors` contains only patterns where `BaseCount > 0 && TargetCount == 0`.
- `ChangedErrors` contains only patterns present in both sides, sorted by `|Change|` descending.
- `LevelChanges` includes every level seen in either side, sorted alphabetically.
- `NewSources` has `BaseCount == 0`. `DisappearedSources` has `TargetCount == 0`.
- `ChangedSources` has both counts > 0, sorted by `|Change|` descending.
- All slice fields are non-nil (empty slice, never null in JSON).
- `ChangedErrors` and `ChangedSources` are capped at 50 entries.

## Reused Components

| Component           | From                        | Purpose                              |
| ------------------- | --------------------------- | ------------------------------------ |
| `normalizeMessage`  | `log_extract_errors.go`         | Normalize error messages for clustering |
| `isErrorLevel`      | `log_extract_errors.go`         | Check if a level is ERROR/FATAL/CRITICAL |
| `parseTimestamp`    | `log_extract_errors.go`         | Parse timestamps in multiple formats |
| `fileutil.ReadLines`| `internal/fileutil`         | Paginated streaming file reader      |
| `parsers.AutoDetectWithHint` | `internal/parsers` | Format detection and parser selection |

## Error Responses

| Condition                         | Error Code         | Message                                     |
| --------------------------------- | ------------------ | ------------------------------------------- |
| `base_path` missing               | `INVALID_INPUT`    | `base_path is required`                     |
| File not found                    | `FILE_NOT_FOUND`   | via `CheckFileAccess`                       |
| Permission denied                 | `PERMISSION_DENIED`| via `CheckFileAccess`                       |
| Single-file mode missing ranges   | `INVALID_INPUT`    | `single-file mode requires all four time range parameters` |
| Overlapping time ranges           | `INVALID_INPUT`    | `time ranges must not overlap: base_before must be <= target_after` |
| Unparseable timestamp             | `INVALID_INPUT`    | `invalid timestamp for {param}: {value}`    |
| No parseable entries in base      | `NO_DATA`          | `no parseable log entries found in base`    |

## Testing Strategy

Table-driven tests in `log_diff_test.go` covering:

1. **Two different files** — base has errors A, B; target has errors B, C → new=[C], resolved=[A], changed=[B].
2. **Identical files** — all diff sections empty except level/source changes (which show zero deltas).
3. **Single-file time ranges** — two time periods with different error profiles.
4. **Rate change detection** — base has 10 lines/min, target has 100 lines/min.
5. **Source changes** — new source appears, old source disappears.
6. **Level distribution shift** — base mostly INFO, target mostly ERROR.
7. **Empty target** — all base errors become "resolved".
8. **Compressed file support** — .gz files work transparently.
9. **Error: missing file** — returns FILE_NOT_FOUND.
10. **Error: missing time ranges in single-file mode** — returns INVALID_INPUT.
11. **Error: overlapping time ranges** — returns INVALID_INPUT.
12. **Error: invalid timestamp** — returns INVALID_INPUT.