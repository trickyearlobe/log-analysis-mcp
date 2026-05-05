package parsers

import (
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
