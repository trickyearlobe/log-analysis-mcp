# Plan: Project Scaffold & Types

## Goal

Initialise the Go module, create the directory structure, implement `internal/types/types.go` with tests.

## Specs to Read

- `specs/build_and_run.md` — module path, Go version, Makefile
- `specs/types.md` — all shared type definitions
- `specs/server_entry.md` — directory layout (cmd/, internal/)

## Steps

1. `go mod init github.com/trickyearlobe/log-analysis-mcp`
2. Create directory scaffold:
   - `cmd/log-analysis-mcp/`
   - `internal/types/`
   - `internal/server/`
   - `internal/tools/`
   - `internal/resources/`
   - `internal/prompts/`
   - `internal/parsers/`
   - `internal/fileutil/`
3. Create `Makefile` per `specs/build_and_run.md`
4. Implement `internal/types/types.go` per `specs/types.md`
5. Write `internal/types/types_test.go` — verify constants, JSON marshaling round-trips, nil-field omission
6. Run `go test -race ./internal/types/`
7. Run `go vet ./...`

## Acceptance Criteria

- `go build ./...` succeeds (even if main.go is a stub)
- `go test -race ./internal/types/` passes
- `go vet ./...` clean
- All types from `specs/types.md` present with correct struct tags
- JSON marshal/unmarshal round-trip tests pass for key types