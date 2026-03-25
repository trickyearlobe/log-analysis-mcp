// Package tools implements the 10 MCP tool handlers for log analysis.
package tools

import (
	"fmt"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
)

// ReadLogsInput defines the parameters for the read_logs tool.
type ReadLogsInput struct {
	Path      string `json:"path"                 jsonschema:"Path to the log file"`
	StartLine int    `json:"start_line,omitempty" jsonschema:"Line number to start reading from (1-based)"`
	NumLines  int    `json:"num_lines,omitempty"  jsonschema:"Number of lines to return (max 1000)"`
	Encoding  string `json:"encoding,omitempty"   jsonschema:"File encoding (utf-8, ascii, latin1)"`
}

// LineRange describes the inclusive range of line numbers returned.
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// ReadLogsOutput is the structured result of the read_logs tool.
type ReadLogsOutput struct {
	Lines         []LogLine `json:"lines"`
	TotalLines    int       `json:"total_lines"`
	HasMore       bool      `json:"has_more"`
	FileSizeBytes int64     `json:"file_size_bytes"`
	CurrentRange  LineRange `json:"current_range"`
}

// RunReadLogs reads lines from a log file with pagination and returns structured output.
func RunReadLogs(input ReadLogsInput) (ReadLogsOutput, error) {
	// Apply defaults.
	input.StartLine = DefaultInt(input.StartLine, 1)
	input.NumLines = DefaultInt(input.NumLines, 100)
	input.NumLines = ClampInt(input.NumLines, 1, 1000)
	input.Encoding = DefaultString(input.Encoding, "utf-8")

	// Validate file access (exists, readable, not binary).
	if err := CheckFileAccess(input.Path); err != nil {
		return ReadLogsOutput{}, fmt.Errorf("read_logs: %w", err)
	}

	// Get file size for metadata.
	size, err := FileSize(input.Path)
	if err != nil {
		return ReadLogsOutput{}, fmt.Errorf("read_logs: %w", err)
	}

	// Stream lines from the file.
	result, err := fileutil.ReadLines(input.Path, input.StartLine, input.NumLines)
	if err != nil {
		return ReadLogsOutput{}, fmt.Errorf("read_logs: %w", err)
	}

	// Convert fileutil.LineRecord slice to LogLine slice.
	lines := make([]LogLine, len(result.Lines))
	for i, lr := range result.Lines {
		lines[i] = LogLine{
			LineNumber: lr.LineNumber,
			Content:    lr.Text,
		}
	}

	// Compute current range.
	rangeStart := input.StartLine
	rangeEnd := input.StartLine
	if len(result.Lines) > 0 {
		rangeStart = result.Lines[0].LineNumber
		rangeEnd = result.Lines[len(result.Lines)-1].LineNumber
	}

	// Estimate total lines: accurate when we've seen everything, zero otherwise.
	totalLines := 0
	if !result.HasMore {
		totalLines = input.StartLine - 1 + result.TotalRead
	}

	return ReadLogsOutput{
		Lines:         lines,
		TotalLines:    totalLines,
		HasMore:       result.HasMore,
		FileSizeBytes: size,
		CurrentRange: LineRange{
			Start: rangeStart,
			End:   rangeEnd,
		},
	}, nil
}
