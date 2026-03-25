# Plan: parsers package

## Goal

Implement `internal/parsers/` — Parser interface, JSON/syslog/Apache parsers, auto-detection engine, and multiline combiner per `specs/parsers.md`.

## Specs to read

- `specs/parsers.md` (primary — already read)
- `specs/types.md` / `internal/types/types.go` (ParsedLogEntry, FormatDetectionResult, LogLevel, LogFormat — already read)

## Ordered steps

1. **Create `internal/parsers/parser.go`** — Parser interface definition.
   - `Parse(line string) *types.ParsedLogEntry`
   - `Detect(lines []string) float64`
   - `Name() string`

2. **Create `internal/parsers/jsonlog.go` + `jsonlog_test.go`** — JSON log parser.
   - Field normalization table (timestamp, level, message, source variants).
   - Level normalization table (trace/debug/info/warn/error/fatal + numeric).
   - Extra fields for anything not in standard set.
   - Returns nil for non-JSON lines.

3. **Create `internal/parsers/syslog.go` + `syslog_test.go`** — Syslog parser (RFC 3164 + 5424).
   - Two compiled RE2 regexes (init-time).
   - Priority → facility + severity derivation.
   - Severity → LogLevel mapping.
   - Detect picks whichever RFC scores higher, names accordingly.

4. **Create `internal/parsers/apache.go` + `apache_test.go`** — Apache/Nginx parser.
   - Combined + Common log format RE2 regexes (init-time).
   - HTTP status → LogLevel mapping.
   - Detect tries combined first, falls back to common.

5. **Create `internal/parsers/autodetect.go` + `autodetect_test.go`** — Auto-detection engine.
   - Register parsers in priority order: JSON > RFC5424 > RFC3164 > Combined > Common.
   - Score each, pick highest ≥ 0.5, tiebreak by priority.
   - `AutoDetect(lines []string) types.FormatDetectionResult`.

6. **Create `internal/parsers/multiline.go` + `multiline_test.go`** — Multiline combiner.
   - Compiled RE2 patterns: javaStack, causedBy, pythonTB, dotnetStack, continuation.
   - Sequential processing: aggregate continuation lines into stack_trace.
   - `line_count` field on combined entries.

7. **Run `go test -race ./internal/parsers/` and `go vet ./...`**.

8. **Commit**: `parsers: json, syslog, apache, autodetect, multiline`.

9. **Delete this plan**.

## Acceptance criteria

- [ ] Parser interface defined; all parsers implement it.
- [ ] JSON parser normalizes fields + levels, handles extra_fields, rejects non-JSON.
- [ ] Syslog parser handles RFC 3164 and 5424, maps severity to LogLevel.
- [ ] Apache parser handles Combined and Common, maps status to LogLevel.
- [ ] AutoDetect scores all parsers, returns best ≥ 0.5, tiebreaks by priority.
- [ ] Multiline combiner detects Java/Python/.NET stack traces, sets line_count.
- [ ] All regexes are RE2-compatible and compiled at init time.
- [ ] Table-driven tests for each parser with real-world log samples.
- [ ] `go test -race ./internal/parsers/` passes.
- [ ] `go vet ./...` clean.
- [ ] No external dependencies added.
- [ ] No panics — all errors returned or nil.
- [ ] No writes to stdout.