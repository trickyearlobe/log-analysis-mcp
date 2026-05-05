package parsers

import (
	"regexp"
	"strings"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// habitatSupRe matches Chef Habitat supervisor log lines.
// Example: hab-sup(MN): 2025-01-15T10:00:01.000000Z INFO hab_sup::manager: Loading service
var habitatSupRe = regexp.MustCompile(`^hab-sup\(([^)]+)\):\s+(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(DEBUG|INFO|WARN|ERROR|TRACE)\s+(\S+)\s+(.+)$`)

// HabitatSupParser handles Chef Habitat supervisor log format.
type HabitatSupParser struct{}

// NewHabitatSupParser creates a Habitat supervisor parser.
func NewHabitatSupParser() *HabitatSupParser {
	return &HabitatSupParser{}
}

func (p *HabitatSupParser) Name() string {
	return string(types.LogFormatHabitatSup)
}

func (p *HabitatSupParser) Parse(line string) *types.ParsedLogEntry {
	m := habitatSupRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	ts := m[2]
	level := habitatLevel(m[3])
	source := strings.TrimSuffix(m[4], ":")

	return &types.ParsedLogEntry{
		Timestamp: &ts,
		Level:     &level,
		Source:    &source,
		Message:   m[5],
		Raw:       line,
		ExtraFields: map[string]interface{}{
			"worker": m[1],
		},
	}
}

func (p *HabitatSupParser) Detect(lines []string) float64 {
	matches := 0
	for _, line := range lines {
		if habitatSupRe.MatchString(line) {
			matches++
		}
	}
	return float64(matches) / float64(len(lines))
}

func habitatLevel(s string) types.LogLevel {
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
