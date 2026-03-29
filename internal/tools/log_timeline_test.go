package tools

import (
	"strings"
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestRunTimeline(t *testing.T) {
	// JSON log with mixed levels and lifecycle events.
	jsonLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","source":"app","message":"Server started on port 3000"}`,
		`{"timestamp":"2025-01-15T10:01:00Z","level":"INFO","source":"app","message":"Heartbeat ok"}`,
		`{"timestamp":"2025-01-15T10:02:00Z","level":"WARN","source":"db","message":"Connection pool exhausted"}`,
		`{"timestamp":"2025-01-15T10:03:00Z","level":"ERROR","source":"db","message":"Connection refused to primary"}`,
		`{"timestamp":"2025-01-15T10:04:00Z","level":"INFO","source":"deploy","message":"Deployment completed for version 2.4.1"}`,
		`{"timestamp":"2025-01-15T10:05:00Z","level":"ERROR","source":"auth","message":"Token validation failed"}`,
		`{"timestamp":"2025-01-15T10:06:00Z","level":"INFO","source":"app","message":"Shutting down gracefully"}`,
		`{"timestamp":"2025-01-15T10:07:00Z","level":"ERROR","source":"app","message":"panic in request handler"}`,
		`{"timestamp":"2025-01-15T10:08:00Z","level":"INFO","source":"app","message":"Server restarted successfully"}`,
		`{"timestamp":"2025-01-15T10:09:00Z","level":"DEBUG","source":"app","message":"Loading config from disk"}`,
	}, "\n") + "\n"
	jsonPath := writeTempLog(t, "timeline.log", jsonLog)

	emptyPath := writeTempLog(t, "empty.log", "")

	// Log with only lifecycle events at INFO level.
	lifecycleLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T08:00:00Z","level":"INFO","source":"app","message":"Server started on port 8080"}`,
		`{"timestamp":"2025-01-15T08:01:00Z","level":"INFO","source":"app","message":"Boot complete"}`,
		`{"timestamp":"2025-01-15T08:30:00Z","level":"INFO","source":"app","message":"Received SIGTERM"}`,
		`{"timestamp":"2025-01-15T08:30:01Z","level":"INFO","source":"app","message":"Graceful shutdown initiated"}`,
	}, "\n") + "\n"
	lifecyclePath := writeTempLog(t, "lifecycle.log", lifecycleLog)

	// Log for truncation test — 10 ERROR lines.
	var truncLines []string
	for i := 0; i < 10; i++ {
		truncLines = append(truncLines,
			`{"timestamp":"2025-01-15T10:00:`+padInt(i)+`Z","level":"ERROR","source":"app","message":"failure"}`)
	}
	truncPath := writeTempLog(t, "trunc.log", strings.Join(truncLines, "\n")+"\n")

	// Log for time range filtering.
	rangeLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T09:00:00Z","level":"ERROR","source":"app","message":"early error"}`,
		`{"timestamp":"2025-01-15T10:00:00Z","level":"ERROR","source":"app","message":"mid error"}`,
		`{"timestamp":"2025-01-15T11:00:00Z","level":"ERROR","source":"app","message":"late error"}`,
		`{"timestamp":"2025-01-15T12:00:00Z","level":"ERROR","source":"app","message":"latest error"}`,
	}, "\n") + "\n"
	rangePath := writeTempLog(t, "range.log", rangeLog)

	// Log where all entries have the same timestamp (duration = 0).
	sameTsLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"ERROR","source":"a","message":"err1"}`,
		`{"timestamp":"2025-01-15T10:00:00Z","level":"WARN","source":"a","message":"warn1"}`,
	}, "\n") + "\n"
	sameTsPath := writeTempLog(t, "samets.log", sameTsLog)

	tests := []struct {
		name        string
		input       TimelineInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out TimelineOutput)
	}{
		{
			name:  "basic timeline from JSON logs with mixed levels",
			input: TimelineInput{Path: jsonPath},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// Default event types: ERROR, WARN, FATAL, startup, shutdown, deploy, restart, crash, connection
				// Line 1: INFO "Server started" → startup (matches default)
				// Line 2: INFO "Heartbeat ok" → INFO (not in default filter)
				// Line 3: WARN "Connection pool exhausted" → WARN (matches default)
				// Line 4: ERROR "Connection refused" → ERROR (matches default)
				// Line 5: INFO "Deployment completed" → deploy (matches default)
				// Line 6: ERROR "Token validation failed" → ERROR (matches default)
				// Line 7: INFO "Shutting down" → shutdown (matches default)
				// Line 8: ERROR "panic in request handler" → crash (matches default)
				// Line 9: INFO "Server restarted" → startup ("restarted" contains "started", startup checked first)
				// Line 10: DEBUG "Loading config" → DEBUG (not in default filter)
				if out.EventCount != 8 {
					t.Errorf("EventCount = %d, want 8", out.EventCount)
				}
				if len(out.Events) != 8 {
					t.Errorf("len(Events) = %d, want 8", len(out.Events))
				}
				if out.Truncated {
					t.Error("Truncated should be false")
				}
				// Events should be sorted by timestamp.
				for i := 1; i < len(out.Events); i++ {
					if out.Events[i].Timestamp < out.Events[i-1].Timestamp {
						t.Errorf("Events not sorted: [%d]=%q before [%d]=%q",
							i-1, out.Events[i-1].Timestamp, i, out.Events[i].Timestamp)
					}
				}
			},
		},
		{
			name:  "event type classification — startup",
			input: TimelineInput{Path: jsonPath, EventTypes: []string{"startup"}},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// "Server started" → startup, "Server restarted" → startup ("restarted" contains "started")
				if out.EventCount != 2 {
					t.Errorf("EventCount = %d, want 2", out.EventCount)
				}
				if len(out.Events) != 2 {
					t.Fatalf("len(Events) = %d, want 2", len(out.Events))
				}
				for _, ev := range out.Events {
					if ev.Type != "startup" {
						t.Errorf("event type = %q, want startup", ev.Type)
					}
				}
			},
		},
		{
			name:  "event type classification — shutdown",
			input: TimelineInput{Path: jsonPath, EventTypes: []string{"shutdown"}},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if out.EventCount != 1 {
					t.Errorf("EventCount = %d, want 1", out.EventCount)
				}
				if len(out.Events) != 1 {
					t.Fatalf("len(Events) = %d, want 1", len(out.Events))
				}
				if out.Events[0].Type != "shutdown" {
					t.Errorf("Events[0].Type = %q, want shutdown", out.Events[0].Type)
				}
			},
		},
		{
			name:  "event type classification — crash overrides ERROR level",
			input: TimelineInput{Path: jsonPath, EventTypes: []string{"crash"}},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// Line 8: ERROR "panic in request handler" → classified as crash
				if out.EventCount != 1 {
					t.Errorf("EventCount = %d, want 1", out.EventCount)
				}
				if len(out.Events) != 1 {
					t.Fatalf("len(Events) = %d, want 1", len(out.Events))
				}
				if out.Events[0].Type != "crash" {
					t.Errorf("Events[0].Type = %q, want crash", out.Events[0].Type)
				}
			},
		},
		{
			name:  "event type classification — ERROR without lifecycle keyword",
			input: TimelineInput{Path: jsonPath, EventTypes: []string{"ERROR"}},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// Line 4: ERROR "Connection refused" → ERROR (no lifecycle keyword match)
				// Line 6: ERROR "Token validation failed" → ERROR (no lifecycle keyword match)
				// Line 8: ERROR "panic" → crash (lifecycle wins), NOT ERROR
				if out.EventCount != 2 {
					t.Errorf("EventCount = %d, want 2", out.EventCount)
				}
				for _, ev := range out.Events {
					if ev.Type != "ERROR" {
						t.Errorf("event type = %q, want ERROR", ev.Type)
					}
				}
			},
		},
		{
			name: "time range filtering — after only",
			input: TimelineInput{
				Path:  rangePath,
				After: "2025-01-15T10:00:00Z",
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// After is exclusive (ts > after): 11:00 and 12:00
				if out.EventCount != 2 {
					t.Errorf("EventCount = %d, want 2", out.EventCount)
				}
				for _, ev := range out.Events {
					if ev.Timestamp <= "2025-01-15T10:00:00Z" {
						t.Errorf("event timestamp %q should be after 2025-01-15T10:00:00Z", ev.Timestamp)
					}
				}
			},
		},
		{
			name: "time range filtering — before only",
			input: TimelineInput{
				Path:   rangePath,
				Before: "2025-01-15T11:00:00Z",
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// Before is exclusive (ts < before): 09:00 and 10:00
				if out.EventCount != 2 {
					t.Errorf("EventCount = %d, want 2", out.EventCount)
				}
				for _, ev := range out.Events {
					if ev.Timestamp >= "2025-01-15T11:00:00Z" {
						t.Errorf("event timestamp %q should be before 2025-01-15T11:00:00Z", ev.Timestamp)
					}
				}
			},
		},
		{
			name: "time range filtering — after and before combined",
			input: TimelineInput{
				Path:   rangePath,
				After:  "2025-01-15T09:00:00Z",
				Before: "2025-01-15T11:00:00Z",
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// After 09:00 exclusive AND before 11:00 exclusive → only 10:00
				if out.EventCount != 1 {
					t.Errorf("EventCount = %d, want 1", out.EventCount)
				}
				if len(out.Events) == 1 && out.Events[0].Timestamp != "2025-01-15T10:00:00Z" {
					t.Errorf("expected timestamp 2025-01-15T10:00:00Z, got %q", out.Events[0].Timestamp)
				}
			},
		},
		{
			name: "event type filtering — specific types",
			input: TimelineInput{
				Path:       jsonPath,
				EventTypes: []string{"deploy"},
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// deploy: line 5 "Deployment completed"
				// Note: "Server restarted" matches "started" (startup) before "restart"
				if out.EventCount != 1 {
					t.Errorf("EventCount = %d, want 1", out.EventCount)
				}
				if len(out.Events) != 1 {
					t.Fatalf("len(Events) = %d, want 1", len(out.Events))
				}
				if out.Events[0].Type != "deploy" {
					t.Errorf("event type = %q, want deploy", out.Events[0].Type)
				}
			},
		},
		{
			name: "MaxEvents truncation",
			input: TimelineInput{
				Path:      truncPath,
				MaxEvents: 3,
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if len(out.Events) != 3 {
					t.Errorf("len(Events) = %d, want 3", len(out.Events))
				}
				if out.EventCount != 10 {
					t.Errorf("EventCount = %d, want 10", out.EventCount)
				}
				if !out.Truncated {
					t.Error("Truncated should be true")
				}
			},
		},
		{
			name: "MaxEvents exact count not truncated",
			input: TimelineInput{
				Path:      truncPath,
				MaxEvents: 10,
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if len(out.Events) != 10 {
					t.Errorf("len(Events) = %d, want 10", len(out.Events))
				}
				if out.Truncated {
					t.Error("Truncated should be false")
				}
			},
		},
		{
			name:  "empty file",
			input: TimelineInput{Path: emptyPath},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if out.EventCount != 0 {
					t.Errorf("EventCount = %d, want 0", out.EventCount)
				}
				if len(out.Events) != 0 {
					t.Errorf("len(Events) = %d, want 0", len(out.Events))
				}
				if out.Events == nil {
					t.Error("Events should be non-nil empty slice, got nil")
				}
				if out.Truncated {
					t.Error("Truncated should be false")
				}
			},
		},
		{
			name:        "file not found",
			input:       TimelineInput{Path: "/nonexistent/path/to/file.log"},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name:  "default event types filter applied — excludes INFO and DEBUG",
			input: TimelineInput{Path: jsonPath},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				for _, ev := range out.Events {
					if ev.Type == "INFO" || ev.Type == "DEBUG" {
						t.Errorf("unexpected event type %q in default-filtered output", ev.Type)
					}
				}
			},
		},
		{
			name: "default MaxEvents is 100",
			input: TimelineInput{
				Path: jsonPath,
				// MaxEvents left as 0 → should default to 100.
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// File only has 8 matching events, no truncation.
				if out.Truncated {
					t.Error("Truncated should be false with default MaxEvents")
				}
			},
		},
		{
			name:  "time span computed correctly",
			input: TimelineInput{Path: jsonPath},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if len(out.Events) == 0 {
					t.Fatal("expected events")
				}
				if out.TimeSpan.Start != out.Events[0].Timestamp {
					t.Errorf("TimeSpan.Start = %q, want %q", out.TimeSpan.Start, out.Events[0].Timestamp)
				}
				if out.TimeSpan.End != out.Events[len(out.Events)-1].Timestamp {
					t.Errorf("TimeSpan.End = %q, want %q", out.TimeSpan.End, out.Events[len(out.Events)-1].Timestamp)
				}
				if out.TimeSpan.DurationMinutes < 0 {
					t.Errorf("DurationMinutes = %f, want >= 0", out.TimeSpan.DurationMinutes)
				}
			},
		},
		{
			name:  "time span zero duration when single event or same timestamps",
			input: TimelineInput{Path: sameTsPath},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if out.TimeSpan.DurationMinutes != 0 {
					t.Errorf("DurationMinutes = %f, want 0", out.TimeSpan.DurationMinutes)
				}
			},
		},
		{
			name:  "lifecycle events detected at INFO level via keywords",
			input: TimelineInput{Path: lifecyclePath},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// All 4 lines are INFO but have lifecycle keywords.
				// Default filter includes startup, shutdown.
				// "Server started" → startup
				// "Boot complete" → startup
				// "Received SIGTERM" → shutdown
				// "Graceful shutdown" → shutdown
				if out.EventCount != 4 {
					t.Errorf("EventCount = %d, want 4", out.EventCount)
				}
				typeCount := make(map[string]int)
				for _, ev := range out.Events {
					typeCount[ev.Type]++
				}
				if typeCount["startup"] != 2 {
					t.Errorf("startup count = %d, want 2", typeCount["startup"])
				}
				if typeCount["shutdown"] != 2 {
					t.Errorf("shutdown count = %d, want 2", typeCount["shutdown"])
				}
			},
		},
		{
			name: "events sorted chronologically",
			input: TimelineInput{
				Path:       rangePath,
				EventTypes: []string{"ERROR"},
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if len(out.Events) < 2 {
					t.Fatalf("expected at least 2 events, got %d", len(out.Events))
				}
				for i := 1; i < len(out.Events); i++ {
					if out.Events[i].Timestamp < out.Events[i-1].Timestamp {
						t.Errorf("events not sorted: [%d]=%q before [%d]=%q",
							i-1, out.Events[i-1].Timestamp, i, out.Events[i].Timestamp)
					}
				}
			},
		},
		{
			name: "source field preserved in events",
			input: TimelineInput{
				Path:       jsonPath,
				EventTypes: []string{"shutdown"},
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// "Shutting down gracefully" → shutdown, only one match
				if len(out.Events) != 1 {
					t.Fatalf("len(Events) = %d, want 1", len(out.Events))
				}
				if out.Events[0].Source == nil {
					t.Fatal("Source should not be nil")
				}
				if *out.Events[0].Source != "app" {
					t.Errorf("Source = %q, want app", *out.Events[0].Source)
				}
			},
		},
		{
			name: "line numbers are correct",
			input: TimelineInput{
				Path:       jsonPath,
				EventTypes: []string{"ERROR"},
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				// ERROR entries (not lifecycle-classified) at lines 4 and 6.
				if len(out.Events) != 2 {
					t.Fatalf("len(Events) = %d, want 2", len(out.Events))
				}
				wantLines := []int{4, 6}
				for i, want := range wantLines {
					if out.Events[i].LineNumber != want {
						t.Errorf("Events[%d].LineNumber = %d, want %d", i, out.Events[i].LineNumber, want)
					}
				}
			},
		},
		{
			name: "time range excluding all entries returns empty",
			input: TimelineInput{
				Path:  jsonPath,
				After: "2099-01-01T00:00:00Z",
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if out.EventCount != 0 {
					t.Errorf("EventCount = %d, want 0", out.EventCount)
				}
				if len(out.Events) != 0 {
					t.Errorf("len(Events) = %d, want 0", len(out.Events))
				}
			},
		},
		{
			name: "connection lifecycle type detected",
			input: TimelineInput{
				Path: writeTempLog(t, "conn.log", strings.Join([]string{
					`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","source":"net","message":"Client connected from 10.0.0.1"}`,
					`{"timestamp":"2025-01-15T10:01:00Z","level":"WARN","source":"net","message":"Connection lost to peer"}`,
					`{"timestamp":"2025-01-15T10:02:00Z","level":"INFO","source":"net","message":"Attempting reconnect"}`,
				}, "\n") + "\n"),
				EventTypes: []string{"connection"},
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if out.EventCount != 3 {
					t.Errorf("EventCount = %d, want 3", out.EventCount)
				}
				for _, ev := range out.Events {
					if ev.Type != "connection" {
						t.Errorf("event type = %q, want connection", ev.Type)
					}
				}
			},
		},
		{
			name: "deploy lifecycle type detected",
			input: TimelineInput{
				Path:       jsonPath,
				EventTypes: []string{"deploy"},
			},
			checkOutput: func(t *testing.T, out TimelineOutput) {
				if out.EventCount != 1 {
					t.Errorf("EventCount = %d, want 1", out.EventCount)
				}
				if len(out.Events) == 1 {
					if out.Events[0].Type != "deploy" {
						t.Errorf("Type = %q, want deploy", out.Events[0].Type)
					}
				}
			},
		},
		{
			name: "binary file error",
			input: TimelineInput{
				Path: writeTempBinary(t),
			},
			wantErr:     true,
			errContains: "BINARY_FILE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RunTimeline(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkOutput != nil {
				tc.checkOutput(t, out)
			}
		})
	}
}

// padInt returns a two-digit zero-padded string for values 0-59.
func padInt(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	tens := n / 10
	ones := n % 10
	return string(rune('0'+tens)) + string(rune('0'+ones))
}

func TestClassifyEventType(t *testing.T) {
	infoLevel := types.LogLevelInfo
	errorLevel := types.LogLevelError
	warnLevel := types.LogLevelWarn

	tests := []struct {
		name    string
		message string
		level   *types.LogLevel
		want    string
	}{
		{name: "startup keyword", message: "Server started on port 3000", level: &infoLevel, want: "startup"},
		{name: "startup keyword listening", message: "Listening on 0.0.0.0:8080", level: &infoLevel, want: "startup"},
		{name: "startup keyword boot", message: "Boot complete in 2.3s", level: &infoLevel, want: "startup"},
		{name: "shutdown keyword", message: "Shutting down gracefully", level: &infoLevel, want: "shutdown"},
		{name: "shutdown keyword stopped", message: "Service stopped", level: &infoLevel, want: "shutdown"},
		{name: "shutdown keyword sigterm", message: "Received SIGTERM, exiting", level: &infoLevel, want: "shutdown"},
		{name: "deploy keyword", message: "Deployed version 2.4.1", level: &infoLevel, want: "deploy"},
		{name: "deploy keyword release", message: "New release pushed", level: &infoLevel, want: "deploy"},
		{name: "restart keyword", message: "Process restarting now", level: &infoLevel, want: "restart"},
		{name: "restart keyword respawn", message: "Worker respawn triggered", level: &infoLevel, want: "restart"},
		{name: "crash keyword panic", message: "panic: runtime error", level: &errorLevel, want: "crash"},
		{name: "crash keyword fatal", message: "Fatal error in handler", level: &errorLevel, want: "crash"},
		{name: "crash keyword segfault", message: "Process segfault at 0xdead", level: &errorLevel, want: "crash"},
		{name: "crash keyword core dump", message: "Core dump generated", level: &errorLevel, want: "crash"},
		{name: "connection keyword connected", message: "Client connected from 10.0.0.1", level: &infoLevel, want: "connection"},
		{name: "connection keyword disconnected", message: "Peer disconnected", level: &infoLevel, want: "connection"},
		{name: "connection keyword lost", message: "Connection lost to upstream", level: &warnLevel, want: "connection"},
		{name: "connection keyword reconnect", message: "Attempting reconnect", level: &infoLevel, want: "connection"},
		{name: "lifecycle wins over level", message: "panic in worker", level: &errorLevel, want: "crash"},
		{name: "falls back to level ERROR", message: "Something went wrong", level: &errorLevel, want: "ERROR"},
		{name: "falls back to level WARN", message: "High memory usage", level: &warnLevel, want: "WARN"},
		{name: "falls back to level INFO", message: "Heartbeat ok", level: &infoLevel, want: "INFO"},
		{name: "nil level falls back to INFO", message: "No level here", level: nil, want: "INFO"},
		{name: "case insensitive keyword match", message: "SERVER READY for requests", level: &infoLevel, want: "startup"},
		{name: "case insensitive keyword match 2", message: "GRACEFUL SHUTDOWN complete", level: &infoLevel, want: "shutdown"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := &types.ParsedLogEntry{
				Message: tc.message,
				Level:   tc.level,
			}
			got := classifyEventType(entry)
			if got != tc.want {
				t.Errorf("classifyEventType(%q) = %q, want %q", tc.message, got, tc.want)
			}
		})
	}
}
