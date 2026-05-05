package parsers

import (
	"regexp"
	"strings"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// Logback/SLF4J has two common pattern orderings:
//   Pattern A (default encoder): 14:04:07.123 [main] INFO com.example.MyClass - message
//   Pattern B (with date):       2025-05-05 14:04:07.123 INFO [main] com.example.MyClass - message
//   Pattern C (date+thread first): 2025-05-05 14:04:07.123 [main] INFO com.example.MyClass - message

var (
	// Pattern A: time-only, [thread] before level
	logbackReA = regexp.MustCompile(
		`^(\d{2}:\d{2}:\d{2}[.,]\d{3})\s+\[([^\]]+)\]\s+(TRACE|DEBUG|INFO|WARN|ERROR)\s+(\S+)\s+-\s+(.*)$`)

	// Pattern B: date+time, level before [thread]
	logbackReB = regexp.MustCompile(
		`^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[.,]\d{3})\s+(TRACE|DEBUG|INFO|WARN|ERROR)\s+\[([^\]]+)\]\s+(\S+)\s+-\s+(.*)$`)

	// Pattern C: date+time, [thread] before level
	logbackReC = regexp.MustCompile(
		`^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[.,]\d{3})\s+\[([^\]]+)\]\s+(TRACE|DEBUG|INFO|WARN|ERROR)\s+(\S+)\s+-\s+(.*)$`)
)

// JavaLogbackParser handles Java Logback/SLF4J/Log4j2 default console patterns.
type JavaLogbackParser struct{}

// NewJavaLogbackParser creates a Java Logback parser.
func NewJavaLogbackParser() *JavaLogbackParser {
	return &JavaLogbackParser{}
}

func (p *JavaLogbackParser) Name() string {
	return string(types.LogFormatJavaLogback)
}

func (p *JavaLogbackParser) Parse(line string) *types.ParsedLogEntry {
	// Try pattern A: time [thread] LEVEL logger - message
	if m := logbackReA.FindStringSubmatch(line); m != nil {
		return p.buildEntry(m[1], m[3], strings.TrimSpace(m[2]), m[4], m[5], line)
	}
	// Try pattern B: datetime LEVEL [thread] logger - message
	if m := logbackReB.FindStringSubmatch(line); m != nil {
		return p.buildEntry(m[1], m[2], strings.TrimSpace(m[3]), m[4], m[5], line)
	}
	// Try pattern C: datetime [thread] LEVEL logger - message
	if m := logbackReC.FindStringSubmatch(line); m != nil {
		return p.buildEntry(m[1], m[3], strings.TrimSpace(m[2]), m[4], m[5], line)
	}
	return nil
}

func (p *JavaLogbackParser) buildEntry(ts, level, thread, logger, msg, raw string) *types.ParsedLogEntry {
	lvl := javaLogLevel(level)
	return &types.ParsedLogEntry{
		Timestamp: &ts,
		Level:     &lvl,
		Source:    &logger,
		Message:   msg,
		Raw:       raw,
		ExtraFields: map[string]interface{}{
			"thread": thread,
		},
	}
}

func (p *JavaLogbackParser) Detect(lines []string) float64 {
	matches := 0
	for _, line := range lines {
		if logbackReA.MatchString(line) || logbackReB.MatchString(line) || logbackReC.MatchString(line) {
			matches++
		}
	}
	return float64(matches) / float64(len(lines))
}

func javaLogLevel(s string) types.LogLevel {
	switch s {
	case "TRACE":
		return types.LogLevelTrace
	case "DEBUG":
		return types.LogLevelDebug
	case "INFO":
		return types.LogLevelInfo
	case "WARN":
		return types.LogLevelWarn
	case "ERROR":
		return types.LogLevelError
	default:
		return types.LogLevelInfo
	}
}
