# File Utilities — Behavioural Specification

This document specifies the required behaviour of the streaming file reader, tail reader, and concurrent file processing utilities. These utilities form the foundation upon which all tool implementations are built.

All descriptions are implementation-agnostic: they define **what** each utility must do, its inputs, outputs, invariants, and edge-case handling — not how to write it.

---

## 1. Streaming Line Reader

### Purpose

Read lines from a log file in a streaming fashion with pagination support. The reader must never load an entire file into memory, making it safe for arbitrarily large files (1 GB+).

### Inputs

| Parameter    | Type    | Required | Description                                      |
|-------------|---------|----------|--------------------------------------------------|
| `path`      | string  | yes      | Absolute or relative path to the log file.       |
| `start_line`| integer | no       | 1-based line number to begin reading from. Defaults to 1. |
| `num_lines` | integer | no       | Maximum number of lines to return. Defaults to 100. |

### Outputs

| Field        | Type            | Description                                              |
|-------------|-----------------|----------------------------------------------------------|
| `lines`     | list of records | Each record contains the 1-based `line_number` (integer) and the `text` content (string) of the line. |
| `has_more`  | boolean         | `true` if additional lines exist beyond the returned range. |
| `total_read`| integer         | Count of lines actually returned (may be less than `num_lines` if the file ends). |

### Behaviour

1. Open the file in read-only mode and create a buffered scanner with a **1 MB** maximum buffer size (both initial allocation and limit set to 1,048,576 bytes). This overrides any smaller default buffer the scanner may use.
2. Advance the scanner line by line, maintaining a 1-based line counter.
3. Skip all lines before `start_line` without storing them.
4. Beginning at `start_line`, collect each line's number and text content until `num_lines` lines have been collected or the file is exhausted.
5. After collecting, attempt one additional scan to determine whether more lines exist and set `has_more` accordingly.
6. Close the file handle before returning, regardless of success or failure.

### Edge Cases

- **File does not exist or is unreadable:** Return an error describing the path and the underlying OS error. Do not panic.
- **`start_line` beyond end of file:** Return an empty `lines` list with `has_more` set to `false`.
- **`start_line` < 1 or `num_lines` < 1:** Treat as invalid input; return a validation error.
- **Empty file:** Return an empty `lines` list with `has_more` set to `false` and `total_read` of 0.
- **Lines exceeding scanner max token size:** Fall back to the Large Line Fallback described in §2.

---

## 2. Large Line Fallback

### Purpose

Handle log files that contain lines exceeding the scanner's maximum token size. Some log formats (e.g., single-line JSON blobs, base64-encoded payloads) can produce lines well beyond 1 MB. When the primary scanner fails on such a line, the reader must degrade gracefully rather than returning an error or truncating silently.

### Behaviour

1. When the streaming line reader's scanner reports a "token too long" error (or equivalent), abandon the scanner.
2. Re-open or re-seek the file to the byte offset of the line that caused the failure.
3. Switch to an unbounded line-reading strategy that reads bytes until it encounters a `'\n'` delimiter (or EOF). This reader must not impose a maximum line length.
4. For each line read, strip trailing `'\r'` and `'\n'` characters before returning the text.
5. Continue reading subsequent lines with this fallback reader for the remainder of the current request. There is no requirement to switch back to the scanner within the same call.

### Edge Cases

- **Final line has no trailing newline:** The reader must still return it as a valid line.
- **File contains only one enormous line:** The fallback must handle this — return the single line with `line_number` 1 and `has_more` set to `false`.
- **Mixed line lengths:** It is acceptable to use the fallback for the entire file when any line triggers it, or to use it only from the failure point onward. Either strategy satisfies the spec as long as no lines are dropped or duplicated.

---

## 3. Tail Reader

### Purpose

Efficiently read the last N lines of a log file without scanning from the beginning. Performance must be **O(N)** in the number of requested lines, not O(file size).

### Inputs

| Parameter    | Type    | Required | Description                                    |
|-------------|---------|----------|------------------------------------------------|
| `path`      | string  | yes      | Path to the log file.                          |
| `num_lines` | integer | no       | Number of lines to return from the end. Defaults to 100. |

### Outputs

| Field        | Type            | Description                                              |
|-------------|-----------------|----------------------------------------------------------|
| `lines`     | list of records | Each record contains the 1-based `line_number` and `text`. Lines are in chronological order (earliest first). |
| `total_lines`| integer        | Total number of lines in the file (or a best-effort estimate if the file is very large). |
| `file_size` | integer         | Size of the file in bytes at the time of reading.        |

### Behaviour

1. Open the file in read-only mode and obtain its size via a stat call.
2. Beginning from the **end** of the file, seek backward in fixed-size chunks of **8,192 bytes** (8 KB).
3. In each chunk, count newline (`'\n'`) characters to track how many line boundaries have been found.
4. Continue seeking backward and reading chunks, prepending each chunk's data to an accumulating buffer, until at least `num_lines + 1` newline characters have been found (the extra one accounts for the boundary before the first requested line) or the beginning of the file is reached.
5. Split the accumulated data by newlines, discard any leading empty segment (artefact of splitting), and take the last `num_lines` entries.
6. Assign correct 1-based line numbers to each returned line. If the total line count of the file is known (because the reader reached the start of the file), use exact numbers. Otherwise, compute them relative to the estimated total.
7. Return lines in chronological (file) order, not reversed.

### Edge Cases

- **File is smaller than one chunk (< 8 KB):** Read the entire file in a single read from offset 0. The algorithm must not seek to a negative offset or fail.
- **File is empty:** Return an empty `lines` list, `total_lines` of 0, and `file_size` of 0.
- **`num_lines` exceeds total lines in file:** Return all lines in the file. Do not error.
- **File has no trailing newline:** The final line must still be included in the result.
- **File contains only newline characters:** Return `num_lines` empty-string lines (or fewer if the file has fewer lines).
- **Concurrent writes during read:** The reader uses the file size obtained at open time. If the file grows during reading, the reader is not required to pick up new data. Behaviour is undefined for files that shrink during reading.
- **Binary content or extremely long lines within a chunk:** The reader may return raw content; it does not need to validate encoding. Line splitting is purely on `'\n'` bytes.

---

## 4. Concurrent File Processing

### Purpose

Enable multi-file tools (e.g., `log_correlate`, multi-path `log_search`) to process several files in parallel while supporting cancellation and structured error propagation.

### Inputs

| Parameter | Type              | Required | Description                                          |
|-----------|-------------------|----------|------------------------------------------------------|
| `ctx`     | context           | yes      | A cancellation-aware context propagated from the MCP request handler. |
| `paths`   | list of strings   | yes      | Ordered list of file paths to process.               |
| `process` | function          | yes      | A per-file processing function that accepts a context and a file path, and returns results or an error. |

### Outputs

| Field     | Type              | Description                                              |
|-----------|-------------------|----------------------------------------------------------|
| `results` | ordered list      | One result entry per input path, in the **same order** as the input `paths` list, regardless of the order in which processing completes. |
| `error`   | error or nil      | The first error encountered by any file processor, or nil if all succeeded. |

### Behaviour

1. Create an error-group (or equivalent structured concurrency primitive) bound to the supplied context. This ensures that if any one file processor returns an error, the context is cancelled and all other in-flight processors are signalled to stop.
2. Launch one concurrent task per file path. Each task:
   - Receives the shared context and must check for cancellation at reasonable intervals (e.g., between processing chunks or lines).
   - Calls the per-file `process` function.
   - Writes its result into the correct position in the results list (indexed by input order).
3. Wait for all tasks to complete or for the first error.
4. If an error occurred, return it alongside any partial results that completed before cancellation. The caller decides whether to use partial results.
5. If all tasks succeed, return the complete ordered results list with a nil error.

### Invariants

- **Ordering:** Results must correspond positionally to the input paths. Parallel execution must not scramble the order.
- **No goroutine leaks:** All spawned concurrent tasks must terminate before the call returns, whether on success, error, or context cancellation.
- **Error precedence:** If multiple file processors fail, only the first error needs to be reported. Others may be suppressed.
- **Resource cleanup:** Each concurrent task is responsible for closing any file handles or resources it opens, even if cancelled mid-operation.

### Edge Cases

- **Empty paths list:** Return an empty results list with no error. Do not spawn any concurrent tasks.
- **Single path:** The implementation may still use the concurrent machinery, or may short-circuit to a direct call. Either is acceptable as long as the output contract is met.
- **Context already cancelled before invocation:** Return immediately with a context cancellation error and an empty results list.
- **All files fail:** Return the first error. The results list may contain zero or more partial entries depending on timing.

---

## General Invariants (All Utilities)

1. **Read-only access:** No utility may modify, truncate, rename, or delete any file. All file handles must be opened in read-only mode.
2. **No full-file buffering:** At no point may an entire file's contents reside in memory. Streaming and chunked reading are mandatory.
3. **Errors as values:** Utilities return errors to their callers rather than panicking or terminating the process. The tool layer is responsible for formatting these errors as MCP error content.
4. **Path transparency:** Utilities accept whatever path the tool layer provides (absolute or relative) and do not impose restrictions on directory structure or file extensions.
5. **Encoding agnosticism:** Utilities operate on raw bytes split by `'\n'`. They do not validate or transcode character encodings. Line text is returned as-is.