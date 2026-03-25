# Compressed File Support — Behavioural Specification

Transparent decompression of compressed log files. All existing tools work with
compressed files without changes to their code. A `decompress_file` tool allows
the LLM to extract a compressed file to a temporary plain file for faster
multi-tool workflows.

---

## Supported Formats

| Extension | Stdlib package     | Seekable | Notes                              |
|-----------|--------------------|----------|------------------------------------|
| `.gz`     | `compress/gzip`    | No       | Most common rotated log format     |
| `.bz2`    | `compress/bzip2`   | No       | Less common, still seen in legacy  |
| `.zip`    | `archive/zip`      | No*      | Read first entry only by default   |

*`archive/zip` requires `io.ReaderAt` (the whole file), but we stream the
decompressed content of a single entry, so downstream sees a non-seekable stream.

No external dependencies. All decompressors are in the Go standard library.

---

## Detection

Compression is detected by **file extension only**, not magic bytes.

| Path                        | Detected as |
|-----------------------------|-------------|
| `/var/log/syslog.gz`        | gzip        |
| `/var/log/app.log.bz2`     | bzip2       |
| `/var/log/archive.zip`     | zip         |
| `/var/log/syslog`          | plain       |
| `/var/log/syslog.zst`      | unsupported |

Extension matching is case-insensitive (`.GZ`, `.Gz` both match).

Unsupported compression extensions are treated as plain files — no error, no
special handling. The content will likely fail to parse, which surfaces naturally.

---

## Core Abstraction: `OpenReader`

A new exported function in `internal/fileutil`:

### Inputs

| Parameter | Type   | Required | Description                        |
|-----------|--------|----------|------------------------------------|
| `path`    | string | yes      | Path to the file (plain or compressed) |

### Outputs

| Field    | Type           | Description                                        |
|----------|----------------|----------------------------------------------------|
| `reader` | `io.ReadCloser` | Decompressed byte stream, or raw file if plain     |
| `size`   | `int64`        | Original compressed file size in bytes (from stat)  |
| `err`    | `error`        | Non-nil on open/decompressor failure               |

### Behaviour

1. Open the file read-only with `os.Open`.
2. Stat the file to obtain `size`.
3. Inspect the file extension:
   - `.gz`: wrap in `gzip.NewReader`. Close must close both the gzip reader and
     the underlying file.
   - `.bz2`: wrap in `bzip2.NewReader`. Close must close the underlying file.
     (`compress/bzip2` returns an `io.Reader`, not an `io.ReadCloser`.)
   - `.zip`: open with `zip.OpenReader`, locate the first entry, call `Open()`
     on it. Close must close the zip reader. If the archive is empty, return
     an error.
   - Any other extension: return the raw `*os.File` as the `io.ReadCloser`.
4. The returned `io.ReadCloser` yields decompressed bytes. Callers read lines
   from it exactly as they would from a plain file.

### Edge Cases

- **File does not exist or is unreadable:** Return the OS error. Do not panic.
- **Corrupt compressed data:** The decompressor will return an error on `Read`.
  This propagates naturally through `bufio.Scanner` or `bufio.Reader`.
- **Empty `.zip` archive:** Return an error describing that no entries were found.
- **`.zip` with multiple entries:** Use the first entry. Document this behaviour.
  A future enhancement could accept an entry name parameter.
- **Nested compression (`.tar.gz`):** Not supported. The `.gz` layer is
  decompressed, revealing tar bytes. Tools will see binary-looking content and
  `CheckBinary` will reject it. This is acceptable.

---

## Impact on `ReadLines`

`ReadLines` currently opens files with `os.Open` and creates a `bufio.Scanner`.

### Change

Replace `os.Open(path)` with `OpenReader(path)`. The returned `io.ReadCloser`
is wrapped in `bufio.Scanner` exactly as before. The large-line fallback path
(`readLinesWithFallback`) also switches to `OpenReader`.

The `ReadLinesResult` contract is unchanged. Callers see no difference.

### Line Numbering

Line numbers remain 1-based and sequential within the decompressed stream.
There is no concept of "byte offset in the compressed file" — lines are
numbered as they appear in the decompressed output.

---

## Impact on `TailLines`

`TailLines` currently uses `os.Open` + `f.ReadAt` for O(N) backward seeking.
Compressed streams are not seekable.

### Change

1. `TailLines` checks the file extension.
2. **Plain files:** Existing backward-seek algorithm, unchanged.
3. **Compressed files:** Fall back to a streaming approach:
   - Call `OpenReader(path)` to get the decompressed stream.
   - Read lines sequentially using `bufio.Scanner` (with the same 1 MB buffer
     and large-line fallback as `ReadLines`).
   - Maintain a ring buffer of the last `numLines` lines.
   - After EOF, return the ring buffer contents.
   - `TotalLines` is exact (the entire stream was read).
   - `FileSize` is the compressed file size on disk (from stat).

### Performance Tradeoff

For compressed files, `TailLines` is O(file size), not O(N). This is unavoidable
without a seekable decompression format. The tradeoff is documented here and
acceptable for single operations because:
- Compressed log files are typically older/archived, not multi-GB active files.
- Users who need fast tail access should use uncompressed files.

For multi-tool workflows (investigate an error, health check, etc.), the LLM
should call `decompress_file` first, then pass the temporary plain file path
to all subsequent tools. This avoids repeated decompression and restores O(N)
`TailLines` performance. See the `decompress_file` tool section below.

---

## Impact on `CheckBinary`

`CheckBinary` currently reads the first 8192 raw bytes with `os.Open` + `f.Read`.

### Change

Replace `os.Open` with `OpenReader`. The binary check runs on decompressed bytes,
not on the raw compressed stream. This is correct because:
- Raw gzip/bzip2 bytes contain `0x00` and would always be flagged as binary.
- The purpose is to detect binary *log content*, which requires decompression.

---

## Impact on `ProcessFiles`

No change. `ProcessFiles` delegates all I/O to caller-supplied functions. Those
functions call `ReadLines` or `TailLines`, which now handle compression internally.

---

## Impact on Tools

**No changes to any tool code.** All tools call `fileutil.ReadLines`,
`fileutil.TailLines`, or `fileutil.CheckBinary`. Since those functions now
handle compression transparently, every tool gains compressed file support for
free.

Tools that call `os.Stat` directly (e.g. `CheckFileAccess`, `FileSize` in
`toolutil.go`) continue to work — stat operates on the compressed file, which
is correct for existence checks and reporting file sizes.

---

## Impact on Resources

The `log:///{+path}` resource handler calls `fileutil.ReadLines` for the preview
and `os.Stat` for metadata. Both work correctly:
- `ReadLines` now decompresses transparently.
- `os.Stat` reports the compressed file size, which is the on-disk size.
- `detected_format` in resource metadata comes from parsing the first lines of
  the decompressed stream, so format detection works correctly.

---

## Testing Strategy

### Unit Tests (`compression_test.go`)

Test `OpenReader` directly:

| Test case                          | Expectation                              |
|------------------------------------|------------------------------------------|
| Plain text file                    | Returns raw file content                 |
| `.gz` file with known content      | Returns decompressed content             |
| `.bz2` file with known content     | Returns decompressed content             |
| `.zip` with one entry              | Returns decompressed first entry         |
| `.zip` with multiple entries       | Returns decompressed first entry         |
| `.zip` with zero entries           | Returns error                            |
| Corrupt `.gz` file                 | Read returns error (not Open)            |
| Non-existent file                  | Returns OS error                         |
| `.GZ` uppercase extension          | Detected as gzip                         |
| `.tar.gz` file                     | Decompresses gzip layer (tar bytes)      |
| Close releases underlying file     | No leaked file descriptors               |

### Integration with `ReadLines` and `TailLines`

| Test case                          | Expectation                              |
|------------------------------------|------------------------------------------|
| `ReadLines` on `.gz` file          | Correct lines and line numbers           |
| `ReadLines` pagination on `.bz2`   | `start_line` and `has_more` work         |
| `TailLines` on `.gz` file          | Last N lines correct, `TotalLines` exact |
| `TailLines` on `.bz2` file         | Last N lines correct                     |
| `CheckBinary` on `.gz` text file   | Returns nil (text content)               |
| `CheckBinary` on `.gz` binary file | Returns error (binary content)           |

### Regression

All 668+ existing tests must continue to pass. No test should need modification
because the interface contracts are unchanged.

---

## File Layout

```
internal/fileutil/
├── compression.go       # OpenReader + helper types
├── compression_test.go  # Unit tests for OpenReader
├── reader.go            # ReadLines (modified to use OpenReader)
├── reader_test.go       # Existing tests (unchanged)
├── tail.go              # TailLines (modified: compressed fallback)
├── tail_test.go         # Existing tests + new compressed tests
├── binary.go            # CheckBinary (modified to use OpenReader)
├── binary_test.go       # Existing tests + new compressed tests
├── concurrent.go        # Unchanged
└── concurrent_test.go   # Unchanged
```

---

## Invariants

1. **No full-file buffering.** Decompressed streams are read line-by-line, never
   accumulated in memory (except the ring buffer in `TailLines`, bounded by
   `numLines`).
2. **Read-only access.** Compressed files are never modified.
3. **Errors as values.** Decompression errors propagate as return values, never panics.
4. **Close always cleans up.** Every `OpenReader` result must be closed, and closing
   must release all underlying resources (file handles, decompressor state).
5. **Extension detection is case-insensitive.**
6. **Plain files have zero overhead.** When the extension is not recognised as a
   compressed format, `OpenReader` returns the raw `*os.File` with no wrapping.

---

## `decompress_file` Tool

### Purpose

Decompress a compressed log file to a temporary plain-text file on disk. This
lets the LLM pay the decompression cost once and then use the temporary file
path with all other tools, getting full seekable performance (O(N) tail, no
repeated decompression per tool call).

The LLM decides when to use this — it is never automatic. For a single
`read_logs` call on a `.gz` file, transparent `OpenReader` is fine. For a
multi-step investigation, the LLM should decompress first.

### Inputs

| Parameter | Type   | Required | Description                                    |
|-----------|--------|----------|------------------------------------------------|
| `path`    | string | yes      | Path to the compressed log file                |

### Outputs

| Field              | Type    | Description                                     |
|--------------------|---------|-------------------------------------------------|
| `temp_path`        | string  | Absolute path to the decompressed temporary file |
| `original_path`    | string  | The input path, echoed back for reference        |
| `compressed_size`  | int64   | Size of the original compressed file in bytes    |
| `decompressed_size`| int64   | Size of the decompressed temporary file in bytes |
| `note`             | string  | Reminder that the file is temporary              |

### Behaviour

1. Validate the file exists, is readable, and is not a directory.
2. If the file is not a recognised compressed format, return an error — the
   caller should use the original path directly.
3. Call `OpenReader(path)` to get the decompressed stream.
4. Create a temporary file in `os.TempDir()` with a name derived from the
   original filename (e.g. `log-analysis-<original-basename>-<random>`).
5. Stream the decompressed content to the temporary file using `io.Copy` with
   a bounded buffer (e.g. 32 KB). NEVER accumulate the whole decompressed
   content in memory.
6. Close both the decompressed reader and the temporary file.
7. Stat the temporary file to get `decompressed_size`.
8. Return the result with `note` set to:
   `"Temporary file — use this path with other tools. File will be cleaned up when the server exits."`

### Cleanup

Temporary files are tracked in a process-level registry (slice protected by a
mutex). On server shutdown (context cancellation or signal), all tracked temp
files are removed. Individual temp files can also be removed explicitly if a
`cleanup_temp_file` tool is added later.

### Edge Cases

- **File is not compressed:** Return error with code `INVALID_INPUT`:
  `"File does not have a recognised compressed extension (.gz, .bz2, .zip). Use the path directly with other tools."`
- **File does not exist:** Return `FILE_NOT_FOUND` error.
- **Decompression fails mid-stream:** Remove the partial temp file, return error.
- **Disk full during decompression:** The `io.Copy` will fail; remove partial
  temp file and return the OS error.
- **File is binary after decompression:** Do not check — let the downstream
  tool's `CheckBinary` catch it. The purpose of this tool is only decompression.
- **Called on an already-plain file by mistake:** Return a clear error, not a
  copy of the file.

### Tool Registration

Registered as `decompress_file` in `internal/tools/decompress_file.go`.
One file, one test file beside it, same as all other tools.

### Server Prompt Guidance

The server's `Instructions` field (sent to clients during MCP initialisation)
should mention that compressed files work transparently but that
`decompress_file` is recommended before multi-tool workflows on large
compressed files. This guides the LLM to make the optimisation decision
without requiring it.