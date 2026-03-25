package tools

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// extractErrorsPageSize is the number of lines to read per streaming page.
const extractErrorsPageSize = 1000

// maxSampleMessages is the maximum number of sample messages kept per cluster.
const maxSampleMessages = 3

// ExtractErrorsInput defines the parameters for the extract_errors tool.
type ExtractErrorsInput struct {
	Path               string `json:"path"                 jsonschema:"required,description=Path to the log file"`
	IncludeStackTraces bool   `json:"include_stack_traces" jsonschema:"description=Capture multiline stack traces with errors"`
	MaxClusters        int    `json:"max_clusters"         jsonschema:"description=Maximum number of error clusters to return (max 100),minimum=1,maximum=100"`
}

// ErrorRate contains error frequency statistics.
type ErrorRate struct {
	ErrorsPerHour        float64 `json:"errors_per_hour"`
	PercentageOfAllLines float64 `json:"percentage_of_all_lines"`
}

// ExtractErrorsOutput is the structured result of the extract_errors tool.
type ExtractErrorsOutput struct {
	Clusters       []types.ErrorCluster `json:"clusters"`
	TotalErrors    int                  `json:"total_errors"`
	ErrorRate      ErrorRate            `json:"error_rate"`
	LevelsIncluded []string             `json:"levels_included"`
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

// clusterAccumulator tracks state for a single error cluster during streaming.
type clusterAccumulator struct {
	pattern        string
	count          int
	firstSeen      types.SeenAt
	lastSeen       types.SeenAt
	sampleMessages []string
	stackTrace     *string
}

// RunExtractErrors extracts error-level log entries from a file, clusters them
// by normalized message pattern, and returns sorted clusters with statistics.
func RunExtractErrors(input ExtractErrorsInput) (ExtractErrorsOutput, error) {
	// Apply defaults and clamp.
	input.MaxClusters = DefaultInt(input.MaxClusters, 20)
	input.MaxClusters = ClampInt(input.MaxClusters, 1, 100)

	// Validate file access.
	if err := CheckFileAccess(input.Path); err != nil {
		return ExtractErrorsOutput{}, fmt.Errorf("extract_errors: %w", err)
	}

	// Sample lines and auto-detect format.
	sampleLines, err := SampleLines(input.Path, sampleLineCount)
	if err != nil {
		return ExtractErrorsOutput{}, fmt.Errorf("extract_errors: %w", err)
	}

	_, parser := parsers.AutoDetectWithHint(sampleLines, "")

	// Build a MultilineCombiner if stack traces are requested and we have a parser.
	var combiner *parsers.MultilineCombiner
	if input.IncludeStackTraces && parser != nil {
		combiner = parsers.NewMultilineCombiner(parser)
	}

	// Cluster map keyed by normalized pattern.
	clusters := make(map[string]*clusterAccumulator)
	totalLines := 0
	totalErrors := 0

	// Track first and last timestamps for error rate calculation.
	var firstTimestamp, lastTimestamp *string

	startLine := 1
	for {
		result, err := fileutil.ReadLines(input.Path, startLine, extractErrorsPageSize)
		if err != nil {
			return ExtractErrorsOutput{}, fmt.Errorf("extract_errors: read at line %d: %w", startLine, err)
		}

		totalLines += len(result.Lines)

		// Collect raw text for this page.
		var entries []*types.ParsedLogEntry

		if combiner != nil {
			// Use multiline combiner for stack trace aggregation.
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
			// Parse line by line without multiline combining.
			for _, lr := range result.Lines {
				entry := parser.Parse(lr.Text)
				if entry != nil {
					entry.LineNumber = lr.LineNumber
					entry.LineCount = 1
					entries = append(entries, entry)
				}
			}
		} else {
			// No parser detected — skip (no structured parsing possible).
			// We still counted the lines above for totalLines.
			if !result.HasMore || len(result.Lines) == 0 {
				break
			}
			startLine += len(result.Lines)
			continue
		}

		// Filter for error-level entries and cluster them.
		for _, entry := range entries {
			if !isErrorLevel(entry.Level) {
				continue
			}

			totalErrors++

			normalized := normalizeMessage(entry.Message)
			seen := types.SeenAt{
				Timestamp:  entry.Timestamp,
				LineNumber: entry.LineNumber,
			}

			// Track global first/last timestamps for error rate.
			if entry.Timestamp != nil {
				if firstTimestamp == nil {
					firstTimestamp = entry.Timestamp
				}
				lastTimestamp = entry.Timestamp
			}

			acc, exists := clusters[normalized]
			if !exists {
				var st *string
				if entry.StackTrace != "" {
					s := entry.StackTrace
					st = &s
				}
				acc = &clusterAccumulator{
					pattern:   normalized,
					count:     0,
					firstSeen: seen,
					lastSeen:  seen,
					stackTrace: st,
				}
				clusters[normalized] = acc
			}

			acc.count++
			acc.lastSeen = seen

			if len(acc.sampleMessages) < maxSampleMessages {
				acc.sampleMessages = append(acc.sampleMessages, entry.Message)
			}
		}

		if !result.HasMore || len(result.Lines) == 0 {
			break
		}
		startLine += len(result.Lines)
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
		clusterSlice = append(clusterSlice, types.ErrorCluster{
			Pattern:        acc.pattern,
			Count:          acc.count,
			Percentage:     pct,
			FirstSeen:      acc.firstSeen,
			LastSeen:       acc.lastSeen,
			SampleMessages: samples,
			StackTrace:     acc.stackTrace,
		})
	}

	// Sort by count descending, then by pattern for deterministic ordering.
	sort.Slice(clusterSlice, func(i, j int) bool {
		if clusterSlice[i].Count != clusterSlice[j].Count {
			return clusterSlice[i].Count > clusterSlice[j].Count
		}
		return clusterSlice[i].Pattern < clusterSlice[j].Pattern
	})

	// Truncate to MaxClusters.
	if len(clusterSlice) > input.MaxClusters {
		clusterSlice = clusterSlice[:input.MaxClusters]
	}

	// Ensure non-nil slice for JSON marshaling.
	if clusterSlice == nil {
		clusterSlice = []types.ErrorCluster{}
	}

	// Compute error rate.
	errRate := ErrorRate{}
	if totalLines > 0 {
		errRate.PercentageOfAllLines = float64(totalErrors) / float64(totalLines) * 100.0
	}
	errRate.ErrorsPerHour = computeErrorsPerHour(firstTimestamp, lastTimestamp, totalErrors)

	return ExtractErrorsOutput{
		Clusters:       clusterSlice,
		TotalErrors:    totalErrors,
		ErrorRate:      errRate,
		LevelsIncluded: []string{"ERROR", "FATAL", "CRITICAL"},
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
