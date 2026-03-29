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

// correlatePageSize is the number of lines to read per streaming page.
const correlatePageSize = 1000

// maxCorrelatedGroups is the maximum number of groups returned.
const maxCorrelatedGroups = 50

// CorrelateLogsInput defines the parameters for the log_correlate tool.
type CorrelateLogsInput struct {
	Paths             []string `json:"paths"                          jsonschema:"Array of log file paths (2-10 files)"`
	CorrelationField  string   `json:"correlation_field,omitempty"    jsonschema:"Field name for correlation"`
	TimeWindowSeconds int      `json:"time_window_seconds,omitempty"  jsonschema:"Max time window in seconds for grouping"`
}

// FileAnalysis records how many entries were parsed from each file.
type FileAnalysis struct {
	Path          string `json:"path"`
	EntriesParsed int    `json:"entries_parsed"`
}

// CorrelateLogsOutput is the structured result of the log_correlate tool.
type CorrelateLogsOutput struct {
	CorrelatedGroups []types.CorrelatedGroup `json:"correlated_groups"`
	TotalGroups      int                     `json:"total_groups"`
	GroupsReturned   int                     `json:"groups_returned"`
	FilesAnalyzed    []FileAnalysis          `json:"files_analyzed"`
}

// correlationAccumulator collects events for a single correlation value.
type correlationAccumulator struct {
	events []types.CorrelatedEvent
	files  map[string]bool
}

// RunCorrelateLogs correlates events across multiple log files using shared identifiers.
func RunCorrelateLogs(input CorrelateLogsInput) (CorrelateLogsOutput, error) {
	// Apply defaults and clamp.
	input.CorrelationField = DefaultString(input.CorrelationField, "request_id")
	input.TimeWindowSeconds = DefaultInt(input.TimeWindowSeconds, 60)
	input.TimeWindowSeconds = ClampInt(input.TimeWindowSeconds, 1, 3600)

	// Validate path count.
	if len(input.Paths) < 2 {
		return CorrelateLogsOutput{}, fmt.Errorf("log_correlate: VALIDATION_ERROR: at least 2 file paths required, got %d", len(input.Paths))
	}
	if len(input.Paths) > 10 {
		return CorrelateLogsOutput{}, fmt.Errorf("log_correlate: VALIDATION_ERROR: at most 10 file paths allowed, got %d", len(input.Paths))
	}

	// Validate file access for each path.
	for _, p := range input.Paths {
		if err := CheckFileAccess(p); err != nil {
			return CorrelateLogsOutput{}, fmt.Errorf("log_correlate: %w", err)
		}
	}

	// Build regex for extracting correlation value from message text.
	// Pattern: correlationField[=: ]["']?(\S+)
	escapedField := regexp.QuoteMeta(input.CorrelationField)
	msgRe := regexp.MustCompile(escapedField + `[=: ]["']?(\S+)`)

	// Groups keyed by correlation value.
	groups := make(map[string]*correlationAccumulator)
	filesAnalyzed := make([]FileAnalysis, len(input.Paths))

	// Process each file sequentially.
	for fileIdx, path := range input.Paths {
		// Sample lines and auto-detect format.
		sample, err := SampleLines(path, sampleLineCount)
		if err != nil {
			return CorrelateLogsOutput{}, fmt.Errorf("log_correlate: %w", err)
		}
		_, parser := parsers.AutoDetectWithHint(sample, "")

		entriesParsed := 0

		if parser == nil {
			filesAnalyzed[fileIdx] = FileAnalysis{Path: path, EntriesParsed: 0}
			continue
		}

		// Stream file page-by-page.
		startLine := 1
		for {
			result, readErr := fileutil.ReadLines(path, startLine, correlatePageSize)
			if readErr != nil {
				return CorrelateLogsOutput{}, fmt.Errorf("log_correlate: read %s at line %d: %w", path, startLine, readErr)
			}

			for _, lr := range result.Lines {
				entry := parser.Parse(lr.Text)
				if entry == nil {
					continue
				}
				entry.LineNumber = lr.LineNumber
				entriesParsed++

				// Extract correlation value.
				corrVal := extractCorrelationValue(entry, input.CorrelationField, msgRe)
				if corrVal == "" {
					continue
				}

				// Build correlated event.
				ev := types.CorrelatedEvent{
					File:       path,
					LineNumber: entry.LineNumber,
					Level:      entry.Level,
					Source:     entry.Source,
					Message:    entry.Message,
				}
				if entry.Timestamp != nil {
					ev.Timestamp = *entry.Timestamp
				}

				acc, exists := groups[corrVal]
				if !exists {
					acc = &correlationAccumulator{
						files: make(map[string]bool),
					}
					groups[corrVal] = acc
				}
				acc.events = append(acc.events, ev)
				acc.files[path] = true
			}

			if !result.HasMore || len(result.Lines) == 0 {
				break
			}
			startLine += len(result.Lines)
		}

		filesAnalyzed[fileIdx] = FileAnalysis{Path: path, EntriesParsed: entriesParsed}
	}

	// Build correlated groups from accumulated data.
	timeWindow := time.Duration(input.TimeWindowSeconds) * time.Second

	var resultGroups []types.CorrelatedGroup
	for corrVal, acc := range groups {
		// Filter: must span >= 2 different files.
		if len(acc.files) < 2 {
			continue
		}

		// Sort events within group by timestamp (lexicographic).
		sort.Slice(acc.events, func(i, j int) bool {
			return acc.events[i].Timestamp < acc.events[j].Timestamp
		})

		// Compute time span.
		spanMs := computeTimeSpanMs(acc.events)

		// Filter: keep only groups where time span <= TimeWindowSeconds.
		if spanMs >= 0 && time.Duration(spanMs)*time.Millisecond > timeWindow {
			continue
		}

		// Collect unique files involved.
		filesInvolved := make([]string, 0, len(acc.files))
		for f := range acc.files {
			filesInvolved = append(filesInvolved, f)
		}
		sort.Strings(filesInvolved)

		resultGroups = append(resultGroups, types.CorrelatedGroup{
			CorrelationID:    corrVal,
			CorrelationField: input.CorrelationField,
			FilesInvolved:    filesInvolved,
			TimeSpanMs:       spanMs,
			Events:           acc.events,
		})
	}

	// Sort groups by number of events descending, then by correlation ID for determinism.
	sort.Slice(resultGroups, func(i, j int) bool {
		if len(resultGroups[i].Events) != len(resultGroups[j].Events) {
			return len(resultGroups[i].Events) > len(resultGroups[j].Events)
		}
		return resultGroups[i].CorrelationID < resultGroups[j].CorrelationID
	})

	totalGroups := len(resultGroups)

	// Limit to maxCorrelatedGroups.
	if len(resultGroups) > maxCorrelatedGroups {
		resultGroups = resultGroups[:maxCorrelatedGroups]
	}

	// Ensure non-nil slice for JSON marshaling.
	if resultGroups == nil {
		resultGroups = []types.CorrelatedGroup{}
	}

	return CorrelateLogsOutput{
		CorrelatedGroups: resultGroups,
		TotalGroups:      totalGroups,
		GroupsReturned:   len(resultGroups),
		FilesAnalyzed:    filesAnalyzed,
	}, nil
}

// extractCorrelationValue extracts the correlation value from a parsed entry.
// It first checks extra_fields, then falls back to regex matching on the message.
func extractCorrelationValue(entry *types.ParsedLogEntry, field string, msgRe *regexp.Regexp) string {
	// Check extra_fields first.
	if entry.ExtraFields != nil {
		if val, ok := entry.ExtraFields[field]; ok {
			s := fmt.Sprintf("%v", val)
			if s != "" {
				return s
			}
		}
	}

	// Fall back to regex search in message.
	matches := msgRe.FindStringSubmatch(entry.Message)
	if len(matches) >= 2 {
		// Strip trailing quotes if present.
		val := matches[1]
		val = strings.TrimRight(val, `"'`)
		if val != "" {
			return val
		}
	}

	return ""
}

// computeTimeSpanMs calculates the time span in milliseconds between the first
// and last events in a sorted event list. Returns 0 if timestamps are missing
// or unparseable.
func computeTimeSpanMs(events []types.CorrelatedEvent) int64 {
	if len(events) == 0 {
		return 0
	}

	first := events[0].Timestamp
	last := events[len(events)-1].Timestamp
	if first == "" || last == "" {
		return 0
	}

	t1, err1 := parseTimestamp(first)
	t2, err2 := parseTimestamp(last)
	if err1 != nil || err2 != nil {
		return 0
	}

	d := t2.Sub(t1)
	if d < 0 {
		return 0
	}
	return d.Milliseconds()
}
