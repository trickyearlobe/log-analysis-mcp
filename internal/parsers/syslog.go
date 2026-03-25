package parsers

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// Regexes compiled once at package init time.
var (
	rfc3164Re = regexp.MustCompile(`^(?:<(\d{1,3})>)?(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(\S+?)(?:\[(\d+)\])?:\s+(.*)$`)
	rfc5424Re = regexp.MustCompile(`^<(\d{1,3})>(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(?:-|(\[(?:[^\]]*)\](?:\s*\[(?:[^\]]*)\])*))\s*(.*)$`)
)

// facilityNames maps syslog facility codes (0-23) to human-readable names.
var facilityNames = [24]string{
	"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news",
	"uucp", "cron", "authpriv", "ftp", "ntp", "audit", "alert", "clock",
	"local0", "local1", "local2", "local3", "local4", "local5", "local6", "local7",
}

// severityNames maps syslog severity codes (0-7) to human-readable names.
var severityNames = [8]string{
	"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug",
}

// severityToLevel maps syslog severity codes (0-7) to normalized LogLevel values.
var severityToLevel = [8]types.LogLevel{
	types.LogLevelFatal,    // 0 = emerg
	types.LogLevelFatal,    // 1 = alert
	types.LogLevelCritical, // 2 = crit
	types.LogLevelError,    // 3 = err
	types.LogLevelWarn,     // 4 = warning
	types.LogLevelInfo,     // 5 = notice
	types.LogLevelInfo,     // 6 = info
	types.LogLevelDebug,    // 7 = debug
}

// SyslogRFC3164Parser parses BSD-style syslog messages (RFC 3164).
type SyslogRFC3164Parser struct{}

// NewSyslogRFC3164Parser returns a new RFC 3164 syslog parser.
func NewSyslogRFC3164Parser() *SyslogRFC3164Parser {
	return &SyslogRFC3164Parser{}
}

// Name returns the format identifier for this parser.
func (p *SyslogRFC3164Parser) Name() string {
	return string(types.LogFormatSyslogRFC3164)
}

// Parse attempts to parse a single RFC 3164 syslog line.
// Returns nil if the line does not match.
func (p *SyslogRFC3164Parser) Parse(line string) *types.ParsedLogEntry {
	m := rfc3164Re.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	// m[1]=priority, m[2]=timestamp, m[3]=hostname, m[4]=app-name, m[5]=procid, m[6]=message
	ts := m[2]
	source := m[4]

	entry := &types.ParsedLogEntry{
		Timestamp: &ts,
		Source:    &source,
		Message:   m[6],
		Raw:       line,
	}

	extra := make(map[string]interface{})
	extra["hostname"] = m[3]

	if m[5] != "" {
		extra["proc_id"] = m[5]
	}

	if m[1] != "" {
		pri, err := strconv.Atoi(m[1])
		if err == nil {
			facility := pri / 8
			severity := pri % 8
			extra["priority"] = pri
			extra["severity"] = severityName(severity)
			if facility >= 0 && facility < len(facilityNames) {
				extra["facility"] = facilityNames[facility]
			} else {
				extra["facility"] = fmt.Sprintf("unknown(%d)", facility)
			}
			level := severityToLevel[severity]
			entry.Level = &level
		}
	}

	entry.ExtraFields = extra
	return entry
}

// Detect returns the fraction of sample lines that match RFC 3164 format.
func (p *SyslogRFC3164Parser) Detect(lines []string) float64 {
	if len(lines) == 0 {
		return 0.0
	}
	matched := 0
	for _, line := range lines {
		if p.Parse(line) != nil {
			matched++
		}
	}
	return float64(matched) / float64(len(lines))
}

// SyslogRFC5424Parser parses IETF syslog messages (RFC 5424).
type SyslogRFC5424Parser struct{}

// NewSyslogRFC5424Parser returns a new RFC 5424 syslog parser.
func NewSyslogRFC5424Parser() *SyslogRFC5424Parser {
	return &SyslogRFC5424Parser{}
}

// Name returns the format identifier for this parser.
func (p *SyslogRFC5424Parser) Name() string {
	return string(types.LogFormatSyslogRFC5424)
}

// Parse attempts to parse a single RFC 5424 syslog line.
// Returns nil if the line does not match.
func (p *SyslogRFC5424Parser) Parse(line string) *types.ParsedLogEntry {
	m := rfc5424Re.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	// m[1]=priority, m[2]=version, m[3]=timestamp, m[4]=hostname,
	// m[5]=app-name, m[6]=procid, m[7]=msgid, m[8]=structured-data (or ""), m[9]=message
	ts := m[3]
	source := m[5]

	entry := &types.ParsedLogEntry{
		Timestamp: &ts,
		Source:    &source,
		Message:   m[9],
		Raw:       line,
	}

	extra := make(map[string]interface{})
	extra["hostname"] = m[4]
	extra["version"] = m[2]

	if m[6] != "-" {
		extra["proc_id"] = m[6]
	}
	if m[7] != "-" {
		extra["msg_id"] = m[7]
	}
	if m[8] != "" {
		// Strip outer brackets from the captured structured data
		sd := m[8]
		if len(sd) >= 2 && sd[0] == '[' && sd[len(sd)-1] == ']' {
			sd = sd[1 : len(sd)-1]
		}
		extra["structured_data"] = sd
	}

	pri, err := strconv.Atoi(m[1])
	if err == nil {
		facility := pri / 8
		severity := pri % 8
		extra["priority"] = pri
		extra["severity"] = severityName(severity)
		if facility >= 0 && facility < len(facilityNames) {
			extra["facility"] = facilityNames[facility]
		} else {
			extra["facility"] = fmt.Sprintf("unknown(%d)", facility)
		}
		level := severityToLevel[severity]
		entry.Level = &level
	}

	entry.ExtraFields = extra
	return entry
}

// Detect returns the fraction of sample lines that match RFC 5424 format.
func (p *SyslogRFC5424Parser) Detect(lines []string) float64 {
	if len(lines) == 0 {
		return 0.0
	}
	matched := 0
	for _, line := range lines {
		if p.Parse(line) != nil {
			matched++
		}
	}
	return float64(matched) / float64(len(lines))
}

// severityName returns the human-readable name for a syslog severity code.
func severityName(severity int) string {
	if severity >= 0 && severity < len(severityNames) {
		return severityNames[severity]
	}
	return fmt.Sprintf("unknown(%d)", severity)
}
