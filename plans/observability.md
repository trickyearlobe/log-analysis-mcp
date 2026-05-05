# Plan: Observability

## Goal

Add performance and efficiency measurement to the MCP server using SDK middleware,
in-memory metrics, and an MCP resource for live introspection.

## Specs to Read

- `specs/observability.md` (this feature)
- `specs/build_and_run.md` (flag handling)
- `specs/resources_and_prompts.md` (resource registration)

## Steps

1. Create `internal/metrics/` package
   - `writer.go`: background goroutine that buffers events and writes JSONL
   - `reader.go`: reads event log, groups/filters, computes percentiles
   - `event.go`: Event struct, warning categories
   - `metrics_test.go`: concurrent writes, reader aggregation, rotation

2. Add middleware in `internal/server/server.go`
   - `AddReceivingMiddleware` that writes events for `tools/call` requests
   - Measure duration and response size
   - Emit `slog.Error` for failures, `slog.Warn` for slow (>2s) / large (>50KB)

3. Add `log_metrics` tool
   - Register in `internal/tools/register.go`
   - Input: since, group_by, tool, top_k
   - Reads event log and returns aggregated stats
   - The LLM can use this to self-diagnose

4. Emit shutdown summary
   - On context cancellation, flush writer and log per-tool stats

5. Tests
   - Unit: writer concurrency, reader aggregation, percentile math
   - Integration: middleware fires on tool call, log_metrics returns valid JSON

## Acceptance Criteria

- `go test -race ./...` passes
- Tool errors emit `slog.Error` to stderr
- Slow/large calls emit `slog.Warn` to stderr
- Events persist to `~/.log-analysis-mcp/metrics/events-YYYY-MM-DD.jsonl`
- `log_metrics` tool returns grouped stats matching nuclia-mcp pattern
- Shutdown emits per-tool summary
