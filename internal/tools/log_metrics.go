package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe/log-analysis-mcp/internal/metrics"
)

// LogMetricsInput defines parameters for the log_metrics tool.
type LogMetricsInput struct {
	Since   string `json:"since" jsonschema:"Time window (e.g. 24h, 7d). Default: 24h"`
	GroupBy string `json:"group_by" jsonschema:"Aggregation key: tool, status, or warning. Default: tool"`
	Tool    string `json:"tool" jsonschema:"Filter to a single tool name (optional)"`
	TopK    int    `json:"top_k" jsonschema:"Max groups to return. Default: 10"`
}

// RunLogMetrics queries the metrics event log.
func RunLogMetrics(metricsDir string, input LogMetricsInput) (*metrics.QueryOutput, error) {
	qi := metrics.QueryInput{
		Since:   input.Since,
		GroupBy: input.GroupBy,
		Tool:    input.Tool,
		TopK:    input.TopK,
	}
	result, err := metrics.Query(metricsDir, qi)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func makeLogMetricsHandler(metricsDir string) func(context.Context, *mcp.CallToolRequest, LogMetricsInput) (*mcp.CallToolResult, any, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, input LogMetricsInput) (*mcp.CallToolResult, any, error) {
		result, err := RunLogMetrics(metricsDir, input)
		if err != nil {
			return nil, nil, err
		}
		return nil, result, nil
	}
}
