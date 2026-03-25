# Plan: fileutil package

## Goal

Implement `internal/fileutil/` — streaming line reader, large-line fallback, tail reader, and concurrent file processor per `specs/fileutil.md`.

## Specs to read

- `specs/fileutil.md` (primary — already read)
- `specs/error_handling.md` (error codes, binary detection — already read)
- `specs/performance.md` (streaming constraints, tail O(N) — already read)
- `specs/types.md` / `internal/types/types.go` (shared types — already read)

## Ordered steps

1. **Create `internal/fileutil/reader.go`** — Streaming line reader (§1) with large-line fallback (§2).
   - `LineRecord` struct: `LineNumber int`, `Text string`.
   - `ReadLinesResult` struct: `Lines []LineRecord`, `HasMore bool`, `TotalRead int`.
   - `ReadLines(path string, startLine, numLines int) (ReadLinesResult, error)` function.
   - 1 MB scanner buffer. On token-too-long, switch to unbounded `bufio.Reader` fallback.
   - Validate startLine >= 1, numLines >= 1.
   - Write test first: `internal/fileutil/reader_test.go`.

2. **Create `internal/fileutil/tail.go`** — Tail reader (§3).
   - `TailResult` struct: `Lines []LineRecord`, `TotalLines int`, `FileSize int64`.
   - `TailLines(path string, numLines int) (TailResult, error)` function.
   - Seek-from-end in 8 KB chunks. O(N) in requested lines, not file size.
   - Handle: empty file, file < 1 chunk, no trailing newline, numLines > total.
   - Write test first: `internal/fileutil/tail_test.go`.

3. **Create `internal/fileutil/concurrent.go`** — Concurrent file processor (§4).
   - Generic with type parameter: `ProcessFiles[T any](ctx context.Context, paths []string, process func(ctx context.Context, path string) (T, error)) ([]T, error)`.
   - `errgroup` from stdlib (`sync/errgroup` — no, that's `golang.org/x/sync`). Use manual goroutine + `sync.WaitGroup` + context cancellation instead since no external deps allowed.
   - Preserve input order. No goroutine leaks. First-error-wins.
   - Write test first: `internal/fileutil/concurrent_test.go`.

4. **Create `internal/fileutil/binary.go`** — Binary file detection helper.
   - `CheckBinary(path string) error` — reads first 8192 bytes, returns error with `BINARY_FILE` code if null byte found.
   - Write test first: `internal/fileutil/binary_test.go`.

5. **Run `go test -race ./internal/fileutil/` and `go vet ./...`**.

6. **Commit**: `fileutil: streaming reader, tail, concurrent processor, binary check`.

7. **Delete this plan**.

## Acceptance criteria

- [ ] `ReadLines` streams without full-file buffering, handles large lines via fallback.
- [ ] `TailLines` is O(N) — seeks from end, reads backward in 8 KB chunks.
- [ ] `ProcessFiles` runs concurrently, preserves order, cancels on first error, no goroutine leaks.
- [ ] `CheckBinary` detects null bytes in first 8192 bytes.
- [ ] All edge cases from spec covered by table-driven tests.
- [ ] `go test -race ./internal/fileutil/` passes.
- [ ] `go vet ./...` clean.
- [ ] No external dependencies added.
- [ ] No file loaded fully into memory.
- [ ] No panics — all errors returned.
- [ ] No writes to stdout.