package parsers

import (
	"strings"
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestKubernetesCRIParser(t *testing.T) {
	p := NewKubernetesCRIParser()

	tests := []struct {
		name       string
		input      string
		wantNil    bool
		level      types.LogLevel
		source     string
		stream     string
		innerFmt   string
		msgPfx     string
		hasTraceID bool
	}{
		{
			name:       "CRI + bracket logback with trace",
			input:      `2026-05-05T03:00:00.005491542+01:00 stdout F [05-05-2026 02:00:00.005] [scheduling-1] [INFO ] [9c10a348d4674afd] [com.aladdin.rms.job.StaleRecordsScheduler] Scheduler job is disabled`,
			level:      types.LogLevelInfo,
			source:     "com.aladdin.rms.job.StaleRecordsScheduler",
			stream:     "stdout",
			innerFmt:   "java-logback-bracket",
			msgPfx:     "Scheduler job is disabled",
			hasTraceID: true,
		},
		{
			name:     "CRI + JSON inner",
			input:    `2026-05-05T03:00:00.123456789Z stdout F {"level":"error","msg":"connection refused","ts":"2026-05-05T03:00:00Z"}`,
			level:    types.LogLevelError,
			stream:   "stdout",
			innerFmt: "json",
			msgPfx:   "connection refused",
		},
		{
			name:   "CRI + plain text (no inner parser match)",
			input:  `2026-05-05T03:00:00.123456789Z stderr F Something went wrong`,
			level:  types.LogLevelError,
			stream: "stderr",
			msgPfx: "Something went wrong",
		},
		{
			name:   "CRI + stdout plain text with ERROR keyword",
			input:  `2026-05-05T14:30:00.000000000+01:00 stdout F ERROR: database connection timed out`,
			level:  types.LogLevelError,
			stream: "stdout",
			msgPfx: "ERROR: database connection",
		},
		{
			name:   "CRI + stdout plain text (info)",
			input:  `2026-05-05T14:30:00.000000000+01:00 stdout F Starting application server on port 8080`,
			level:  types.LogLevelInfo,
			stream: "stdout",
			msgPfx: "Starting application server",
		},
		{
			name:     "CRI + Go logrus bracket inner",
			input:    `2026-05-05T11:49:52.939000000+01:00 stderr F 2025-12-09 11:49:52.939 [ERROR][7059] plugin.go 162: Final result of CNI ADD was an error.`,
			level:    types.LogLevelError,
			stream:   "stderr",
			innerFmt: "go-logrus-bracket",
			msgPfx:   "Final result of CNI ADD",
		},
		{
			name:    "not matching - no CRI prefix",
			input:   `[05-05-2026 02:00:00.005] [scheduling-1] [INFO ] [abc] [com.example.App] Hello`,
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Level == nil || *result.Level != tt.level {
				t.Errorf("level = %v, want %v", result.Level, tt.level)
			}
			if tt.source != "" && (result.Source == nil || *result.Source != tt.source) {
				t.Errorf("source = %v, want %q", result.Source, tt.source)
			}
			if result.Timestamp == nil {
				t.Error("timestamp is nil")
			}
			if len(result.Message) < len(tt.msgPfx) || result.Message[:len(tt.msgPfx)] != tt.msgPfx {
				t.Errorf("message = %q, want prefix %q", result.Message, tt.msgPfx)
			}
			stream, ok := result.ExtraFields["stream"]
			if !ok || stream != tt.stream {
				t.Errorf("stream = %v, want %q", stream, tt.stream)
			}
			if tt.innerFmt != "" {
				innerFmt, ok := result.ExtraFields["inner_format"]
				if !ok || innerFmt != tt.innerFmt {
					t.Errorf("inner_format = %v, want %q", innerFmt, tt.innerFmt)
				}
			}
			if tt.hasTraceID {
				if _, ok := result.ExtraFields["trace_id"]; !ok {
					t.Error("expected trace_id in extra_fields")
				}
			}
		})
	}
}

func TestKubernetesCRIDetect(t *testing.T) {
	p := NewKubernetesCRIParser()

	lines := []string{
		`2026-05-05T03:00:00.005491542+01:00 stdout F [05-05-2026 02:00:00.005] [scheduling-1] [INFO ] [abc123] [com.example.App] Starting`,
		`2026-05-05T03:00:00.551131508+01:00 stdout F [05-05-2026 02:00:00.551] [kafka-0-C-1] [INFO ] [def456] [com.example.Kafka] Received`,
		`2026-05-05T03:00:00.687827272+01:00 stdout F [05-05-2026 02:00:00.687] [kafka-0-C-1] [WARN ] [ghi789] [com.example.Service] Slow`,
		`2026-05-05T03:00:00.800496969+01:00 stderr F Exception in thread "main" java.lang.NullPointerException`,
	}

	score := p.Detect(lines)
	if score < 0.9 {
		t.Errorf("Detect score = %f, want >= 0.9", score)
	}
}

func TestKubernetesCRIAutoDetect(t *testing.T) {
	// Real-world k8s pod log lines
	lines := []string{
		`2026-05-05T03:00:00.005491542+01:00 stdout F [05-05-2026 02:00:00.005] [scheduling-1] [INFO ] [9c10a348d4674afd] [com.aladdin.rms.job.StaleRecordsScheduler] Scheduler job is disabled`,
		`2026-05-05T03:00:00.551131508+01:00 stdout F [05-05-2026 02:00:00.551] [kafka-0-C-1] [INFO ] [a65f687638c14ffd] [com.aladdin.rms.messaging.KafkaMessageReceiver] Received FX rate event`,
		`2026-05-05T03:00:00.551145634+01:00 stdout F [05-05-2026 02:00:00.551] [kafka-0-C-1] [INFO ] [a65f687638c14ffd] [com.aladdin.rms.service.RatesManagerService] Processing FX rates`,
		`2026-05-05T03:00:00.687827272+01:00 stdout F [05-05-2026 02:00:00.687] [kafka-0-C-1] [INFO ] [a65f687638c14ffd] [com.aladdin.util.service.KafkaProducerService] Message sent successfully`,
	}

	result := AutoDetect(lines)
	if result.Format != types.LogFormatKubernetesCRI {
		t.Errorf("AutoDetect = %q, want %q", result.Format, types.LogFormatKubernetesCRI)
	}
	if result.Confidence < 0.9 {
		t.Errorf("confidence = %f, want >= 0.9", result.Confidence)
	}
}

func TestKubernetesCRITimestampOverride(t *testing.T) {
	p := NewKubernetesCRIParser()

	// CRI timestamp should win over inner timestamp
	input := `2026-05-05T03:00:00.005491542+01:00 stdout F [05-05-2026 02:00:00.005] [main] [INFO ] [abc] [com.example.App] Hello`
	result := p.Parse(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Timestamp == nil || *result.Timestamp != "2026-05-05T03:00:00.005491542+01:00" {
		t.Errorf("timestamp = %v, want CRI timestamp", result.Timestamp)
	}
}

func TestKubernetesCRIStackTraceContinuation(t *testing.T) {
	p := NewKubernetesCRIParser()

	// Stack trace lines wrapped in CRI should return nil (not a new entry)
	// so MultilineCombiner can attach them to the previous entry.
	stackLines := []string{
		`2026-05-05T14:30:00.001000000+01:00 stderr F 	at com.example.service.UserService.getUser(UserService.java:42)`,
		`2026-05-05T14:30:00.001000000+01:00 stderr F 	at com.example.controller.UserController.handle(UserController.java:15)`,
		`2026-05-05T14:30:00.001000000+01:00 stderr F Caused by: java.sql.SQLException: Connection refused`,
		`2026-05-05T14:30:00.001000000+01:00 stderr F    at System.Data.SqlClient.Connect(Connection.cs:44)`,
	}

	for _, line := range stackLines {
		result := p.Parse(line)
		if result != nil {
			t.Errorf("expected nil for stack trace line, got %+v for: %s", result, line[:80])
		}
	}

	// Normal lines should still parse
	normalLine := `2026-05-05T14:30:00.000000000+01:00 stderr F [05-05-2026 13:30:00.000] [main] [ERROR] [abc] [com.example.App] NullPointerException`
	result := p.Parse(normalLine)
	if result == nil {
		t.Fatal("expected non-nil for normal error line")
	}
}

func TestKubernetesCRIMultilineCombiner(t *testing.T) {
	p := NewKubernetesCRIParser()
	mc := NewMultilineCombiner(p)

	lines := []string{
		`2026-05-05T14:30:00.000000000+01:00 stderr F [05-05-2026 13:30:00.000] [http-1] [ERROR] [abc123] [com.example.App] NullPointerException`,
		`2026-05-05T14:30:00.001000000+01:00 stderr F 	at com.example.service.UserService.getUser(UserService.java:42)`,
		`2026-05-05T14:30:00.001000000+01:00 stderr F 	at com.example.controller.UserController.handle(UserController.java:15)`,
		`2026-05-05T14:30:00.002000000+01:00 stderr F Caused by: java.sql.SQLException: Connection refused`,
		`2026-05-05T14:30:00.002000000+01:00 stderr F 	at com.example.db.Pool.getConnection(Pool.java:99)`,
		`2026-05-05T14:30:01.000000000+01:00 stdout F [05-05-2026 13:30:01.000] [http-2] [INFO ] [def456] [com.example.App] Request completed`,
	}

	entries := mc.Combine(lines, 1)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// First entry should have the stack trace attached
	first := entries[0]
	if first.Level == nil || *first.Level != types.LogLevelError {
		t.Errorf("first level = %v, want ERROR", first.Level)
	}
	if first.LineCount != 5 {
		t.Errorf("first LineCount = %d, want 5", first.LineCount)
	}
	if first.StackTrace == "" {
		t.Error("first entry should have stack trace")
	}
	if !strings.Contains(first.StackTrace, "UserService.java:42") {
		t.Errorf("stack trace missing UserService reference: %q", first.StackTrace)
	}
	if !strings.Contains(first.StackTrace, "Caused by:") {
		t.Errorf("stack trace missing Caused by: %q", first.StackTrace)
	}

	// Second entry should be clean INFO with no stack trace
	second := entries[1]
	if second.Level == nil || *second.Level != types.LogLevelInfo {
		t.Errorf("second level = %v, want INFO", second.Level)
	}
	if second.StackTrace != "" {
		t.Errorf("second entry should have no stack trace, got %q", second.StackTrace)
	}
	if second.LineCount != 1 {
		t.Errorf("second LineCount = %d, want 1", second.LineCount)
	}
}
