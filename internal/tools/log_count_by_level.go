package tools

import (
	"fmt"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
)

// countByLevelPageSize is the number of lines to read per streaming page.
const countByLevelPageSize = 1000

// CountByLevelInput defines the parameters for the log_count_by_level tool.
type CountByLevelInput struct {
	Path string `json:"path" jsonschema:"Path to the log file"`
}

// CountByLevelOutput is the structured result of the log_count_by_level tool.
type CountByLevelOutput struct {
	Counts      map[string]int `json:"counts"`
	TotalLines  int            `json:"total_lines"`
	ParsedLines int            `json:"parsed_lines"`
}

// RunCountByLevel counts log entries by severity level in a single streaming pass.
func RunCountByLevel(input CountByLevelInput) (CountByLevelOutput, error) {
	if err := CheckFileAccess(input.Path); err != nil {
		return CountByLevelOutput{}, fmt.Errorf("log_count_by_level: %w", err)
	}

	sample, err := SampleLines(input.Path, sampleLineCount)
	if err != nil {
		return CountByLevelOutput{}, fmt.Errorf("log_count_by_level: %w", err)
	}
	_, parser := parsers.AutoDetectWithHint(sample, "")

	counts := make(map[string]int)
	totalLines := 0
	parsedLines := 0

	startLine := 1
	for {
		result, readErr := fileutil.ReadLines(input.Path, startLine, countByLevelPageSize)
		if readErr != nil {
			return CountByLevelOutput{}, fmt.Errorf("log_count_by_level: read at line %d: %w", startLine, readErr)
		}

		for _, lr := range result.Lines {
			totalLines++

			var levelStr string
			if parser != nil {
				entry := parser.Parse(lr.Text)
				if entry != nil && entry.Level != nil {
					levelStr = string(*entry.Level)
				}
			}
			// Fallback to keyword detection if parser didn't find a level.
			if levelStr == "" {
				level := inferLevelFromText(lr.Text)
				if level != nil {
					levelStr = string(*level)
				}
			}

			if levelStr != "" {
				counts[levelStr]++
				parsedLines++
			}
		}

		if !result.HasMore || len(result.Lines) == 0 {
			break
		}
		startLine += len(result.Lines)
	}

	if counts == nil {
		counts = make(map[string]int)
	}

	return CountByLevelOutput{
		Counts:      counts,
		TotalLines:  totalLines,
		ParsedLines: parsedLines,
	}, nil
}
