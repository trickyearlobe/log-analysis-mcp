# Plan: Compressed File Support

## Goal

All existing tools transparently handle `.gz`, `.bz2`, and `.zip` log files
via a new `OpenReader` abstraction in `fileutil`. A new `decompress_file` tool
lets the LLM extract compressed files to temp files for faster multi-tool
workflows (pay the decompression cost once, then use seekable reads).

## Specs to Read

- `specs/compression.md` — full behavioural spec (just written)
- `specs/fileutil.md` — existing streaming reader, tail, binary contracts
- `specs/performance.md` — streaming architecture, no full-file buffering

## Steps

1. **Write `internal/fileutil/compression.go`** — `OpenReader` function + helpers:
   - Detect compression by case-insensitive file extension
   - `.gz` → `compress/gzip`
   - `.bz2` → `compress/bzip2`
   - `.zip` → `archive/zip` (first entry)
   - Return `io.ReadCloser` + `int64` size + `error`
   - Custom closer types to ensure underlying file handles are released

2. **Write `internal/fileutil/compression_test.go`** — TDD, table-driven:
   - Plain file passthrough
   - `.gz` round-trip (create with `gzip.NewWriter`, read back with `OpenReader`)
   - `.bz2` round-trip (create with `bzip2` writer helper, read back)
   - `.zip` with one entry, multiple entries (reads first), zero entries (error)
   - Corrupt `.gz` (Open succeeds, Read returns error)
   - Non-existent file → OS error
   - Case-insensitive extension (`.GZ`, `.Bz2`)
   - Close releases file descriptor (verify with re-open or stat)
   - Run: `go test -race ./internal/fileutil/`

3. **Modify `internal/fileutil/reader.go`** — `ReadLines` uses `OpenReader`:
   - Replace `os.Open(path)` with `OpenReader(path)` in main path
   - Replace `os.Open(path)` with `OpenReader(path)` in `readLinesWithFallback`
   - No interface changes — `ReadLinesResult` contract unchanged
   - Run: `go test -race ./internal/fileutil/`

4. **Modify `internal/fileutil/tail.go`** — compressed fallback in `TailLines`:
   - Check extension with the same detection helper used by `OpenReader`
   - Plain files: existing backward-seek algorithm, unchanged
   - Compressed files: stream with `OpenReader` → `bufio.Scanner` → ring buffer
   - Reuse `maxScannerBuf` constant and large-line fallback from reader
   - `TotalLines` is exact for compressed (full stream read)
   - `FileSize` is compressed size on disk (from stat)
   - Run: `go test -race ./internal/fileutil/`

5. **Modify `internal/fileutil/binary.go`** — `CheckBinary` uses `OpenReader`:
   - Replace `os.Open(path)` with `OpenReader(path)`
   - Binary check now runs on decompressed bytes (correct behaviour)
   - Run: `go test -race ./internal/fileutil/`

6. **Add compressed file tests to existing test files**:
   - `reader_test.go`: `ReadLines` on `.gz` and `.bz2` files, pagination
   - `tail_test.go`: `TailLines` on `.gz` file, verify exact total lines
   - `binary_test.go`: `CheckBinary` on `.gz` wrapping text vs binary content
   - Run: `go test -race ./internal/fileutil/`

7. **Write `internal/tools/decompress_file.go`** — new `decompress_file` tool:
   - `DecompressFileInput` with `Path` field
   - `DecompressFileOutput` with `TempPath`, `OriginalPath`, `CompressedSize`,
     `DecompressedSize`, `Note` fields
   - `RunDecompressFile` function:
     - Validate file exists, is readable, is a recognised compressed format
     - Call `OpenReader` → stream to temp file via `io.Copy` (32 KB buffer)
     - Track temp file in process-level registry for cleanup
     - Return error if not compressed (tell LLM to use path directly)
   - Process-level temp file registry: `sync.Mutex`-protected slice, cleanup
     function called on server shutdown
   - Run: `go test -race ./internal/tools/`

8. **Write `internal/tools/decompress_file_test.go`** — TDD, table-driven:
   - Decompress `.gz` file → verify temp file content matches original
   - Decompress `.bz2` file → verify content
   - Decompress `.zip` file → verify content
   - Non-compressed file → error with guidance message
   - File not found → `FILE_NOT_FOUND` error
   - Corrupt `.gz` → error, partial temp file cleaned up
   - Verify temp file appears in cleanup registry
   - Run: `go test -race ./internal/tools/`

9. **Register `decompress_file` in `internal/server/server.go`**

10. **Full regression test**:
   - `go test -race ./...` — all 668+ existing tests must pass
   - `go vet ./...` — clean

11. **Integration test** (optional, if time permits):
   - Add compressed file tests to `internal/integration/` exercising
     `read_logs` and `tail_logs` through the full MCP round-trip
   - Add `decompress_file` round-trip test

## Acceptance Criteria

- `OpenReader` correctly decompresses `.gz`, `.bz2`, `.zip` files
- `ReadLines` on a compressed file returns correct lines with correct numbering
- `TailLines` on a compressed file returns the last N lines (streaming fallback)
- `CheckBinary` on a compressed text file returns nil
- `CheckBinary` on a compressed binary file returns error
- `decompress_file` extracts to temp file, returns correct sizes and path
- `decompress_file` on a non-compressed file returns a clear error
- `decompress_file` cleans up partial temp file on decompression failure
- Temp files are tracked and cleaned up on server shutdown
- All existing tests pass unchanged (no interface breakage)
- `go test -race ./...` passes
- `go vet ./...` clean
- No full-file buffering — streaming maintained
- No external dependencies — stdlib only

## Key Design Notes

- `OpenReader` returns `(io.ReadCloser, int64, error)` not `(*os.File, error)`
  so callers cannot assume seekability
- `TailLines` is the only function that needs branching logic (seek vs stream)
- The ring buffer in `TailLines` compressed path is bounded by `numLines`
- `.zip` handling uses `zip.OpenReader` which needs the full path (not streaming),
  but we only stream the decompressed content of the first entry
- `bzip2` stdlib package provides `NewReader` (returns `io.Reader`) but no writer;
  tests create `.bz2` files using `dsnet/compress/bzip2` or by embedding a
  pre-compressed byte literal — check what's feasible with stdlib only
- `decompress_file` streams to disk with `io.Copy` — never buffers the whole
  decompressed content in memory
- Temp file registry is a package-level `sync.Mutex`-protected slice in
  `internal/tools/decompress_file.go`; exported `CleanupTempFiles()` called
  from `main.go` shutdown path
- The LLM chooses when to decompress — single tool calls use transparent
  `OpenReader`, multi-tool workflows use `decompress_file` then plain paths