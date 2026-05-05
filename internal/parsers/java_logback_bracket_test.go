package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestJavaLogbackBracketParser(t *testing.T) {
	p := NewJavaLogbackBracketParser()

	tests := []struct {
		name    string
		input   string
		wantNil bool
		level   types.LogLevel
		source  string
		thread  string
		traceID string
		msgPfx  string
	}{
		{
			name:    "INFO with trace ID",
			input:   `[05-05-2026 02:00:00.005] [scheduling-1] [INFO ] [9c10a348d4674afd] [com.aladdin.rms.job.StaleRecordsScheduler] Scheduler job is disabled to fetch the fx rates`,
			level:   types.LogLevelInfo,
			source:  "com.aladdin.rms.job.StaleRecordsScheduler",
			thread:  "scheduling-1",
			traceID: "9c10a348d4674afd",
			msgPfx:  "Scheduler job is disabled",
		},
		{
			name:    "INFO with long thread name",
			input:   `[05-05-2026 02:00:00.551] [org.springframework.kafka.KafkaListenerEndpointContainer#0-0-C-1] [INFO ] [a65f687638c14ffd] [com.aladdin.rms.messaging.KafkaMessageReceiver] Received FX rate event from provider [CURRENCYCLOUD]`,
			level:   types.LogLevelInfo,
			source:  "com.aladdin.rms.messaging.KafkaMessageReceiver",
			thread:  "org.springframework.kafka.KafkaListenerEndpointContainer#0-0-C-1",
			traceID: "a65f687638c14ffd",
			msgPfx:  "Received FX rate event",
		},
		{
			name:   "ERROR without trace ID",
			input:  `[05-05-2026 14:30:22.100] [http-nio-8080-exec-3] [ERROR] [com.aladdin.gpp.controller.PaymentController] Payment processing failed`,
			level:  types.LogLevelError,
			source: "com.aladdin.gpp.controller.PaymentController",
			thread: "http-nio-8080-exec-3",
			msgPfx: "Payment processing failed",
		},
		{
			name:    "WARN with trace ID",
			input:   `[05-05-2026 09:15:00.789] [pool-2-thread-1] [WARN ] [abcdef1234567890] [c.a.util.RetryHandler] Retry attempt 3 of 5`,
			level:   types.LogLevelWarn,
			source:  "c.a.util.RetryHandler",
			thread:  "pool-2-thread-1",
			traceID: "abcdef1234567890",
			msgPfx:  "Retry attempt 3",
		},
		{
			name:    "DEBUG level",
			input:   `[05-05-2026 09:15:00.001] [main] [DEBUG] [0000000000000000] [org.hibernate.SQL] select * from rates where id = ?`,
			level:   types.LogLevelDebug,
			source:  "org.hibernate.SQL",
			thread:  "main",
			traceID: "0000000000000000",
			msgPfx:  "select * from rates",
		},
		{
			name:    "not matching - standard logback",
			input:   `14:04:07.123 [main] INFO com.example.MyApp - Starting`,
			wantNil: true,
		},
		{
			name:    "not matching - Spring Boot",
			input:   `2025-05-05 14:04:07.123  INFO 12345 --- [main] c.e.MyApp : Starting`,
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
			if result.Source == nil || *result.Source != tt.source {
				t.Errorf("source = %v, want %q", result.Source, tt.source)
			}
			if result.Timestamp == nil {
				t.Error("timestamp is nil")
			}
			if len(result.Message) < len(tt.msgPfx) || result.Message[:len(tt.msgPfx)] != tt.msgPfx {
				t.Errorf("message = %q, want prefix %q", result.Message, tt.msgPfx)
			}
			thread, ok := result.ExtraFields["thread"]
			if !ok || thread != tt.thread {
				t.Errorf("thread = %v, want %q", thread, tt.thread)
			}
			if tt.traceID != "" {
				traceID, ok := result.ExtraFields["trace_id"]
				if !ok || traceID != tt.traceID {
					t.Errorf("trace_id = %v, want %q", traceID, tt.traceID)
				}
			}
		})
	}
}

func TestJavaLogbackBracketDetect(t *testing.T) {
	p := NewJavaLogbackBracketParser()

	lines := []string{
		`[05-05-2026 02:00:00.005] [scheduling-1] [INFO ] [9c10a348d4674afd] [com.aladdin.rms.job.Scheduler] Job running`,
		`[05-05-2026 02:00:00.551] [kafka-0-C-1] [INFO ] [a65f687638c14ffd] [com.aladdin.rms.Receiver] Received event`,
		`[05-05-2026 02:00:00.680] [kafka-0-C-1] [WARN ] [a65f687638c14ffd] [com.aladdin.rms.Service] Slow operation`,
		`[05-05-2026 02:00:00.800] [kafka-0-C-1] [ERROR] [1def95cb8c604cb8] [com.aladdin.rms.Service] Failed`,
	}

	score := p.Detect(lines)
	if score < 0.9 {
		t.Errorf("Detect score = %f, want >= 0.9", score)
	}
}
