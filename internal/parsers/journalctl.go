package parsers

import (
	"regexp"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// journalISOre matches journalctl --output=short-iso lines.
// Example: 2025-01-15T10:00:01+0000 myhost sshd[1234]: Accepted publickey for user
var journalISOre = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?[+-]\d{4})\s+(\S+)\s+([^\[ :]+)(?:\[(\d+)\])?:\s+(.*)$`)

// JournalISOParser handles journalctl --output=short-iso format.
type JournalISOParser struct{}

// NewJournalISOParser creates a journalctl short-iso parser.
func NewJournalISOParser() *JournalISOParser {
	return &JournalISOParser{}
}

func (p *JournalISOParser) Name() string {
	return string(types.LogFormatJournalISO)
}

func (p *JournalISOParser) Parse(line string) *types.ParsedLogEntry {
	m := journalISOre.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	ts := m[1]
	source := m[3]

	entry := &types.ParsedLogEntry{
		Timestamp: &ts,
		Source:    &source,
		Message:   m[5],
		Raw:       line,
	}

	extras := map[string]interface{}{
		"hostname": m[2],
	}
	if m[4] != "" {
		extras["pid"] = m[4]
	}
	entry.ExtraFields = extras

	return entry
}

func (p *JournalISOParser) Detect(lines []string) float64 {
	matches := 0
	for _, line := range lines {
		if journalISOre.MatchString(line) {
			matches++
		}
	}
	return float64(matches) / float64(len(lines))
}
