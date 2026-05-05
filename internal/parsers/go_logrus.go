package parsers

import (
	"regexp"
	"strings"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// goLogrusBracketRe matches Go logrus-style bracket log lines used by Calico, containerd, etc.
// Example: 2025-12-09 11:49:52.939 [ERROR][7059] plugin.go 162: Final result of CNI ADD was an error.
var goLogrusBracketRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+)\s+\[([A-Z]+)\]\[(\d+)\]\s+(\S+)\s+(\d+):\s+(.+)$`)

// GoLogrusBracketParser handles Go logrus bracket format used by Calico and similar projects.
type GoLogrusBracketParser struct{}

// NewGoLogrusBracketParser creates a Go logrus bracket parser.
func NewGoLogrusBracketParser() *GoLogrusBracketParser {
	return &GoLogrusBracketParser{}
}

func (p *GoLogrusBracketParser) Name() string {
	return string(types.LogFormatGoLogrusBracket)
}

func (p *GoLogrusBracketParser) Parse(line string) *types.ParsedLogEntry {
	m := goLogrusBracketRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	ts := m[1]
	level := goLogrusLevel(m[2])
	source := m[4]

	entry := &types.ParsedLogEntry{
		Timestamp: &ts,
		Level:     &level,
		Source:    &source,
		Message:   m[6],
		Raw:       line,
		ExtraFields: map[string]interface{}{
			"pid":         m[3],
			"line_number": m[5],
		},
	}

	// Extract key=value pairs from message
	extractKeyValues(entry)

	return entry
}

func (p *GoLogrusBracketParser) Detect(lines []string) float64 {
	matches := 0
	for _, line := range lines {
		if goLogrusBracketRe.MatchString(line) {
			matches++
		}
	}
	return float64(matches) / float64(len(lines))
}

func goLogrusLevel(s string) types.LogLevel {
	switch s {
	case "TRACE":
		return types.LogLevelTrace
	case "DEBUG":
		return types.LogLevelDebug
	case "INFO":
		return types.LogLevelInfo
	case "WARN", "WARNING":
		return types.LogLevelWarn
	case "ERROR":
		return types.LogLevelError
	case "FATAL":
		return types.LogLevelFatal
	case "CRITICAL":
		return types.LogLevelCritical
	default:
		return types.LogLevelInfo
	}
}

// kvPairRe matches key="value" or key=value patterns in log messages.
var kvPairRe = regexp.MustCompile(`(\w+)=(?:"([^"]*)"|([\S]+))`)

func extractKeyValues(entry *types.ParsedLogEntry) {
	matches := kvPairRe.FindAllStringSubmatch(entry.Message, 10)
	if len(matches) == 0 {
		return
	}
	for _, m := range matches {
		key := m[1]
		value := m[2]
		if value == "" {
			value = m[3]
		}
		// Skip very long values (container IDs etc.) to keep output clean
		if len(value) > 100 {
			value = value[:100] + "..."
		}
		entry.ExtraFields[key] = strings.TrimRight(value, ",")
	}
}
