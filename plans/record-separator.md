# Plan: Record Separator Implementation

## Goal
Add `record_separator` regex parameter that groups multi-line log entries into single records.

## Steps

1. Implement `fileutil.RecordScanner` in `internal/fileutil/record.go` (TDD)
2. Write tests for edge cases: empty file, no match, every-line match, truncation, compression
3. Add `record_separator` to `log_extract_errors` input + integrate RecordScanner
4. Add `record_separator` to `log_filter` input + integrate
5. Add `record_separator` to `log_search` input + integrate
6. Add `record_separator` to `log_summarize` input + integrate
7. Run full test suite, vet, commit

## Acceptance Criteria
- Tools with `record_separator=""` behave identically to before
- Java stack trace grouped into single record with `^\d{4}-\d{2}-\d{2}` separator
- Records exceeding 500 lines or 64KB are truncated safely
- Invalid regex returns error, not panic
