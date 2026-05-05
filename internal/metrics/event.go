// Package metrics provides event logging and aggregation for MCP tool observability.
package metrics

import (
	"encoding/json"
	"time"
)

// Event represents a single tool invocation record.
type Event struct {
	Timestamp     time.Time `json:"ts"`
	Tool          string    `json:"tool"`
	Status        string    `json:"status"`
	DurationMs    int64     `json:"duration_ms"`
	ResponseBytes int       `json:"response_bytes"`
	Warning       string    `json:"warning,omitempty"`
	ErrorCode     string    `json:"error_code,omitempty"`
	FileBytes     int64     `json:"file_bytes,omitempty"`
}

// Warning categories for notable events.
const (
	WarnSlowCall      = "SLOW_CALL"
	WarnLargeResponse = "LARGE_RESPONSE"
	WarnNoResults     = "NO_RESULTS"
	WarnFileTooLarge  = "FILE_TOO_LARGE"
	WarnParseFailure  = "PARSE_FAILURE"
)

// Status values.
const (
	StatusOK    = "ok"
	StatusError = "error"
)

// Thresholds for warning detection.
const (
	SlowCallThresholdMs    = 2000
	LargeResponseThreshold = 50 * 1024
)

// MarshalEvent serializes an event to a single JSON line.
func MarshalEvent(e Event) ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalEvent deserializes a JSON line into an Event.
func UnmarshalEvent(data []byte) (Event, error) {
	var e Event
	err := json.Unmarshal(data, &e)
	return e, err
}
