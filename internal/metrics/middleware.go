package metrics

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewMiddleware returns an MCP receiving middleware that records tool call metrics.
func NewMiddleware(w *Writer) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			toolName := extractToolName(req)
			start := time.Now()

			result, err := next(ctx, method, req)

			duration := time.Since(start)
			durationMs := duration.Milliseconds()

			event := Event{
				Timestamp:  start,
				Tool:       toolName,
				DurationMs: durationMs,
			}

			if err != nil {
				event.Status = StatusError
				event.ErrorCode = "HANDLER_ERROR"
				slog.Error("tool call failed", "tool", toolName, "duration_ms", durationMs, "error", err)
			} else if ctr, ok := result.(*mcp.CallToolResult); ok && ctr.IsError {
				event.Status = StatusError
				event.ErrorCode = "TOOL_ERROR"
				slog.Error("tool returned error", "tool", toolName, "duration_ms", durationMs)
			} else {
				event.Status = StatusOK
			}

			event.ResponseBytes = estimateResponseSize(result)

			// Detect warnings
			if durationMs > SlowCallThresholdMs {
				event.Warning = WarnSlowCall
				slog.Warn("slow tool call", "tool", toolName, "duration_ms", durationMs)
			} else if event.ResponseBytes > LargeResponseThreshold {
				event.Warning = WarnLargeResponse
				slog.Warn("large tool response", "tool", toolName, "response_bytes", event.ResponseBytes)
			}

			w.Record(event)
			return result, err
		}
	}
}

func extractToolName(req mcp.Request) string {
	params := req.GetParams()
	if p, ok := params.(*mcp.CallToolParamsRaw); ok {
		return p.Name
	}
	return "unknown"
}

func estimateResponseSize(result mcp.Result) int {
	if result == nil {
		return 0
	}
	data, err := json.Marshal(result)
	if err != nil {
		return 0
	}
	return len(data)
}
