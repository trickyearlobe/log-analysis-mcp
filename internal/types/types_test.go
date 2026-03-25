package types

import (
	"encoding/json"
	"testing"
)

func TestLogLevelConstants(t *testing.T) {
	tests := []struct {
		name  string
		level LogLevel
		want  string
	}{
		{"trace", LogLevelTrace, "TRACE"},
		{"debug", LogLevelDebug, "DEBUG"},
		{"info", LogLevelInfo, "INFO"},
		{"warn", LogLevelWarn, "WARN"},
		{"error", LogLevelError, "ERROR"},
		{"critical", LogLevelCritical, "CRITICAL"},
		{"fatal", LogLevelFatal, "FATAL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.level) != tt.want {
				t.Errorf("LogLevel %s = %q, want %q", tt.name, tt.level, tt.want)
			}
		})
	}
}

func TestLogFormatConstants(t *testing.T) {
	tests := []struct {
		name   string
		format LogFormat
		want   string
	}{
		{"syslog-rfc3164", LogFormatSyslogRFC3164, "syslog-rfc3164"},
		{"syslog-rfc5424", LogFormatSyslogRFC5424, "syslog-rfc5424"},
		{"apache-combined", LogFormatApacheCombined, "apache-combined"},
		{"apache-common", LogFormatApacheCommon, "apache-common"},
		{"nginx", LogFormatNginx, "nginx"},
		{"json", LogFormatJSON, "json"},
		{"unknown", LogFormatUnknown, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.format) != tt.want {
				t.Errorf("LogFormat %s = %q, want %q", tt.name, tt.format, tt.want)
			}
		})
	}
}

func TestParsedLogEntryJSONRoundTrip(t *testing.T) {
	ts := "2024-01-15T10:30:00Z"
	level := LogLevelError
	source := "nginx"

	entry := ParsedLogEntry{
		LineNumber:  42,
		LineCount:   3,
		Timestamp:   &ts,
		Level:       &level,
		Source:      &source,
		Message:     "connection refused",
		Raw:         "Jan 15 10:30:00 nginx: connection refused",
		StackTrace:  "at main.go:10\nat handler.go:55",
		ExtraFields: map[string]interface{}{"request_id": "abc-123"},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got ParsedLogEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.LineNumber != entry.LineNumber {
		t.Errorf("LineNumber = %d, want %d", got.LineNumber, entry.LineNumber)
	}
	if got.LineCount != entry.LineCount {
		t.Errorf("LineCount = %d, want %d", got.LineCount, entry.LineCount)
	}
	if *got.Timestamp != *entry.Timestamp {
		t.Errorf("Timestamp = %q, want %q", *got.Timestamp, *entry.Timestamp)
	}
	if *got.Level != *entry.Level {
		t.Errorf("Level = %q, want %q", *got.Level, *entry.Level)
	}
	if *got.Source != *entry.Source {
		t.Errorf("Source = %q, want %q", *got.Source, *entry.Source)
	}
	if got.Message != entry.Message {
		t.Errorf("Message = %q, want %q", got.Message, entry.Message)
	}
	if got.Raw != entry.Raw {
		t.Errorf("Raw = %q, want %q", got.Raw, entry.Raw)
	}
	if got.StackTrace != entry.StackTrace {
		t.Errorf("StackTrace = %q, want %q", got.StackTrace, entry.StackTrace)
	}
}

func TestParsedLogEntryNilFieldOmission(t *testing.T) {
	entry := ParsedLogEntry{
		LineNumber: 1,
		Message:    "test",
		Raw:        "test line",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	// nil pointer fields should marshal as JSON null, not be omitted
	if _, ok := m["timestamp"]; !ok {
		t.Error("expected 'timestamp' key present (as null), but missing")
	}
	if _, ok := m["level"]; !ok {
		t.Error("expected 'level' key present (as null), but missing")
	}
	if _, ok := m["source"]; !ok {
		t.Error("expected 'source' key present (as null), but missing")
	}

	// omitempty fields should be absent when zero/nil
	if _, ok := m["line_count"]; ok {
		t.Error("expected 'line_count' omitted when zero, but present")
	}
	if _, ok := m["stack_trace"]; ok {
		t.Error("expected 'stack_trace' omitted when empty, but present")
	}
	if _, ok := m["extra_fields"]; ok {
		t.Error("expected 'extra_fields' omitted when nil, but present")
	}
}

func TestFormatDetectionResultJSONRoundTrip(t *testing.T) {
	result := FormatDetectionResult{
		Format:           LogFormatApacheCombined,
		Confidence:       0.95,
		SampleSize:       100,
		SuccessfulParses: 95,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got FormatDetectionResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Format != result.Format {
		t.Errorf("Format = %q, want %q", got.Format, result.Format)
	}
	if got.Confidence != result.Confidence {
		t.Errorf("Confidence = %f, want %f", got.Confidence, result.Confidence)
	}
	if got.SampleSize != result.SampleSize {
		t.Errorf("SampleSize = %d, want %d", got.SampleSize, result.SampleSize)
	}
	if got.SuccessfulParses != result.SuccessfulParses {
		t.Errorf("SuccessfulParses = %d, want %d", got.SuccessfulParses, result.SuccessfulParses)
	}
}

func TestSearchMatchJSONRoundTrip(t *testing.T) {
	match := SearchMatch{
		LineNumber:    10,
		Line:          "ERROR: disk full",
		BeforeContext: []string{"WARN: disk 90%", "WARN: disk 95%"},
		AfterContext:  []string{"INFO: retrying..."},
	}

	data, err := json.Marshal(match)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got SearchMatch
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.LineNumber != match.LineNumber {
		t.Errorf("LineNumber = %d, want %d", got.LineNumber, match.LineNumber)
	}
	if got.Line != match.Line {
		t.Errorf("Line = %q, want %q", got.Line, match.Line)
	}
	if len(got.BeforeContext) != len(match.BeforeContext) {
		t.Errorf("BeforeContext len = %d, want %d", len(got.BeforeContext), len(match.BeforeContext))
	}
	if len(got.AfterContext) != len(match.AfterContext) {
		t.Errorf("AfterContext len = %d, want %d", len(got.AfterContext), len(match.AfterContext))
	}
}

func TestAnomalyJSONRoundTrip(t *testing.T) {
	anomaly := Anomaly{
		Type:        "error_spike",
		Severity:    "high",
		Description: "Error rate increased 5x",
		TimeRange: TimeRange{
			Start: "2024-01-15T10:00:00Z",
			End:   "2024-01-15T10:05:00Z",
		},
		Details: map[string]interface{}{"baseline_rate": 2.0, "spike_rate": 10.0},
		EvidenceLines: []EvidenceLine{
			{LineNumber: 100, Content: "ERROR: timeout"},
			{LineNumber: 101, Content: "ERROR: timeout"},
		},
	}

	data, err := json.Marshal(anomaly)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Anomaly
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Type != anomaly.Type {
		t.Errorf("Type = %q, want %q", got.Type, anomaly.Type)
	}
	if got.Severity != anomaly.Severity {
		t.Errorf("Severity = %q, want %q", got.Severity, anomaly.Severity)
	}
	if got.TimeRange.Start != anomaly.TimeRange.Start {
		t.Errorf("TimeRange.Start = %q, want %q", got.TimeRange.Start, anomaly.TimeRange.Start)
	}
	if len(got.EvidenceLines) != len(anomaly.EvidenceLines) {
		t.Errorf("EvidenceLines len = %d, want %d", len(got.EvidenceLines), len(anomaly.EvidenceLines))
	}
}

func TestErrorClusterJSONRoundTrip(t *testing.T) {
	ts1 := "2024-01-15T10:00:00Z"
	ts2 := "2024-01-15T10:30:00Z"
	stack := "goroutine 1:\nmain.go:42"

	cluster := ErrorCluster{
		Pattern:    "connection refused to .*",
		Count:      15,
		Percentage: 23.4,
		FirstSeen:  SeenAt{Timestamp: &ts1, LineNumber: 50},
		LastSeen:   SeenAt{Timestamp: &ts2, LineNumber: 200},
		SampleMessages: []string{
			"connection refused to db-1",
			"connection refused to db-2",
		},
		StackTrace: &stack,
	}

	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got ErrorCluster
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Pattern != cluster.Pattern {
		t.Errorf("Pattern = %q, want %q", got.Pattern, cluster.Pattern)
	}
	if got.Count != cluster.Count {
		t.Errorf("Count = %d, want %d", got.Count, cluster.Count)
	}
	if *got.StackTrace != *cluster.StackTrace {
		t.Errorf("StackTrace = %q, want %q", *got.StackTrace, *cluster.StackTrace)
	}
}

func TestErrorClusterNilStackTrace(t *testing.T) {
	cluster := ErrorCluster{
		Pattern:        "test",
		Count:          1,
		Percentage:     100.0,
		FirstSeen:      SeenAt{LineNumber: 1},
		LastSeen:       SeenAt{LineNumber: 1},
		SampleMessages: []string{"test"},
	}

	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	// stack_trace is a *string without omitempty, so should be present as null
	if _, ok := m["stack_trace"]; !ok {
		t.Error("expected 'stack_trace' key present (as null), but missing")
	}
}

func TestCorrelatedGroupJSONRoundTrip(t *testing.T) {
	level := LogLevelInfo
	source := "api"

	group := CorrelatedGroup{
		CorrelationID:    "req-abc-123",
		CorrelationField: "request_id",
		FilesInvolved:    []string{"app.log", "nginx.log"},
		TimeSpanMs:       1500,
		Events: []CorrelatedEvent{
			{
				Timestamp:  "2024-01-15T10:00:00Z",
				File:       "nginx.log",
				LineNumber: 10,
				Level:      &level,
				Source:     &source,
				Message:    "GET /api/users",
			},
		},
	}

	data, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got CorrelatedGroup
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.CorrelationID != group.CorrelationID {
		t.Errorf("CorrelationID = %q, want %q", got.CorrelationID, group.CorrelationID)
	}
	if len(got.FilesInvolved) != len(group.FilesInvolved) {
		t.Errorf("FilesInvolved len = %d, want %d", len(got.FilesInvolved), len(group.FilesInvolved))
	}
	if got.TimeSpanMs != group.TimeSpanMs {
		t.Errorf("TimeSpanMs = %d, want %d", got.TimeSpanMs, group.TimeSpanMs)
	}
	if len(got.Events) != 1 {
		t.Fatalf("Events len = %d, want 1", len(got.Events))
	}
	if *got.Events[0].Level != *group.Events[0].Level {
		t.Errorf("Event Level = %q, want %q", *got.Events[0].Level, *group.Events[0].Level)
	}
}

func TestFileInfoJSONRoundTrip(t *testing.T) {
	tr := &TimeRange{Start: "2024-01-15T00:00:00Z", End: "2024-01-15T23:59:59Z"}

	info := FileInfo{
		Name:         "app.log",
		Path:         "/var/log/app.log",
		SizeBytes:    1048576,
		SizeHuman:    "1.0 MB",
		TotalLines:   5000,
		LastModified: "2024-01-15T23:59:59Z",
		TimeRange:    tr,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got FileInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Name != info.Name {
		t.Errorf("Name = %q, want %q", got.Name, info.Name)
	}
	if got.SizeBytes != info.SizeBytes {
		t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, info.SizeBytes)
	}
	if got.TimeRange == nil {
		t.Fatal("TimeRange is nil")
	}
	if got.TimeRange.Start != tr.Start {
		t.Errorf("TimeRange.Start = %q, want %q", got.TimeRange.Start, tr.Start)
	}
}

func TestFileInfoNilTimeRange(t *testing.T) {
	info := FileInfo{
		Name:         "app.log",
		Path:         "/var/log/app.log",
		SizeBytes:    0,
		SizeHuman:    "0 B",
		TotalLines:   0,
		LastModified: "2024-01-15T00:00:00Z",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got FileInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.TimeRange != nil {
		t.Errorf("TimeRange = %v, want nil", got.TimeRange)
	}
}

func TestToolErrorJSONRoundTrip(t *testing.T) {
	toolErr := ToolError{
		Error:   true,
		Code:    "FILE_NOT_FOUND",
		Message: "file /var/log/missing.log does not exist",
		Details: map[string]interface{}{"path": "/var/log/missing.log"},
	}

	data, err := json.Marshal(toolErr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got ToolError
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Error != true {
		t.Error("Error = false, want true")
	}
	if got.Code != toolErr.Code {
		t.Errorf("Code = %q, want %q", got.Code, toolErr.Code)
	}
	if got.Message != toolErr.Message {
		t.Errorf("Message = %q, want %q", got.Message, toolErr.Message)
	}
}

func TestToolErrorDetailsOmission(t *testing.T) {
	toolErr := ToolError{
		Error:   true,
		Code:    "INTERNAL",
		Message: "something broke",
	}

	data, err := json.Marshal(toolErr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	if _, ok := m["details"]; ok {
		t.Error("expected 'details' omitted when nil, but present")
	}
}

func TestPaginationInfoJSONRoundTrip(t *testing.T) {
	pag := PaginationInfo{
		Start:   1,
		End:     100,
		Total:   5000,
		HasMore: true,
	}

	data, err := json.Marshal(pag)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got PaginationInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Start != pag.Start {
		t.Errorf("Start = %d, want %d", got.Start, pag.Start)
	}
	if got.End != pag.End {
		t.Errorf("End = %d, want %d", got.End, pag.End)
	}
	if got.Total != pag.Total {
		t.Errorf("Total = %d, want %d", got.Total, pag.Total)
	}
	if got.HasMore != pag.HasMore {
		t.Errorf("HasMore = %v, want %v", got.HasMore, pag.HasMore)
	}
}
