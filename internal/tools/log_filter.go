package tools

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// filterPageSize is the number of lines to read per streaming page.
const filterPageSize = 1000

// FilterLogsInput defines the parameters for the log_filter tool.
type FilterLogsInput struct {
	Path            string   `json:"path"                      jsonschema:"Path to the log file"`
	Level           []string `json:"level,omitempty"           jsonschema:"Log levels to include (e.g. ERROR, WARN)"`
	After           string   `json:"after,omitempty"           jsonschema:"ISO 8601 timestamp — include entries after this time"`
	Before          string   `json:"before,omitempty"          jsonschema:"ISO 8601 timestamp — include entries before this time"`
	Source          string   `json:"source,omitempty"          jsonschema:"Regex pattern to match the source/component field"`
	MessagePattern  string   `json:"message_pattern,omitempty" jsonschema:"Regex pattern to match the message content"`
	MaxResults      int      `json:"max_results,omitempty"     jsonschema:"Maximum entries to return (max 1000)"`
	RecordSeparator string   `json:"record_separator,omitempty" jsonschema:"RE2 regex matching the start of a new log record (returns full records on match)"`
}

// FilteredEntry represents a single log entry that matched the filter criteria.
type FilteredEntry struct {
	LineNumber int             `json:"line_number"`
	Timestamp  *string         `json:"timestamp"`
	Level      *types.LogLevel `json:"level"`
	Source     *string         `json:"source"`
	Message    string          `json:"message"`
	Raw        string          `json:"raw"`
}

// AppliedFilters records which filters were active during the search.
type AppliedFilters struct {
	Level  []string `json:"level,omitempty"`
	After  string   `json:"after,omitempty"`
	Before string   `json:"before,omitempty"`
}

// FilterLogsOutput is the structured result of the log_filter tool.
type FilterLogsOutput struct {
	Entries        []FilteredEntry `json:"entries"`
	TotalMatched   int             `json:"total_matched"`
	TotalScanned   int             `json:"total_scanned"`
	AppliedFilters AppliedFilters  `json:"applied_filters"`
	Truncated      bool            `json:"truncated"`
}

// RunFilterLogs filters parsed log entries by level, time range, source, and
// message content. Multiple filters are combined with AND logic.
func RunFilterLogs(input FilterLogsInput) (FilterLogsOutput, error) {
	// Apply defaults and clamp.
	input.MaxResults = DefaultInt(input.MaxResults, 100)
	input.MaxResults = ClampInt(input.MaxResults, 1, 1000)

	// Validate file access.
	if err := CheckFileAccess(input.Path); err != nil {
		return FilterLogsOutput{}, fmt.Errorf("log_filter: %w", err)
	}

	// Sample lines and auto-detect format.
	sample, err := SampleLines(input.Path, sampleLineCount)
	if err != nil {
		return FilterLogsOutput{}, fmt.Errorf("log_filter: %w", err)
	}
	_, parser := parsers.AutoDetectWithHint(sample, "")
	if parser == nil {
		// No parser detected — return empty results.
		return FilterLogsOutput{
			Entries: []FilteredEntry{},
			AppliedFilters: AppliedFilters{
				Level: input.Level,
				After: input.After,
				Before: input.Before,
			},
		}, nil
	}

	// Compile source regex if provided.
	var sourceRe *regexp.Regexp
	if input.Source != "" {
		var compileErr error
		sourceRe, _, compileErr = CompilePattern(input.Source, true, false)
		if compileErr != nil {
			return FilterLogsOutput{}, fmt.Errorf("log_filter: %w", compileErr)
		}
	}

	// Compile message_pattern regex if provided.
	var messageRe *regexp.Regexp
	if input.MessagePattern != "" {
		var compileErr error
		messageRe, _, compileErr = CompilePattern(input.MessagePattern, true, false)
		if compileErr != nil {
			return FilterLogsOutput{}, fmt.Errorf("log_filter: %w", compileErr)
		}
	}

	// Parse after/before timestamps if provided.
	var afterTime time.Time
	var hasAfter bool
	if input.After != "" {
		afterTime, err = time.Parse(time.RFC3339, input.After)
		if err != nil {
			return FilterLogsOutput{}, fmt.Errorf("log_filter: INVALID_TIMESTAMP: invalid timestamp format: %q — use ISO 8601 format (e.g., 2025-01-15T10:30:00Z)", input.After)
		}
		hasAfter = true
	}

	var beforeTime time.Time
	var hasBefore bool
	if input.Before != "" {
		beforeTime, err = time.Parse(time.RFC3339, input.Before)
		if err != nil {
			return FilterLogsOutput{}, fmt.Errorf("log_filter: INVALID_TIMESTAMP: invalid timestamp format: %q — use ISO 8601 format (e.g., 2025-01-15T10:30:00Z)", input.Before)
		}
		hasBefore = true
	}

	// Normalize requested levels to uppercase for case-insensitive comparison.
	levelSet := make(map[string]bool, len(input.Level))
	for _, l := range input.Level {
		levelSet[strings.ToUpper(l)] = true
	}
	hasLevelFilter := len(levelSet) > 0

	// Compile record separator if provided.
	var recordSep *regexp.Regexp
	if input.RecordSeparator != "" {
		recordSep, err = regexp.Compile(input.RecordSeparator)
		if err != nil {
			return FilterLogsOutput{}, fmt.Errorf("log_filter: VALIDATION_ERROR: invalid record_separator regex: %w", err)
		}
	}

	// Stream file and apply filters.
	var entries []FilteredEntry
	totalMatched := 0
	totalScanned := 0

	if recordSep != nil {
		entries, totalMatched, totalScanned, err = filterRecordMode(
			input.Path, recordSep, parser, input.MaxResults,
			hasLevelFilter, levelSet, hasAfter, afterTime, hasBefore, beforeTime,
			sourceRe, messageRe,
		)
		if err != nil {
			return FilterLogsOutput{}, err
		}
	} else {
		entries, totalMatched, totalScanned, err = filterLineMode(
			input.Path, parser, input.MaxResults,
			hasLevelFilter, levelSet, hasAfter, afterTime, hasBefore, beforeTime,
			sourceRe, messageRe,
		)
		if err != nil {
			return FilterLogsOutput{}, err
		}
	}

	if entries == nil {
		entries = []FilteredEntry{}
	}

	return FilterLogsOutput{
		Entries:      entries,
		TotalMatched: totalMatched,
		TotalScanned: totalScanned,
		AppliedFilters: AppliedFilters{
			Level: input.Level,
			After: input.After,
			Before: input.Before,
		},
		Truncated: totalMatched > len(entries),
	}, nil
}

// filterLineMode streams the file line-by-line and applies filters.
func filterLineMode(
	path string, parser parsers.Parser, maxResults int,
	hasLevelFilter bool, levelSet map[string]bool,
	hasAfter bool, afterTime time.Time, hasBefore bool, beforeTime time.Time,
	sourceRe, messageRe *regexp.Regexp,
) ([]FilteredEntry, int, int, error) {
	var entries []FilteredEntry
	totalMatched := 0
	totalScanned := 0

	startLine := 1
	for {
		result, readErr := fileutil.ReadLines(path, startLine, filterPageSize)
		if readErr != nil {
			return nil, 0, 0, fmt.Errorf("log_filter: read %s at line %d: %w", path, startLine, readErr)
		}

		for _, lr := range result.Lines {
			totalScanned++

			parsed := parser.Parse(lr.Text)
			if parsed == nil {
				continue
			}

			if !matchesFilters(parsed, parsed.Message, hasLevelFilter, levelSet, hasAfter, afterTime, hasBefore, beforeTime, sourceRe, messageRe) {
				continue
			}

			totalMatched++
			if len(entries) < maxResults {
				entries = append(entries, FilteredEntry{
					LineNumber: lr.LineNumber,
					Timestamp:  parsed.Timestamp,
					Level:      parsed.Level,
					Source:     parsed.Source,
					Message:    parsed.Message,
					Raw:        lr.Text,
				})
			}
		}

		if !result.HasMore || len(result.Lines) == 0 {
			break
		}
		startLine += len(result.Lines)
	}
	return entries, totalMatched, totalScanned, nil
}

// filterRecordMode uses RecordScanner to filter multi-line records.
// Structured fields (level, timestamp, source) come from the first line.
// message_pattern matches against the full record text.
func filterRecordMode(
	path string, sep *regexp.Regexp, parser parsers.Parser, maxResults int,
	hasLevelFilter bool, levelSet map[string]bool,
	hasAfter bool, afterTime time.Time, hasBefore bool, beforeTime time.Time,
	sourceRe, messageRe *regexp.Regexp,
) ([]FilteredEntry, int, int, error) {
	rs, err := fileutil.NewRecordScanner(path, sep)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("log_filter: %w", err)
	}
	defer rs.Close()

	var entries []FilteredEntry
	totalMatched := 0
	totalScanned := 0

	for rs.Scan() {
		rec := rs.Record()
		totalScanned += rec.LineCount

		// Parse the first line for structured fields.
		firstLine := rec.Text
		if idx := strings.IndexByte(rec.Text, '\n'); idx >= 0 {
			firstLine = rec.Text[:idx]
		}

		parsed := parser.Parse(firstLine)
		if parsed == nil {
			continue
		}

		// For message_pattern, match against the full record text.
		matchText := rec.Text
		if !matchesFilters(parsed, matchText, hasLevelFilter, levelSet, hasAfter, afterTime, hasBefore, beforeTime, sourceRe, messageRe) {
			continue
		}

		totalMatched++
		if len(entries) < maxResults {
			entries = append(entries, FilteredEntry{
				LineNumber: rec.StartLine,
				Timestamp:  parsed.Timestamp,
				Level:      parsed.Level,
				Source:     parsed.Source,
				Message:    parsed.Message,
				Raw:        rec.Text,
			})
		}
	}
	if rs.Err() != nil {
		return nil, 0, 0, fmt.Errorf("log_filter: record scan: %w", rs.Err())
	}
	return entries, totalMatched, totalScanned, nil
}

// matchesFilters applies all configured filters to a parsed entry.
// messageText is what message_pattern is matched against (may be full record in record mode).
func matchesFilters(
	parsed *types.ParsedLogEntry, messageText string,
	hasLevelFilter bool, levelSet map[string]bool,
	hasAfter bool, afterTime time.Time, hasBefore bool, beforeTime time.Time,
	sourceRe, messageRe *regexp.Regexp,
) bool {
	if hasLevelFilter {
		if parsed.Level == nil {
			return false
		}
		if !levelSet[strings.ToUpper(string(*parsed.Level))] {
			return false
		}
	}
	if hasAfter {
		if parsed.Timestamp == nil {
			return false
		}
		if *parsed.Timestamp < afterTime.Format(time.RFC3339Nano) {
			return false
		}
	}
	if hasBefore {
		if parsed.Timestamp == nil {
			return false
		}
		if *parsed.Timestamp >= beforeTime.Format(time.RFC3339Nano) {
			return false
		}
	}
	if sourceRe != nil {
		if parsed.Source == nil {
			return false
		}
		if !sourceRe.MatchString(*parsed.Source) {
			return false
		}
	}
	if messageRe != nil {
		if !messageRe.MatchString(messageText) {
			return false
		}
	}
	return true
}
