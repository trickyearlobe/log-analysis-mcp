package parsers

import (
	"regexp"
	"strings"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// logbackBracketRe matches the bracket-style Logback pattern common in enterprise Java apps:
//   [DD-MM-YYYY HH:MM:SS.mmm] [thread] [LEVEL ] [trace-id] [logger] message
//   [DD-MM-YYYY HH:MM:SS.mmm] [thread] [LEVEL ] [logger] message (no trace)
//
// Note: level field may have trailing spaces (e.g. "INFO " padded to 5 chars).
var logbackBracketRe = regexp.MustCompile(
	`^\[(\d{2}-\d{2}-\d{4} \d{2}:\d{2}:\d{2}\.\d{3})\]\s+\[([^\]]+)\]\s+\[([A-Z]+)\s*\]\s+(?:\[([0-9a-f]+)\]\s+)?\[([^\]]+)\]\s+(.+)$`)

// JavaLogbackBracketParser handles bracket-style Logback patterns where all fields are in [brackets].
type JavaLogbackBracketParser struct{}

// NewJavaLogbackBracketParser creates a bracket-style Logback parser.
func NewJavaLogbackBracketParser() *JavaLogbackBracketParser {
	return &JavaLogbackBracketParser{}
}

func (p *JavaLogbackBracketParser) Name() string {
	return string(types.LogFormatJavaLogbackBracket)
}

func (p *JavaLogbackBracketParser) Parse(line string) *types.ParsedLogEntry {
	m := logbackBracketRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	ts := m[1]
	thread := strings.TrimSpace(m[2])
	lvl := javaLogLevel(strings.TrimSpace(m[3]))
	traceID := m[4]
	logger := strings.TrimSpace(m[5])
	msg := strings.TrimSpace(m[6])

	entry := &types.ParsedLogEntry{
		Timestamp:   &ts,
		Level:       &lvl,
		Source:      &logger,
		Message:     msg,
		Raw:         line,
		ExtraFields: map[string]interface{}{
			"thread": thread,
		},
	}
	if traceID != "" {
		entry.ExtraFields["trace_id"] = traceID
	}
	return entry
}

func (p *JavaLogbackBracketParser) Detect(lines []string) float64 {
	matches := 0
	for _, line := range lines {
		if logbackBracketRe.MatchString(line) {
			matches++
		}
	}
	return float64(matches) / float64(len(lines))
}
