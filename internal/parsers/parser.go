// Package parsers implements log format detection and parsing for common log formats.
package parsers

import "github.com/trickyearlobe/log-analysis-mcp/internal/types"

// Parser defines the interface every log format parser must implement.
type Parser interface {
	// Parse attempts to parse a single log line. Returns nil if the line
	// does not match this parser's format.
	Parse(line string) *types.ParsedLogEntry

	// Detect returns a confidence score (0.0–1.0) for a sample of lines.
	Detect(lines []string) float64

	// Name returns the format identifier (e.g., "syslog-rfc3164").
	Name() string
}
