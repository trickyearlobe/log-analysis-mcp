package tools

import (
	"fmt"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// ParseLogsInput defines the parameters for the parse_logs tool.
type ParseLogsInput struct {
	Path       string `json:"path"        jsonschema:"required,description=Path to the log file"`
	StartLine  int    `json:"start_line"  jsonschema:"description=Line number to start parsing from (1-based),minimum=1"`
	NumLines   int    `json:"num_lines"   jsonschema:"description=Number of lines to parse (max 500),minimum=1,maximum=500"`
	FormatHint string `json:"format_hint" jsonschema:"description=Log format hint,enum=syslog-rfc3164,enum=syslog-rfc5424,enum=apache-combined,enum=apache-common,enum=json,enum=auto"`
}

// ParsedRecord represents a single successfully parsed log entry.
type ParsedRecord struct {
	LineNumber  int                    `json:"line_number"`
	Timestamp   *string                `json:"timestamp"`
	Level       *types.LogLevel        `json:"level"`
	Source      *string                `json:"source"`
	Message     string                 `json:"message"`
	Raw         string                 `json:"raw"`
	ExtraFields map[string]interface{} `json:"extra_fields,omitempty"`
}

// ParseError represents a line that could not be parsed.
type ParseError struct {
	LineNumber int    `json:"line_number"`
	Raw        string `json:"raw"`
	Error      string `json:"error"`
}

// ParseLogsOutput is the structured result of the parse_logs tool.
type ParseLogsOutput struct {
	DetectedFormat string         `json:"detected_format"`
	Confidence     float64        `json:"confidence"`
	Records        []ParsedRecord `json:"records"`
	ParseErrors    []ParseError   `json:"parse_errors"`
	TotalParsed    int            `json:"total_parsed"`
	TotalErrors    int            `json:"total_errors"`
}

// RunParseLogs auto-detects the log format and parses log lines into structured records.
func RunParseLogs(input ParseLogsInput) (ParseLogsOutput, error) {
	// Apply defaults and clamp.
	input.StartLine = DefaultInt(input.StartLine, 1)
	input.NumLines = DefaultInt(input.NumLines, 50)
	input.NumLines = ClampInt(input.NumLines, 1, 500)
	input.FormatHint = DefaultString(input.FormatHint, "auto")

	// Validate file access.
	if err := CheckFileAccess(input.Path); err != nil {
		return ParseLogsOutput{}, fmt.Errorf("parse_logs: %w", err)
	}

	// Sample lines for format detection.
	sampleLines, err := SampleLines(input.Path, sampleLineCount)
	if err != nil {
		return ParseLogsOutput{}, fmt.Errorf("parse_logs: %w", err)
	}

	// Detect format and obtain parser.
	detection, parser := parsers.AutoDetectWithHint(sampleLines, input.FormatHint)

	// Read the requested range of lines.
	result, err := fileutil.ReadLines(input.Path, input.StartLine, input.NumLines)
	if err != nil {
		return ParseLogsOutput{}, fmt.Errorf("parse_logs: read lines: %w", err)
	}

	records := make([]ParsedRecord, 0, len(result.Lines))
	parseErrors := make([]ParseError, 0)

	for _, lr := range result.Lines {
		if parser == nil {
			// No parser detected — every line is a parse error.
			parseErrors = append(parseErrors, ParseError{
				LineNumber: lr.LineNumber,
				Raw:        lr.Text,
				Error:      "no parser available for unknown format",
			})
			continue
		}

		entry := parser.Parse(lr.Text)
		if entry == nil {
			parseErrors = append(parseErrors, ParseError{
				LineNumber: lr.LineNumber,
				Raw:        lr.Text,
				Error:      fmt.Sprintf("line does not match expected %s format", detection.Format),
			})
			continue
		}

		records = append(records, ParsedRecord{
			LineNumber:  lr.LineNumber,
			Timestamp:   entry.Timestamp,
			Level:       entry.Level,
			Source:      entry.Source,
			Message:     entry.Message,
			Raw:         entry.Raw,
			ExtraFields: entry.ExtraFields,
		})
	}

	return ParseLogsOutput{
		DetectedFormat: string(detection.Format),
		Confidence:     detection.Confidence,
		Records:        records,
		ParseErrors:    parseErrors,
		TotalParsed:    len(records),
		TotalErrors:    len(parseErrors),
	}, nil
}
