# Pagination — Behavioural Specification

## Purpose

Allow callers to page through large result sets without re-scanning entire files. All paginated tools share a consistent interface.

## Scope

Affects tools: `log_search`, `log_filter`, `log_extract_errors`, `log_detect_anomalies`.

---

## Parameters (added to each tool's input struct)

| Field    | Type | Required | Default | Description                            |
|----------|------|----------|---------|----------------------------------------|
| `offset` | int  | no       | `0`     | Number of matching results to skip.    |

When `offset` is 0 (or absent), tools behave as before — return results starting from the first match.

---

## Output Fields (added to each tool's output struct)

| Field         | Type | Description                                                |
|---------------|------|------------------------------------------------------------|
| `has_more`    | bool | `true` if additional results exist beyond those returned.  |
| `next_offset` | int  | Offset to pass for the next page (`offset + len(results)`).|

### Semantics

- `has_more` is true when `total_matches > offset + len(results)`.
- `next_offset` = `offset + len(results)`. Only meaningful when `has_more` is true.
- Existing `truncated` field is **kept** for backward compatibility. Set `truncated = has_more` consistently.
- Existing `total_matches`/`total_matched` counts remain — they reflect the total in the file, not the page.
- With `offset > 0`, `truncated` derivation becomes `total > offset + len(results)`, NOT `total > len(results)`.

---

## What "offset" Counts (per tool)

| Tool                   | Offset counts           | Notes                                               |
|------------------------|------------------------|-----------------------------------------------------|
| `log_search`           | Matches                | Records when `record_separator` is active           |
| `log_filter`           | Matched entries        | Records when `record_separator` is active           |
| `log_extract_errors`   | Clusters (post-sort)   | Skip first N sorted clusters                        |
| `log_detect_anomalies` | Anomalies (post-sort)  | Skip first N sorted anomalies                       |

### Aggregate Tools: Page-Unit Totals

Aggregate tools must expose the total count **in pagination units** (not just raw counts):

- `log_extract_errors`: add `total_clusters int` (total unique clusters before pagination)
- `log_detect_anomalies`: add `total_anomalies int` (total anomalies detected before pagination)

These are distinct from `total_errors` (raw error count) and enable callers to compute page counts.

### log_detect_anomalies: Max Results

`log_detect_anomalies` must add a `max_results` parameter (default 50, max 200) to cap anomaly output. Without this, pagination has no page-size boundary.

### Stable Sort Requirement

All sorted output must have a total deterministic order to prevent pagination drift:
- `log_extract_errors`: sort by count/impact desc → pattern asc (already stable via pattern tie-break)
- `log_detect_anomalies`: sort by severity desc → start time asc → type asc → description asc

---

## Implementation Strategy

### log_search, log_filter (streaming tools)

These tools stream the file counting matches. With offset:

1. Stream as normal, counting all matches.
2. When a match is found, increment `totalMatches`.
3. Only append to results slice when `totalMatches > offset` AND `len(results) < maxResults`.
4. Continue streaming to EOF to get accurate `total_matches` count.

This is an O(n) scan regardless — offset doesn't enable seeking. The benefit is reduced output size and context tokens for the LLM.

### log_extract_errors, log_detect_anomalies (aggregate tools)

These tools must process the full file to build clusters/anomalies before sorting. With offset:

1. Process full file as before.
2. Sort results.
3. Apply `offset`: skip first N items from sorted slice.
4. Apply `max_clusters`/implicit max: take up to N items from remaining.
5. Set `has_more` / `next_offset` based on what remains.

---

## Validation

- `offset` < 0: clamp to 0.
- `offset` very large (beyond total results): return empty results with `has_more: false`, `next_offset: offset`.

---

## Struct Tag Format

```go
Offset int `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination"`
```

---

## Edge Cases

- **offset > total_matches:** Empty results, `has_more: false`.
- **offset = 0, max_results >= total:** All results returned, `has_more: false`.
- **record_separator active:** Offset counts records (not lines). No change to semantics.
- **File changes between pages:** No guarantees — results may shift. Document this as eventual consistency.
