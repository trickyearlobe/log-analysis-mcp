package parsers

import (
	"regexp"
	"strings"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// criPrefixRe matches the Kubernetes Container Runtime Interface log line prefix:
//   <ISO-timestamp> (stdout|stderr) (F|P) <rest>
// where F = full line, P = partial (continuation).
var criPrefixRe = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+\S*)\s+(stdout|stderr)\s+([FP])\s+(.*)$`)

// innerParsers are tried in order against the CRI-stripped inner content.
var innerParsers = []Parser{
	NewJSONParser(),
	NewJavaLogbackBracketParser(),
	NewSpringBootParser(),
	NewGoLogrusBracketParser(),
	NewJavaLogbackParser(),
}

// KubernetesCRIParser handles Kubernetes container log lines wrapped with CRI metadata.
type KubernetesCRIParser struct{}

// NewKubernetesCRIParser creates a Kubernetes CRI format parser.
func NewKubernetesCRIParser() *KubernetesCRIParser {
	return &KubernetesCRIParser{}
}

func (p *KubernetesCRIParser) Name() string {
	return string(types.LogFormatKubernetesCRI)
}

func (p *KubernetesCRIParser) Parse(line string) *types.ParsedLogEntry {
	m := criPrefixRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	criTimestamp := m[1]
	stream := m[2]
	inner := m[4]

	// Try inner parsers for structured extraction
	for _, ip := range innerParsers {
		if entry := ip.Parse(inner); entry != nil {
			// Use CRI timestamp (nanosecond precision) over inner timestamp
			entry.Timestamp = &criTimestamp
			entry.Raw = line
			if entry.ExtraFields == nil {
				entry.ExtraFields = make(map[string]interface{})
			}
			entry.ExtraFields["stream"] = stream
			entry.ExtraFields["inner_format"] = ip.Name()
			return entry
		}
	}

	// Fallback: return entry with CRI timestamp and raw inner content as message
	lvl := inferLevelFromInner(inner, stream)
	return &types.ParsedLogEntry{
		Timestamp: &criTimestamp,
		Level:     &lvl,
		Message:   inner,
		Raw:       line,
		ExtraFields: map[string]interface{}{
			"stream": stream,
		},
	}
}

func (p *KubernetesCRIParser) Detect(lines []string) float64 {
	matches := 0
	for _, line := range lines {
		if criPrefixRe.MatchString(line) {
			matches++
		}
	}
	return float64(matches) / float64(len(lines))
}

// inferLevelFromInner attempts basic level detection from inner content and stream.
func inferLevelFromInner(inner, stream string) types.LogLevel {
	upper := strings.ToUpper(inner)
	switch {
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "[ERROR"):
		return types.LogLevelError
	case strings.Contains(upper, "WARN") || strings.Contains(upper, "[WARN"):
		return types.LogLevelWarn
	case strings.Contains(upper, "DEBUG") || strings.Contains(upper, "[DEBUG"):
		return types.LogLevelDebug
	case stream == "stderr":
		return types.LogLevelError
	default:
		return types.LogLevelInfo
	}
}
