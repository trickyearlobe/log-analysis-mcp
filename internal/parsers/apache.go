package parsers

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// Regexes compiled once at package init time.
var (
	apacheCombinedRe = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+"([A-Z]+)\s+(\S+)\s+(\S+)"\s+(\d{3})\s+(\S+)(?:\s+"([^"]*)")?(?:\s+"([^"]*)")?$`)
	apacheCommonRe   = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+"([A-Z]+)\s+(\S+)\s+(\S+)"\s+(\d{3})\s+(\S+)$`)
)

// statusToLevel maps an HTTP status code to a normalized log level.
func statusToLevel(status int) types.LogLevel {
	switch {
	case status >= 500:
		return types.LogLevelError
	case status >= 400:
		return types.LogLevelWarn
	default:
		return types.LogLevelInfo
	}
}

// ApacheCombinedParser parses Apache/Nginx Combined Log Format lines.
type ApacheCombinedParser struct{}

// NewApacheCombinedParser returns a new ApacheCombinedParser instance.
func NewApacheCombinedParser() *ApacheCombinedParser {
	return &ApacheCombinedParser{}
}

// Name returns the format identifier for this parser.
func (p *ApacheCombinedParser) Name() string {
	return "apache-combined"
}

// Parse attempts to parse a single Combined Log Format line. Returns nil if the line does not match.
func (p *ApacheCombinedParser) Parse(line string) *types.ParsedLogEntry {
	m := apacheCombinedRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	remoteHost := m[1]
	identity := m[2]
	user := m[3]
	timestamp := m[4]
	method := m[5]
	path := m[6]
	protocol := m[7]
	status := m[8]
	bytes := m[9]
	referer := m[10]
	userAgent := m[11]

	if bytes == "-" {
		bytes = "0"
	}

	statusCode, err := strconv.Atoi(status)
	if err != nil {
		return nil
	}
	level := statusToLevel(statusCode)

	source := remoteHost
	message := fmt.Sprintf("%s %s %s", method, path, protocol)

	entry := &types.ParsedLogEntry{
		Timestamp:   &timestamp,
		Level:       &level,
		Source:      &source,
		Message:     message,
		Raw:         line,
		ExtraFields: make(map[string]interface{}),
	}

	entry.ExtraFields["remote_host"] = remoteHost
	if identity != "-" {
		entry.ExtraFields["identity"] = identity
	}
	if user != "-" {
		entry.ExtraFields["user"] = user
	}
	entry.ExtraFields["method"] = method
	entry.ExtraFields["path"] = path
	entry.ExtraFields["protocol"] = protocol
	entry.ExtraFields["status"] = status
	entry.ExtraFields["bytes"] = bytes
	if referer != "-" {
		entry.ExtraFields["referer"] = referer
	}
	entry.ExtraFields["user_agent"] = userAgent

	return entry
}

// Detect returns the fraction of lines that successfully parse as Combined Log Format.
func (p *ApacheCombinedParser) Detect(lines []string) float64 {
	if len(lines) == 0 {
		return 0.0
	}

	successes := 0
	for _, line := range lines {
		if p.Parse(line) != nil {
			successes++
		}
	}

	return float64(successes) / float64(len(lines))
}

// ApacheCommonParser parses Apache/Nginx Common Log Format lines.
type ApacheCommonParser struct{}

// NewApacheCommonParser returns a new ApacheCommonParser instance.
func NewApacheCommonParser() *ApacheCommonParser {
	return &ApacheCommonParser{}
}

// Name returns the format identifier for this parser.
func (p *ApacheCommonParser) Name() string {
	return "apache-common"
}

// Parse attempts to parse a single Common Log Format line. Returns nil if the line does not match.
func (p *ApacheCommonParser) Parse(line string) *types.ParsedLogEntry {
	m := apacheCommonRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	remoteHost := m[1]
	identity := m[2]
	user := m[3]
	timestamp := m[4]
	method := m[5]
	path := m[6]
	protocol := m[7]
	status := m[8]
	bytes := m[9]

	if bytes == "-" {
		bytes = "0"
	}

	statusCode, err := strconv.Atoi(status)
	if err != nil {
		return nil
	}
	level := statusToLevel(statusCode)

	source := remoteHost
	message := fmt.Sprintf("%s %s %s", method, path, protocol)

	entry := &types.ParsedLogEntry{
		Timestamp:   &timestamp,
		Level:       &level,
		Source:      &source,
		Message:     message,
		Raw:         line,
		ExtraFields: make(map[string]interface{}),
	}

	entry.ExtraFields["remote_host"] = remoteHost
	if identity != "-" {
		entry.ExtraFields["identity"] = identity
	}
	if user != "-" {
		entry.ExtraFields["user"] = user
	}
	entry.ExtraFields["method"] = method
	entry.ExtraFields["path"] = path
	entry.ExtraFields["protocol"] = protocol
	entry.ExtraFields["status"] = status
	entry.ExtraFields["bytes"] = bytes

	return entry
}

// Detect returns the fraction of lines that successfully parse as Common Log Format.
func (p *ApacheCommonParser) Detect(lines []string) float64 {
	if len(lines) == 0 {
		return 0.0
	}

	successes := 0
	for _, line := range lines {
		if p.Parse(line) != nil {
			successes++
		}
	}

	return float64(successes) / float64(len(lines))
}
