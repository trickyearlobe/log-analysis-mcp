# Performance Requirements

---

## File Size Support

| File Size   | Strategy                                                           |
| ----------- | ------------------------------------------------------------------ |
| < 10 MB     | Full file operations supported for all tools                       |
| 10–100 MB   | Full file supported with streaming; pagination recommended         |
| 100 MB–1 GB | Streaming required; pagination enforced; sampling recommended      |
| > 1 GB      | Warning issued; operations restricted to streaming/paginated tools |

## Streaming Architecture

All file operations use line-by-line streaming. No file is ever read fully into memory.

See `specs/fileutil.md` for streaming, tail, and concurrency implementation patterns.

## Pagination Defaults

| Tool              | Default Page Size | Maximum Page Size |
| ----------------- | ----------------- | ----------------- |
| `log_read`       | 100 lines         | 1000 lines        |
| `log_search`     | 50 matches        | 500 matches       |
| `log_parse`      | 50 records        | 500 records       |
| `log_filter`     | 100 entries       | 1000 entries      |
| `log_tail`       | 50 lines          | 1000 lines        |
| `log_timeline`        | 100 events        | 500 events        |
| `log_extract_errors`  | 20 clusters       | 100 clusters      |
| `log_correlate`  | 50 groups         | 200 groups        |

## Output Size Limits

To avoid overwhelming the LLM context window, tool outputs are capped at approximately **100 KB** of JSON text. If a result would exceed this limit:

1. Truncate the results array to fit within the limit.
2. Set `truncated: true` in the response.
3. Include `total_available` count so the LLM knows how much data exists.
4. Suggest more specific filters or pagination in the truncation message.

## Statistical Operations

Statistical tools (`log_summarize`, `log_detect_anomalies`) use single-pass streaming algorithms. A single read through the file accumulates all required metrics — line counts, level-frequency counters, min/max timestamps, per-minute throughput buckets, and error-message frequencies. Frequency counters are pruned periodically if they exceed 10,000 entries to bound memory usage. Anomaly detection applies sliding-window rate analysis over the time-bucketed counts gathered during the same pass.

## `log_tail` Performance

Tail reading must be O(N) in the number of requested lines, not in total file size. The implementation seeks to the end of the file and reads backwards in chunks rather than scanning from the beginning.

## Concurrent Operations

For tools that process multiple files (e.g., `log_correlate`), files are read concurrently with support for cancellation.