package tools

import (
	"fmt"
	"sort"
	"strings"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// timelinePageSize is the number of lines to read per streaming page.
const timelinePageSize = 1000

// TimelineInput defines the parameters for the timeline tool.
type TimelineInput struct {
	Path       string   `json:"path"        jsonschema:"required,description=Path to the log file"`
	After      string   `json:"after"       jsonschema:"description=ISO 8601 timestamp — include events after this time"`
	Before     string   `json:"before"      jsonschema:"description=ISO 8601 timestamp — include events before this time"`
	EventTypes []string `json:"event_types" jsonschema:"description=Event types to include"`
	MaxEvents  int      `json:"max_events"  jsonschema:"description=Maximum number of events to return (max 500),minimum=1,maximum=500"`
}

// TimeSpan describes the time range covered by returned events.
type TimeSpan struct {
	Start           string  `json:"start"`
	End             string  `json:"end"`
	DurationMinutes float64 `json:"duration_minutes"`
}

// TimelineOutput is the structured result of the timeline tool.
type TimelineOutput struct {
	Events     []types.TimelineEvent `json:"events"`
	TimeSpan   TimeSpan              `json:"time_span"`
	EventCount int                   `json:"event_count"`
	Truncated  bool                  `json:"truncated"`
}

// lifecycleKeywords maps event type names to their keyword triggers.
// Keywords are stored lowercase for case-insensitive matching.
var lifecycleKeywords = map[string][]string{
	"startup":    {"started", "listening on", "server ready", "boot complete"},
	"shutdown":   {"shutting down", "stopped", "graceful shutdown", "sigterm"},
	"deploy":     {"deployed", "deployment", "release", "version"},
	"restart":    {"restarting", "restarted", "respawn"},
	"crash":      {"crash", "panic", "fatal", "core dump", "segfault"},
	"connection": {"connected", "disconnected", "connection lost", "reconnect"},
}

// lifecycleOrder defines a deterministic check order so the first matching
// category wins when a message could match multiple lifecycle types.
var lifecycleOrder = []string{"startup", "shutdown", "deploy", "restart", "crash", "connection"}

// defaultEventTypes is the filter applied when the caller omits EventTypes.
var defaultEventTypes = map[string]bool{
	"ERROR":      true,
	"WARN":       true,
	"FATAL":      true,
	"startup":    true,
	"shutdown":   true,
	"deploy":     true,
	"restart":    true,
	"crash":      true,
	"connection": true,
}

// classifyEventType determines the event type for a parsed log entry.
// Lifecycle keywords take precedence over log-level classification.
func classifyEventType(entry *types.ParsedLogEntry) string {
	msgLower := strings.ToLower(entry.Message)

	for _, category := range lifecycleOrder {
		for _, keyword := range lifecycleKeywords[category] {
			if strings.Contains(msgLower, keyword) {
				return category
			}
		}
	}

	// Fall back to the log level as event type.
	if entry.Level != nil {
		return string(*entry.Level)
	}
	return "INFO"
}

// RunTimeline builds a chronological event timeline from a log file.
func RunTimeline(input TimelineInput) (TimelineOutput, error) {
	// Apply defaults and clamp.
	input.MaxEvents = DefaultInt(input.MaxEvents, 100)
	input.MaxEvents = ClampInt(input.MaxEvents, 1, 500)

	// Validate file access.
	if err := CheckFileAccess(input.Path); err != nil {
		return TimelineOutput{}, fmt.Errorf("timeline: %w", err)
	}

	// Sample lines and auto-detect format.
	sample, err := SampleLines(input.Path, sampleLineCount)
	if err != nil {
		return TimelineOutput{}, fmt.Errorf("timeline: %w", err)
	}
	_, parser := parsers.AutoDetectWithHint(sample, "")
	if parser == nil {
		return TimelineOutput{
			Events: []types.TimelineEvent{},
		}, nil
	}

	// Build event type filter set.
	typeFilter := make(map[string]bool)
	if len(input.EventTypes) > 0 {
		for _, et := range input.EventTypes {
			typeFilter[et] = true
		}
	} else {
		for k, v := range defaultEventTypes {
			typeFilter[k] = v
		}
	}

	// Stream file page-by-page, parse, classify, and filter.
	var events []types.TimelineEvent

	startLine := 1
	for {
		result, readErr := fileutil.ReadLines(input.Path, startLine, timelinePageSize)
		if readErr != nil {
			return TimelineOutput{}, fmt.Errorf("timeline: read at line %d: %w", startLine, readErr)
		}

		for _, lr := range result.Lines {
			entry := parser.Parse(lr.Text)
			if entry == nil {
				continue
			}
			entry.LineNumber = lr.LineNumber

			// Skip entries without a timestamp — can't place on timeline.
			if entry.Timestamp == nil {
				continue
			}
			ts := *entry.Timestamp

			// Apply time range filter using lexicographic string comparison.
			if input.After != "" && ts <= input.After {
				continue
			}
			if input.Before != "" && ts >= input.Before {
				continue
			}

			// Classify event type.
			eventType := classifyEventType(entry)

			// Apply event type filter.
			if !typeFilter[eventType] {
				continue
			}

			events = append(events, types.TimelineEvent{
				Timestamp:  ts,
				Type:       eventType,
				Source:     entry.Source,
				Message:    entry.Message,
				LineNumber: entry.LineNumber,
			})
		}

		if !result.HasMore || len(result.Lines) == 0 {
			break
		}
		startLine += len(result.Lines)
	}

	// Sort events chronologically by timestamp (lexicographic on ISO 8601).
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp < events[j].Timestamp
	})

	// Determine total count before truncation.
	totalCount := len(events)
	truncated := totalCount > input.MaxEvents
	if truncated {
		events = events[:input.MaxEvents]
	}

	// Ensure non-nil slice for JSON marshaling.
	if events == nil {
		events = []types.TimelineEvent{}
	}

	// Compute time span from first/last event timestamps.
	span := TimeSpan{}
	if len(events) > 0 {
		span.Start = events[0].Timestamp
		span.End = events[len(events)-1].Timestamp
		span.DurationMinutes = computeDurationMinutes(span.Start, span.End)
	}

	return TimelineOutput{
		Events:     events,
		TimeSpan:   span,
		EventCount: totalCount,
		Truncated:  truncated,
	}, nil
}

// computeDurationMinutes parses two timestamp strings and returns the duration
// between them in minutes. Returns 0 if either timestamp is unparseable.
func computeDurationMinutes(start, end string) float64 {
	t1, err1 := parseTimestamp(start)
	t2, err2 := parseTimestamp(end)
	if err1 != nil || err2 != nil {
		return 0
	}

	d := t2.Sub(t1)
	if d < 0 {
		return 0
	}

	return d.Minutes()
}
