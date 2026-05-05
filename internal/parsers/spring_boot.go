package parsers

import (
	"regexp"
	"strings"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// Spring Boot log formats are distinguished by the "PID ---" separator.
//   2.x: 2025-05-05 14:04:07.123  INFO 12345 --- [           main] c.e.MyClass              : message
//   3.x: 2025-05-05T14:04:07.123+01:00  INFO 12345 --- [myapp] [           main] c.e.MyClass              : message

var (
	// Spring Boot 3.x: ISO timestamp, LEVEL, PID, ---, [appname], [thread], logger, :, message
	springBoot3Re = regexp.MustCompile(
		`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}\S*)\s+(TRACE|DEBUG|INFO|WARN|ERROR)\s+(\d+)\s+---\s+\[([^\]]*)\]\s+\[([^\]]*)\]\s+(\S+)\s+:\s+(.*)$`)

	// Spring Boot 2.x: date time, LEVEL, PID, ---, [thread], logger, :, message
	springBoot2Re = regexp.MustCompile(
		`^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[.,]\d{3}\S*)\s+(TRACE|DEBUG|INFO|WARN|ERROR)\s+(\d+)\s+---\s+\[([^\]]*)\]\s+(\S+)\s+:\s+(.*)$`)
)

// SpringBootParser handles Spring Boot 2.x and 3.x default log patterns.
type SpringBootParser struct{}

// NewSpringBootParser creates a Spring Boot log parser.
func NewSpringBootParser() *SpringBootParser {
	return &SpringBootParser{}
}

func (p *SpringBootParser) Name() string {
	return string(types.LogFormatSpringBoot)
}

func (p *SpringBootParser) Parse(line string) *types.ParsedLogEntry {
	// Try 3.x first (has app name bracket)
	if m := springBoot3Re.FindStringSubmatch(line); m != nil {
		lvl := javaLogLevel(m[2])
		thread := strings.TrimSpace(m[5])
		logger := m[6]
		return &types.ParsedLogEntry{
			Timestamp: &m[1],
			Level:     &lvl,
			Source:    &logger,
			Message:   m[7],
			Raw:       line,
			ExtraFields: map[string]interface{}{
				"pid":     m[3],
				"app":     strings.TrimSpace(m[4]),
				"thread":  thread,
			},
		}
	}
	// Fall back to 2.x
	if m := springBoot2Re.FindStringSubmatch(line); m != nil {
		lvl := javaLogLevel(m[2])
		thread := strings.TrimSpace(m[4])
		logger := m[5]
		return &types.ParsedLogEntry{
			Timestamp: &m[1],
			Level:     &lvl,
			Source:    &logger,
			Message:   m[6],
			Raw:       line,
			ExtraFields: map[string]interface{}{
				"pid":    m[3],
				"thread": thread,
			},
		}
	}
	return nil
}

func (p *SpringBootParser) Detect(lines []string) float64 {
	matches := 0
	for _, line := range lines {
		if springBoot3Re.MatchString(line) || springBoot2Re.MatchString(line) {
			matches++
		}
	}
	return float64(matches) / float64(len(lines))
}
