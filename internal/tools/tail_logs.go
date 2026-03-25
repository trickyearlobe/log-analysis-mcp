// Package tools implements the 10 MCP tool handlers for log analysis.
package tools

import (
	"fmt"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
)

// LogLine represents a single line of log output with its line number.
type LogLine struct {
	LineNumber int    `json:"line_number"`
	Content    string `json:"content"`
}

// TailLogsInput is the input schema for the tail_logs tool.
type TailLogsInput struct {
	Path     string `json:"path"      jsonschema:"required,description=Path to the log file"`
	NumLines int    `json:"num_lines" jsonschema:"description=Number of lines to read from the end of the file (max 1000),minimum=1,maximum=1000"`
}

// TailLogsOutput is the result of a tail_logs invocation.
type TailLogsOutput struct {
	Lines           []LogLine `json:"lines"`
	TotalLines      int       `json:"total_lines"`
	FileSizeBytes   int64     `json:"file_size_bytes"`
	ShowingFromLine int       `json:"showing_from_line"`
}

// RunTailLogs reads the last N lines from a log file and returns them as structured output.
func RunTailLogs(input TailLogsInput) (TailLogsOutput, error) {
	numLines := DefaultInt(input.NumLines, 50)
	numLines = ClampInt(numLines, 1, 1000)

	if err := CheckFileAccess(input.Path); err != nil {
		return TailLogsOutput{}, fmt.Errorf("tail_logs: %w", err)
	}

	result, err := fileutil.TailLines(input.Path, numLines)
	if err != nil {
		return TailLogsOutput{}, fmt.Errorf("tail_logs: %w", err)
	}

	lines := make([]LogLine, len(result.Lines))
	for i, lr := range result.Lines {
		lines[i] = LogLine{
			LineNumber: lr.LineNumber,
			Content:    lr.Text,
		}
	}

	showingFrom := 0
	if len(lines) > 0 {
		showingFrom = lines[0].LineNumber
	}

	return TailLogsOutput{
		Lines:           lines,
		TotalLines:      result.TotalLines,
		FileSizeBytes:   result.FileSize,
		ShowingFromLine: showingFrom,
	}, nil
}
