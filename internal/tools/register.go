// Package tools implements the 10 MCP tool handlers for log analysis.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register adds all tool definitions to the MCP server.
func Register(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_logs",
		Description: "Read a log file with pagination support. Returns lines from the specified file along with metadata about file size and total line count. Use start_line and num_lines to paginate through large files.",
	}, handleReadLogs)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "tail_logs",
		Description: "Read the last N lines of a log file (most recent entries). Equivalent to the Unix tail command. Useful for checking the latest activity in a log file.",
	}, handleTailLogs)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_logs",
		Description: "Search a log file using regex or text patterns. Returns matching lines with optional surrounding context. Useful for finding specific errors, request IDs, or patterns in log files.",
	}, handleSearchLogs)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "parse_logs",
		Description: "Auto-detect log format and parse entries into structured records with timestamp, level, source, and message fields.",
	}, handleParseLogs)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "filter_logs",
		Description: "Filter log entries by level, time range, source, or message pattern. All filters are combined with AND logic.",
	}, handleFilterLogs)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "extract_errors",
		Description: "Extract and cluster error entries by similarity. Normalizes variable parts to find common patterns.",
	}, handleExtractErrors)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "summarize_logs",
		Description: "Generate a statistical summary of a log file including level distribution, top sources, top errors, and throughput metrics.",
	}, handleSummarizeLogs)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "detect_anomalies",
		Description: "Detect anomalies in log files including error spikes, rate changes, gaps, and new error types.",
	}, handleDetectAnomalies)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "timeline",
		Description: "Build a chronological timeline of significant events from a log file.",
	}, handleTimeline)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "correlate_logs",
		Description: "Correlate events across multiple log files by a shared field like request_id or trace_id.",
	}, handleCorrelateLogs)
}

func handleReadLogs(_ context.Context, _ *mcp.CallToolRequest, input ReadLogsInput) (*mcp.CallToolResult, ReadLogsOutput, error) {
	result, err := RunReadLogs(input)
	if err != nil {
		return nil, ReadLogsOutput{}, err
	}
	return nil, result, nil
}

func handleTailLogs(_ context.Context, _ *mcp.CallToolRequest, input TailLogsInput) (*mcp.CallToolResult, TailLogsOutput, error) {
	result, err := RunTailLogs(input)
	if err != nil {
		return nil, TailLogsOutput{}, err
	}
	return nil, result, nil
}

func handleSearchLogs(_ context.Context, _ *mcp.CallToolRequest, input SearchLogsInput) (*mcp.CallToolResult, SearchLogsOutput, error) {
	result, err := RunSearchLogs(input)
	if err != nil {
		return nil, SearchLogsOutput{}, err
	}
	return nil, result, nil
}

func handleParseLogs(_ context.Context, _ *mcp.CallToolRequest, input ParseLogsInput) (*mcp.CallToolResult, ParseLogsOutput, error) {
	result, err := RunParseLogs(input)
	if err != nil {
		return nil, ParseLogsOutput{}, err
	}
	return nil, result, nil
}

func handleFilterLogs(_ context.Context, _ *mcp.CallToolRequest, input FilterLogsInput) (*mcp.CallToolResult, FilterLogsOutput, error) {
	result, err := RunFilterLogs(input)
	if err != nil {
		return nil, FilterLogsOutput{}, err
	}
	return nil, result, nil
}

func handleExtractErrors(_ context.Context, _ *mcp.CallToolRequest, input ExtractErrorsInput) (*mcp.CallToolResult, ExtractErrorsOutput, error) {
	result, err := RunExtractErrors(input)
	if err != nil {
		return nil, ExtractErrorsOutput{}, err
	}
	return nil, result, nil
}

func handleSummarizeLogs(_ context.Context, _ *mcp.CallToolRequest, input SummarizeLogsInput) (*mcp.CallToolResult, SummarizeLogsOutput, error) {
	result, err := RunSummarizeLogs(input)
	if err != nil {
		return nil, SummarizeLogsOutput{}, err
	}
	return nil, result, nil
}

func handleDetectAnomalies(_ context.Context, _ *mcp.CallToolRequest, input DetectAnomaliesInput) (*mcp.CallToolResult, DetectAnomaliesOutput, error) {
	result, err := RunDetectAnomalies(input)
	if err != nil {
		return nil, DetectAnomaliesOutput{}, err
	}
	return nil, result, nil
}

func handleTimeline(_ context.Context, _ *mcp.CallToolRequest, input TimelineInput) (*mcp.CallToolResult, TimelineOutput, error) {
	result, err := RunTimeline(input)
	if err != nil {
		return nil, TimelineOutput{}, err
	}
	return nil, result, nil
}

func handleCorrelateLogs(_ context.Context, _ *mcp.CallToolRequest, input CorrelateLogsInput) (*mcp.CallToolResult, CorrelateLogsOutput, error) {
	result, err := RunCorrelateLogs(input)
	if err != nil {
		return nil, CorrelateLogsOutput{}, err
	}
	return nil, result, nil
}
