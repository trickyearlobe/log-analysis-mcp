// Package parsers implements log format detection and parsing for common log formats.
package parsers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// fieldMapping maps variant field names to their canonical standard names.
var fieldMapping = map[string]string{
	// timestamp variants
	"ts":         "timestamp",
	"time":       "timestamp",
	"timestamp":  "timestamp",
	"@timestamp": "timestamp",
	"date":       "timestamp",
	"datetime":   "timestamp",
	"t":          "timestamp",
	// level variants
	"level":     "level",
	"severity":  "level",
	"log_level": "level",
	"loglevel":  "level",
	"lvl":       "level",
	"priority":  "level",
	// message variants
	"msg":     "message",
	"message": "message",
	"log":     "message",
	"text":    "message",
	"body":    "message",
	// source variants
	"source":    "source",
	"logger":    "source",
	"component": "source",
	"module":    "source",
	"name":      "source",
	"service":   "source",
}

// levelMapping maps variant level values (lowercased) to normalized LogLevel constants.
var levelMapping = map[string]types.LogLevel{
	"trace":       types.LogLevelTrace,
	"10":          types.LogLevelTrace,
	"debug":       types.LogLevelDebug,
	"20":          types.LogLevelDebug,
	"info":        types.LogLevelInfo,
	"information": types.LogLevelInfo,
	"30":          types.LogLevelInfo,
	"warn":        types.LogLevelWarn,
	"warning":     types.LogLevelWarn,
	"40":          types.LogLevelWarn,
	"error":       types.LogLevelError,
	"err":         types.LogLevelError,
	"50":          types.LogLevelError,
	"fatal":       types.LogLevelFatal,
	"critical":    types.LogLevelFatal,
	"60":          types.LogLevelFatal,
}

// JSONParser parses structured JSON log lines.
type JSONParser struct{}

// NewJSONParser returns a new JSONParser instance.
func NewJSONParser() *JSONParser {
	return &JSONParser{}
}

// Name returns the format identifier for this parser.
func (p *JSONParser) Name() string {
	return "json"
}

// Parse attempts to parse a single JSON log line. Returns nil if the line is not valid JSON.
func (p *JSONParser) Parse(line string) *types.ParsedLogEntry {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil
	}

	entry := &types.ParsedLogEntry{
		Raw:         line,
		ExtraFields: make(map[string]interface{}),
	}

	for key, val := range raw {
		canonical, ok := fieldMapping[strings.ToLower(key)]
		if !ok {
			entry.ExtraFields[key] = val
			continue
		}

		strVal := valueToString(val)

		switch canonical {
		case "timestamp":
			ts := strVal
			entry.Timestamp = &ts
		case "level":
			normalized, found := levelMapping[strings.ToLower(strVal)]
			if found {
				entry.Level = &normalized
			} else {
				// Unrecognized level value — store as extra field
				entry.ExtraFields[key] = val
			}
		case "message":
			entry.Message = strVal
		case "source":
			src := strVal
			entry.Source = &src
		}
	}

	// Remove extra_fields if empty so omitempty works
	if len(entry.ExtraFields) == 0 {
		entry.ExtraFields = nil
	}

	return entry
}

// Detect returns the fraction of lines that successfully parse as JSON.
func (p *JSONParser) Detect(lines []string) float64 {
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

// valueToString converts a JSON value to its string representation.
// Numeric values (float64 from json.Unmarshal) are formatted without
// trailing decimals when they represent whole numbers.
func valueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case nil:
		return ""
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}
