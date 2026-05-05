package tools

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// extractErrorsPageSize is the number of lines to read per streaming page.
const extractErrorsPageSize = 1000

// maxSampleMessages is the maximum number of sample messages kept per cluster.
const maxSampleMessages = 3

// ExtractErrorsInput defines the parameters for the log_extract_errors tool.
type ExtractErrorsInput struct {
	Path               string `json:"path"                           jsonschema:"Path to the log file"`
	IncludeStackTraces bool   `json:"include_stack_traces,omitempty" jsonschema:"Capture multiline stack traces with errors"`
	MaxClusters        int    `json:"max_clusters,omitempty"         jsonschema:"Maximum number of error clusters to return (max 100)"`
	Offset             int    `json:"offset,omitempty"               jsonschema:"Number of clusters to skip for pagination"`
	SortBy             string `json:"sort_by,omitempty"              jsonschema:"Sort clusters by: count (default) or impact (count × severity weight)"`
	RecordSeparator    string `json:"record_separator,omitempty"     jsonschema:"RE2 regex matching the start of a new log record (groups multi-line entries)"`
}

// ErrorRate contains error frequency statistics.
type ErrorRate struct {
	ErrorsPerHour        float64 `json:"errors_per_hour"`
	PercentageOfAllLines float64 `json:"percentage_of_all_lines"`
}

// ExtractErrorsOutput is the structured result of the log_extract_errors tool.
type ExtractErrorsOutput struct {
	Clusters       []types.ErrorCluster `json:"clusters"`
	TotalErrors    int                  `json:"total_errors"`
	TotalClusters  int                  `json:"total_clusters"`
	ErrorRate      ErrorRate            `json:"error_rate"`
	LevelsIncluded []string             `json:"levels_included"`
	HasMore        bool                 `json:"has_more"`
	NextOffset     int                  `json:"next_offset"`
}

// Compiled normalization regexes (RE2-compatible, no lookaheads/lookbehinds).
var (
	uuidRe      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	ipRe        = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	hexRe       = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	pathRe      = regexp.MustCompile(`/[\w./\-]+`)
	quotedStrRe = regexp.MustCompile(`"[^"]*"`)
	numberRe    = regexp.MustCompile(`\b\d+\b`)
)

// normalizeMessage replaces variable parts of a log message with placeholders
// so that structurally identical messages cluster together.
func normalizeMessage(msg string) string {
	// Order matters: more specific patterns first to avoid partial replacement.
	msg = uuidRe.ReplaceAllString(msg, "<UUID>")
	msg = ipRe.ReplaceAllString(msg, "<IP>")
	msg = hexRe.ReplaceAllString(msg, "<HEX>")
	msg = pathRe.ReplaceAllString(msg, "<PATH>")
	msg = quotedStrRe.ReplaceAllString(msg, "<STR>")
	msg = numberRe.ReplaceAllString(msg, "<N>")
	return msg
}

// isErrorLevel returns true if the given level is ERROR, FATAL, or CRITICAL.
func isErrorLevel(level *types.LogLevel) bool {
	if level == nil {
		return false
	}
	switch *level {
	case types.LogLevelError, types.LogLevelFatal, types.LogLevelCritical:
		return true
	}
	return false
}

// levelKeywordRe matches common level keywords in free-form log lines.
var levelKeywordRe = regexp.MustCompile(`(?i)\b(FATAL|CRITICAL|ERROR|WARN(?:ING)?|INFO|DEBUG|TRACE)\b`)

// inferLevelFromText scans a line for level keywords when no parser is available.
// Returns nil if no recognizable level keyword is found.
func inferLevelFromText(line string) *types.LogLevel {
	match := levelKeywordRe.FindString(line)
	if match == "" {
		return nil
	}
	upper := strings.ToUpper(match)
	var level types.LogLevel
	switch upper {
	case "FATAL":
		level = types.LogLevelFatal
	case "CRITICAL":
		level = types.LogLevelCritical
	case "ERROR":
		level = types.LogLevelError
	case "WARN", "WARNING":
		level = types.LogLevelWarn
	case "INFO":
		level = types.LogLevelInfo
	case "DEBUG":
		level = types.LogLevelDebug
	case "TRACE":
		level = types.LogLevelTrace
	default:
		return nil
	}
	return &level
}

// clusterAccumulator tracks state for a single error cluster during streaming.
type clusterAccumulator struct {
	pattern        string
	count          int
	maxSeverity    types.LogLevel
	firstSeen      types.SeenAt
	lastSeen       types.SeenAt
	sampleMessages []string
	stackTrace     *string
}

// severityWeight returns the impact weight for a log level.
func severityWeight(level types.LogLevel) float64 {
	switch level {
	case types.LogLevelFatal:
		return 10
	case types.LogLevelCritical:
		return 8
	case types.LogLevelError:
		return 5
	case types.LogLevelWarn:
		return 2
	default:
		return 1
	}
}

// RunExtractErrors extracts error-level log entries from a file, clusters them
// by normalized message pattern, and returns sorted clusters with statistics.
func RunExtractErrors(input ExtractErrorsInput) (ExtractErrorsOutput, error) {
	// Apply defaults and clamp.
	input.MaxClusters = DefaultInt(input.MaxClusters, 20)
	input.MaxClusters = ClampInt(input.MaxClusters, 1, 100)
	input.SortBy = DefaultString(input.SortBy, "count")
	if input.Offset < 0 {
		input.Offset = 0
	}

	if input.SortBy != "count" && input.SortBy != "impact" {
		return ExtractErrorsOutput{}, fmt.Errorf("log_extract_errors: VALIDATION_ERROR: sort_by must be \"count\" or \"impact\", got %q", input.SortBy)
	}

	// Validate file access.
	if err := CheckFileAccess(input.Path); err != nil {
		return ExtractErrorsOutput{}, fmt.Errorf("log_extract_errors: %w", err)
	}

	// Sample lines and auto-detect format.
	// When record_separator is active, we still need a parser for the first line of each record.
	sampleLines, err := SampleLines(input.Path, sampleLineCount)
	if err != nil {
		return ExtractErrorsOutput{}, fmt.Errorf("log_extract_errors: %w", err)
	}

	_, parser := parsers.AutoDetectWithHint(sampleLines, "")

	// Compile record separator regex if provided.
	var recordSep *regexp.Regexp
	if input.RecordSeparator != "" {
		recordSep, err = regexp.Compile(input.RecordSeparator)
		if err != nil {
			return ExtractErrorsOutput{}, fmt.Errorf("log_extract_errors: VALIDATION_ERROR: invalid record_separator regex: %w", err)
		}
	}

	// Build a MultilineCombiner if stack traces are requested, no record_separator,
	// and we have a parser.
	var combiner *parsers.MultilineCombiner
	if input.IncludeStackTraces && parser != nil && recordSep == nil {
		combiner = parsers.NewMultilineCombiner(parser)
	}

	// Cluster map keyed by normalized pattern.
	clusters := make(map[string]*clusterAccumulator)
	totalLines := 0
	totalErrors := 0

	// Track first and last timestamps for error rate calculation.
	var firstTimestamp, lastTimestamp *string

	if recordSep != nil {
		// Record-separator mode: use RecordScanner.
		totalLines, totalErrors, err = extractErrorsRecordMode(
			input, recordSep, parser, clusters, &firstTimestamp, &lastTimestamp,
		)
		if err != nil {
			return ExtractErrorsOutput{}, err
		}
	} else {
		// Line-based mode (existing behaviour).
		totalLines, totalErrors, err = extractErrorsLineMode(
			input, parser, combiner, clusters, &firstTimestamp, &lastTimestamp,
		)
		if err != nil {
			return ExtractErrorsOutput{}, err
		}
	}

	// Convert cluster map to sorted slice.
	clusterSlice := make([]types.ErrorCluster, 0, len(clusters))
	for _, acc := range clusters {
		pct := 0.0
		if totalErrors > 0 {
			pct = float64(acc.count) / float64(totalErrors) * 100.0
		}
		samples := acc.sampleMessages
		if samples == nil {
			samples = []string{}
		}
		impact := float64(acc.count) * severityWeight(acc.maxSeverity)
		clusterSlice = append(clusterSlice, types.ErrorCluster{
			Pattern:        acc.pattern,
			Count:          acc.count,
			Percentage:     pct,
			ImpactScore:    impact,
			FirstSeen:      acc.firstSeen,
			LastSeen:       acc.lastSeen,
			SampleMessages: samples,
			StackTrace:     acc.stackTrace,
		})
	}

	// Sort clusters.
	sort.Slice(clusterSlice, func(i, j int) bool {
		if input.SortBy == "impact" {
			if clusterSlice[i].ImpactScore != clusterSlice[j].ImpactScore {
				return clusterSlice[i].ImpactScore > clusterSlice[j].ImpactScore
			}
		} else {
			if clusterSlice[i].Count != clusterSlice[j].Count {
				return clusterSlice[i].Count > clusterSlice[j].Count
			}
		}
		return clusterSlice[i].Pattern < clusterSlice[j].Pattern
	})

	// Apply pagination: offset then cap.
	totalClusters := len(clusterSlice)
	if input.Offset >= len(clusterSlice) {
		clusterSlice = []types.ErrorCluster{}
	} else {
		clusterSlice = clusterSlice[input.Offset:]
		if len(clusterSlice) > input.MaxClusters {
			clusterSlice = clusterSlice[:input.MaxClusters]
		}
	}

	// Ensure non-nil slice for JSON marshaling.
	if clusterSlice == nil {
		clusterSlice = []types.ErrorCluster{}
	}

	hasMore := totalClusters > input.Offset+len(clusterSlice)
	nextOffset := input.Offset + len(clusterSlice)

	// Compute error rate.
	errRate := ErrorRate{}
	if totalLines > 0 {
		errRate.PercentageOfAllLines = float64(totalErrors) / float64(totalLines) * 100.0
	}
	errRate.ErrorsPerHour = computeErrorsPerHour(firstTimestamp, lastTimestamp, totalErrors)

	return ExtractErrorsOutput{
		Clusters:       clusterSlice,
		TotalErrors:    totalErrors,
		TotalClusters:  totalClusters,
		ErrorRate:      errRate,
		LevelsIncluded: []string{"ERROR", "FATAL", "CRITICAL"},
		HasMore:        hasMore,
		NextOffset:     nextOffset,
	}, nil
}

// computeErrorsPerHour calculates error frequency from the time range spanned
// by the first and last error timestamps. Returns 0 if timestamps are missing
// or unparseable.
func computeErrorsPerHour(first, last *string, totalErrors int) float64 {
	if first == nil || last == nil || totalErrors == 0 {
		return 0
	}

	t1, err1 := parseTimestamp(*first)
	t2, err2 := parseTimestamp(*last)
	if err1 != nil || err2 != nil {
		return 0
	}

	duration := t2.Sub(t1)
	if duration <= 0 {
		return 0
	}

	hours := duration.Hours()
	if hours == 0 {
		return 0
	}
	return float64(totalErrors) / hours
}

// Common timestamp formats to try when parsing.
var timestampFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05",
	"Jan  2 15:04:05",
	"Jan 2 15:04:05",
	"02/Jan/2006:15:04:05 -0700",
}

// parseTimestamp attempts to parse a timestamp string using common formats.
func parseTimestamp(s string) (time.Time, error) {
	for _, fmt := range timestampFormats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp format: %q", s)
}

// extractErrorsLineMode processes errors using line-based ReadLines pagination.
func extractErrorsLineMode(
	input ExtractErrorsInput,
	parser parsers.Parser,
	combiner *parsers.MultilineCombiner,
	clusters map[string]*clusterAccumulator,
	firstTimestamp, lastTimestamp **string,
) (totalLines, totalErrors int, err error) {
	startLine := 1
	for {
		result, readErr := fileutil.ReadLines(input.Path, startLine, extractErrorsPageSize)
		if readErr != nil {
			return totalLines, totalErrors, fmt.Errorf("log_extract_errors: read at line %d: %w", startLine, readErr)
		}

		totalLines += len(result.Lines)

		var entries []*types.ParsedLogEntry

		if combiner != nil {
			rawLines := make([]string, len(result.Lines))
			for i, lr := range result.Lines {
				rawLines[i] = lr.Text
			}
			pageStartLine := startLine
			if len(result.Lines) > 0 {
				pageStartLine = result.Lines[0].LineNumber
			}
			entries = combiner.Combine(rawLines, pageStartLine)
		} else if parser != nil {
			for _, lr := range result.Lines {
				entry := parser.Parse(lr.Text)
				if entry != nil {
					entry.LineNumber = lr.LineNumber
					entry.LineCount = 1
					entries = append(entries, entry)
				}
			}
		} else {
			if !result.HasMore || len(result.Lines) == 0 {
				break
			}
			startLine += len(result.Lines)
			continue
		}

		totalErrors += accumulateErrors(entries, clusters, firstTimestamp, lastTimestamp)

		if !result.HasMore || len(result.Lines) == 0 {
			break
		}
		startLine += len(result.Lines)
	}
	return totalLines, totalErrors, nil
}

// extractErrorsRecordMode processes errors using RecordScanner for multi-line grouping.
func extractErrorsRecordMode(
	input ExtractErrorsInput,
	sep *regexp.Regexp,
	parser parsers.Parser,
	clusters map[string]*clusterAccumulator,
	firstTimestamp, lastTimestamp **string,
) (totalLines, totalErrors int, err error) {
	rs, err := fileutil.NewRecordScanner(input.Path, sep)
	if err != nil {
		return 0, 0, fmt.Errorf("log_extract_errors: %w", err)
	}
	defer rs.Close()

	for rs.Scan() {
		rec := rs.Record()
		totalLines += rec.LineCount

		// Parse the first line of the record.
		firstLine := rec.Text
		if idx := strings.IndexByte(rec.Text, '\n'); idx >= 0 {
			firstLine = rec.Text[:idx]
		}

		var entry *types.ParsedLogEntry
		if parser != nil {
			entry = parser.Parse(firstLine)
		}
		// Fallback: if parser didn't recognize the line, infer level from
		// keywords so record_separator works with unrecognized formats.
		if entry == nil {
			level := inferLevelFromText(firstLine)
			if level == nil {
				continue
			}
			entry = &types.ParsedLogEntry{
				Level:   level,
				Message: firstLine,
				Raw:     firstLine,
			}
		}

		entry.LineNumber = rec.StartLine
		entry.LineCount = rec.LineCount

		// Remaining lines become the stack trace.
		if idx := strings.IndexByte(rec.Text, '\n'); idx >= 0 {
			entry.StackTrace = rec.Text[idx+1:]
		}

		if !isErrorLevel(entry.Level) {
			continue
		}

		entries := []*types.ParsedLogEntry{entry}
		totalErrors += accumulateErrors(entries, clusters, firstTimestamp, lastTimestamp)
	}
	if rs.Err() != nil {
		return totalLines, totalErrors, fmt.Errorf("log_extract_errors: record scan: %w", rs.Err())
	}
	return totalLines, totalErrors, nil
}

// accumulateErrors processes parsed entries and adds error-level ones to clusters.
// Returns the number of errors found.
func accumulateErrors(
	entries []*types.ParsedLogEntry,
	clusters map[string]*clusterAccumulator,
	firstTimestamp, lastTimestamp **string,
) int {
	count := 0
	for _, entry := range entries {
		if !isErrorLevel(entry.Level) {
			continue
		}
		count++

		normalized := normalizeMessage(entry.Message)
		seen := types.SeenAt{
			Timestamp:  entry.Timestamp,
			LineNumber: entry.LineNumber,
		}

		if entry.Timestamp != nil {
			if *firstTimestamp == nil {
				*firstTimestamp = entry.Timestamp
			}
			*lastTimestamp = entry.Timestamp
		}

		acc, exists := clusters[normalized]
		if !exists {
			var st *string
			if entry.StackTrace != "" {
				s := entry.StackTrace
				st = &s
			}
			acc = &clusterAccumulator{
				pattern:     normalized,
				count:       0,
				maxSeverity: *entry.Level,
				firstSeen:   seen,
				lastSeen:    seen,
				stackTrace:  st,
			}
			clusters[normalized] = acc
		}

		acc.count++
		acc.lastSeen = seen

		if severityWeight(*entry.Level) > severityWeight(acc.maxSeverity) {
			acc.maxSeverity = *entry.Level
		}

		if len(acc.sampleMessages) < maxSampleMessages {
			acc.sampleMessages = append(acc.sampleMessages, entry.Message)
		}
	}
	return count
}
