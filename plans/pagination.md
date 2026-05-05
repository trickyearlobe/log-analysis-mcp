# Plan: Pagination

## Goal

Add `offset`/`has_more`/`next_offset` pagination to `log_search`, `log_filter`, `log_extract_errors`, and `log_detect_anomalies`.

## Specs to Read

- `specs/pagination.md` (just written)
- `specs/tools/log_search.md`, `log_filter.md`, `log_extract_errors.md`, `log_detect_anomalies.md`

## Steps

1. Add `Offset` field to all 4 input structs
2. Add `HasMore` + `NextOffset` fields to all 4 output structs
3. Update `log_search` — streaming offset logic (line mode + record mode)
4. Update `log_filter` — streaming offset logic (line mode + record mode)
5. Update `log_extract_errors` — post-sort slice offset
6. Update `log_detect_anomalies` — post-sort slice offset
7. Write/update tests for each tool
8. Run `go test -race ./...` + `go vet ./...`
9. Update tool specs with new parameters
10. Commit

## Acceptance

- All existing tests pass
- New pagination tests pass for each tool
- `offset=0` produces identical results to current behaviour
- `offset=N` skips first N matches/clusters/anomalies
- `has_more` correctly indicates remaining results
- `next_offset` equals `offset + len(results)`
