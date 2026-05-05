// Package types defines shared Go types used across all tools and parsers.
package types

// LogLevel represents normalized log severity levels.
type LogLevel string

const (
    LogLevelTrace    LogLevel = "TRACE"
    LogLevelDebug    LogLevel = "DEBUG"
    LogLevelInfo     LogLevel = "INFO"
    LogLevelWarn     LogLevel = "WARN"
    LogLevelError    LogLevel = "ERROR"
    LogLevelCritical LogLevel = "CRITICAL"
    LogLevelFatal    LogLevel = "FATAL"
)

// LogFormat represents detected log format identifiers.
type LogFormat string

const (
    LogFormatSyslogRFC3164  LogFormat = "syslog-rfc3164"
    LogFormatSyslogRFC5424  LogFormat = "syslog-rfc5424"
    LogFormatApacheCombined LogFormat = "apache-combined"
    LogFormatApacheCommon   LogFormat = "apache-common"
    LogFormatNginx          LogFormat = "nginx"
    LogFormatJSON           LogFormat = "json"
    LogFormatErlangSASL     LogFormat = "erlang-sasl"
    LogFormatHabitatSup     LogFormat = "habitat-sup"
    LogFormatJournalISO     LogFormat = "journalctl-short-iso"
    LogFormatGoLogrusBracket LogFormat = "go-logrus-bracket"
    LogFormatJavaLogback    LogFormat = "java-logback"
    LogFormatSpringBoot     LogFormat = "spring-boot"
    LogFormatUnknown        LogFormat = "unknown"
)

// ParsedLogEntry represents a single parsed log entry.
type ParsedLogEntry struct {
    LineNumber  int                    `json:"line_number"`
    LineCount   int                    `json:"line_count,omitempty"`
    Timestamp   *string                `json:"timestamp"`
    Level       *LogLevel              `json:"level"`
    Source      *string                `json:"source"`
    Message     string                 `json:"message"`
    Raw         string                 `json:"raw"`
    StackTrace  string                 `json:"stack_trace,omitempty"`
    ExtraFields map[string]interface{} `json:"extra_fields,omitempty"`
}

// FormatDetectionResult contains the result of format auto-detection.
type FormatDetectionResult struct {
    Format           LogFormat `json:"format"`
    Confidence       float64   `json:"confidence"`
    SampleSize       int       `json:"sample_size"`
    SuccessfulParses int       `json:"successful_parses"`
}

// SearchMatch represents a search result with optional context lines.
type SearchMatch struct {
    LineNumber    int      `json:"line_number"`
    Line          string   `json:"line"`
    BeforeContext []string `json:"before_context"`
    AfterContext  []string `json:"after_context"`
}

// TimeRange represents a start/end time range.
type TimeRange struct {
    Start string `json:"start"`
    End   string `json:"end"`
}

// EvidenceLine is a line of evidence supporting an anomaly.
type EvidenceLine struct {
    LineNumber int    `json:"line_number"`
    Content    string `json:"content"`
}

// Anomaly represents an anomalous pattern detected in log data.
type Anomaly struct {
    Type          string                 `json:"type"`
    Severity      string                 `json:"severity"`
    Description   string                 `json:"description"`
    TimeRange     TimeRange              `json:"time_range"`
    Details       map[string]interface{} `json:"details"`
    EvidenceLines []EvidenceLine         `json:"evidence_lines"`
}

// SeenAt records when/where an error was observed.
type SeenAt struct {
    Timestamp  *string `json:"timestamp"`
    LineNumber int     `json:"line_number"`
}

// ErrorCluster represents a cluster of similar errors from log_extract_errors.
type ErrorCluster struct {
    Pattern        string   `json:"pattern"`
    Count          int      `json:"count"`
    Percentage     float64  `json:"percentage"`
    ImpactScore    float64  `json:"impact_score,omitempty"`
    FirstSeen      SeenAt   `json:"first_seen"`
    LastSeen       SeenAt   `json:"last_seen"`
    SampleMessages []string `json:"sample_messages"`
    StackTrace     *string  `json:"stack_trace"`
}

// TimelineEvent represents a single event in a log_timeline.
type TimelineEvent struct {
    Timestamp  string  `json:"timestamp"`
    Type       string  `json:"type"`
    Source     *string `json:"source"`
    Message    string  `json:"message"`
    LineNumber int     `json:"line_number"`
}

// CorrelatedEvent is a single event within a correlated group.
type CorrelatedEvent struct {
    Timestamp  string    `json:"timestamp"`
    File       string    `json:"file"`
    LineNumber int       `json:"line_number"`
    Level      *LogLevel `json:"level"`
    Source     *string   `json:"source"`
    Message    string    `json:"message"`
}

// CorrelatedGroup represents a group of correlated events across files.
type CorrelatedGroup struct {
    CorrelationID    string            `json:"correlation_id"`
    CorrelationField string            `json:"correlation_field"`
    FilesInvolved    []string          `json:"files_involved"`
    TimeSpanMs       int64             `json:"time_span_ms"`
    Events           []CorrelatedEvent `json:"events"`
}

// FileInfo contains metadata about a log file.
type FileInfo struct {
    Name         string     `json:"name"`
    Path         string     `json:"path"`
    SizeBytes    int64      `json:"size_bytes"`
    SizeHuman    string     `json:"size_human"`
    TotalLines   int        `json:"total_lines"`
    LastModified string     `json:"last_modified"`
    TimeRange    *TimeRange `json:"time_range"`
}

// PaginationInfo describes a paginated result range.
type PaginationInfo struct {
    Start   int  `json:"start"`
    End     int  `json:"end"`
    Total   int  `json:"total"`
    HasMore bool `json:"has_more"`
}

// ToolError is the standard tool error response structure.
type ToolError struct {
    Error   bool                   `json:"error"`
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Details map[string]interface{} `json:"details,omitempty"`
}
