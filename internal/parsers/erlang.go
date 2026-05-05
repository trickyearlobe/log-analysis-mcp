package parsers

import (
	"regexp"
	"strings"
	"time"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// erlangSASLRe matches Erlang/OTP SASL report headers.
// Example: =ERROR REPORT==== 15-Jan-2025::10:00:01 ===
var erlangSASLRe = regexp.MustCompile(`^=([A-Z][A-Z ]+)=+\s+(\d{1,2}-[A-Za-z]{3}-\d{4})::(\d{2}:\d{2}:\d{2})\s+===\s*$`)

// ErlangSASLParser handles Erlang/OTP SASL report header lines.
type ErlangSASLParser struct{}

// NewErlangSASLParser creates an Erlang/OTP SASL parser.
func NewErlangSASLParser() *ErlangSASLParser {
	return &ErlangSASLParser{}
}

func (p *ErlangSASLParser) Name() string {
	return string(types.LogFormatErlangSASL)
}

func (p *ErlangSASLParser) Parse(line string) *types.ParsedLogEntry {
	m := erlangSASLRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	reportType := strings.TrimSpace(m[1])
	dateStr := m[2]
	timeStr := m[3]

	ts := formatErlangTimestamp(dateStr, timeStr)
	level := erlangReportLevel(reportType)

	return &types.ParsedLogEntry{
		Timestamp: &ts,
		Level:     &level,
		Message:   reportType,
		Raw:       line,
	}
}

// Detect returns a confidence score. SASL logs have few header lines among
// many continuation lines, so we use a lower threshold: if any line matches,
// return at least 0.6 confidence.
func (p *ErlangSASLParser) Detect(lines []string) float64 {
	matches := 0
	for _, line := range lines {
		if erlangSASLRe.MatchString(line) {
			matches++
		}
	}
	if matches == 0 {
		return 0
	}
	// Even a single header line in the sample indicates SASL format
	score := float64(matches) / float64(len(lines))
	if score < 0.6 {
		return 0.6
	}
	return score
}

func formatErlangTimestamp(date, t string) string {
	combined := date + "::" + t
	parsed, err := time.Parse("2-Jan-2006::15:04:05", combined)
	if err != nil {
		return date + "T" + t
	}
	return parsed.Format(time.RFC3339)
}

func erlangReportLevel(reportType string) types.LogLevel {
	switch reportType {
	case "ERROR REPORT":
		return types.LogLevelError
	case "CRASH REPORT":
		return types.LogLevelFatal
	case "SUPERVISOR REPORT":
		return types.LogLevelWarn
	case "WARNING REPORT":
		return types.LogLevelWarn
	case "PROGRESS REPORT":
		return types.LogLevelInfo
	default:
		return types.LogLevelInfo
	}
}
