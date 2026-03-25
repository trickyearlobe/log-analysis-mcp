# Tool: `decompress_file`

**Description (shown to LLM):**
> Decompress a compressed log file (.gz, .bz2, .zip) to a temporary plain-text
> file on disk. Use this before running multiple tools on the same compressed
> file — it pays the decompression cost once, then all subsequent tools get
> full seekable performance. For a single tool call, you can pass the compressed
> path directly — all tools handle decompression transparently.

---

## Input Schema

| Parameter | Type     | Required | Default | Description                          |
|-----------|----------|----------|---------|--------------------------------------|
| `path`    | `string` | Yes      | —       | Path to the compressed log file      |

### Go Input Struct

```go
type DecompressFileInput struct {
    Path string `json:"path" jsonschema:"Path to the compressed log file (.gz, .bz2, .zip)"`
}
```

---

## Output Format

### Go Output Struct

```go
type DecompressFileOutput struct {
    TempPath         string `json:"temp_path"`
    OriginalPath     string `json:"original_path"`
    CompressedSize   int64  `json:"compressed_size"`
    DecompressedSize int64  `json:"decompressed_size"`
    Note             string `json:"note"`
}
```

### JSON Example

```json
{
  "temp_path": "/tmp/log-analysis-syslog.log-a1b2c3d4",
  "original_path": "/var/log/syslog.gz",
  "compressed_size": 4521984,
  "decompressed_size": 38912000,
  "note": "Temporary file — use this path with other tools. File will be cleaned up when the server exits."
}
```

---

## Handler Signature

```go
func handleDecompressFile(ctx context.Context, req *mcp.CallToolRequest, input DecompressFileInput) (*mcp.CallToolResult, DecompressFileOutput, error)
```

---

## Registration

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "decompress_file",
    Description: "Decompress a compressed log file (.gz, .bz2, .zip) to a temporary plain-text file on disk. Use this before running multiple tools on the same compressed file — it pays the decompression cost once, then all subsequent tools get full seekable performance. For a single tool call, you can pass the compressed path directly — all tools handle decompression transparently.",
}, handleDecompressFile)
```

---

## Behaviour

1. Validate the file exists, is readable, and is not a directory (reuse
   `toolutil.CheckFileAccess` minus the binary check).
2. Check that the file has a recognised compressed extension (`.gz`, `.bz2`,
   `.zip`). If not, return a tool error:
   `"File does not have a recognised compressed extension (.gz, .bz2, .zip). Pass the path directly to other tools — no decompression needed."`
3. Call `fileutil.OpenReader(path)` to obtain a decompressed `io.ReadCloser`
   and the compressed file size.
4. Create a temporary file in `os.TempDir()` with a name pattern of
   `log-analysis-<original-basename>-*` (the `*` is filled by `os.CreateTemp`).
5. Stream the decompressed content to the temporary file using `io.Copy`.
   Do NOT accumulate the entire decompressed content in memory.
6. Close the decompressed reader and the temporary file.
7. Stat the temporary file to obtain `decompressed_size`.
8. Register the temporary file path in the process-level cleanup registry.
9. Return the output struct.

---

## Temp File Lifecycle

### Registry

A package-level registry in `internal/tools/decompress_file.go`:

```go
var (
    tempFilesMu sync.Mutex
    tempFiles   []string
)
```

Each successful decompression appends to `tempFiles`.

### Cleanup

An exported `CleanupTempFiles()` function iterates the registry, removes each
file with `os.Remove`, and clears the slice. This is called from `main.go`
during shutdown (after context cancellation or signal).

Errors during removal are logged via `slog.Error` but do not prevent other
files from being cleaned up.

### Crash Safety

If the server crashes, temp files remain on disk in `os.TempDir()`. This is
acceptable — the OS temp directory is periodically cleaned by the system.
The `log-analysis-` prefix makes them identifiable for manual cleanup.

---

## Error Handling

| Condition                            | Error Code       | Message                                                                                     |
|--------------------------------------|------------------|---------------------------------------------------------------------------------------------|
| File does not exist                  | `FILE_NOT_FOUND` | `File not found: /path/to/file — verify the path is correct and accessible`                 |
| File is not a compressed format      | `INVALID_INPUT`  | `File does not have a recognised compressed extension (.gz, .bz2, .zip). Pass the path directly to other tools — no decompression needed.` |
| File is a directory                  | `INVALID_INPUT`  | `Path is a directory, not a file: /path/to/dir`                                             |
| Decompression fails mid-stream       | `DECOMPRESS_ERROR` | `Decompression failed for /path/to/file.gz: <underlying error>`                           |
| Disk full during write               | `DECOMPRESS_ERROR` | `Failed to write decompressed data: <underlying error>`                                   |
| Empty zip archive                    | `DECOMPRESS_ERROR` | `Zip archive contains no entries: /path/to/file.zip`                                      |

On any error after the temp file has been created, the partial temp file is
removed before returning.

---

## Usage Scenario

An AI assistant is asked to investigate errors in `/var/log/app.log.gz`. It
recognises this will require `summarize_logs`, `extract_errors`, `search_logs`,
and `detect_anomalies` — four decompression passes on the same file. Instead,
it first calls `decompress_file` to get a temp path, then passes that path to
all four tools. The result is one decompression pass plus four fast seekable
reads, rather than four full decompression passes.

For a simple "show me the last 20 lines of app.log.gz", the assistant skips
`decompress_file` and calls `tail_logs` directly — transparent decompression
handles it in a single pass.