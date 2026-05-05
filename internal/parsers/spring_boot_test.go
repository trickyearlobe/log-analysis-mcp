package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestSpringBootParser(t *testing.T) {
	p := NewSpringBootParser()

	tests := []struct {
		name    string
		input   string
		wantNil bool
		level   types.LogLevel
		source  string
		thread  string
		pid     string
		app     string
		msgPfx  string
	}{
		{
			name:   "Spring Boot 2.x - INFO",
			input:  `2025-05-05 14:04:07.123  INFO 12345 --- [           main] o.s.b.w.e.t.TomcatWebServer  : Tomcat initialized with port(s): 8080 (http)`,
			level:  types.LogLevelInfo,
			source: "o.s.b.w.e.t.TomcatWebServer",
			thread: "main",
			pid:    "12345",
			msgPfx: "Tomcat initialized",
		},
		{
			name:   "Spring Boot 2.x - ERROR with complex thread",
			input:  `2025-05-05 14:04:07.456  ERROR 12345 --- [http-nio-8080-exec-3] c.e.a.GlobalExceptionHandler : Unhandled exception`,
			level:  types.LogLevelError,
			source: "c.e.a.GlobalExceptionHandler",
			thread: "http-nio-8080-exec-3",
			pid:    "12345",
			msgPfx: "Unhandled exception",
		},
		{
			name:   "Spring Boot 2.x - WARN with comma millis",
			input:  `2025-05-05 14:04:07,789  WARN 9876 --- [scheduling-1] o.s.s.c.ThreadPoolTaskScheduler : Thread pool full`,
			level:  types.LogLevelWarn,
			source: "o.s.s.c.ThreadPoolTaskScheduler",
			thread: "scheduling-1",
			pid:    "9876",
			msgPfx: "Thread pool full",
		},
		{
			name:   "Spring Boot 3.x - with app name",
			input:  `2025-05-05T14:04:07.123+01:00  INFO 35368 --- [myapp] [           main] o.s.b.w.e.t.TomcatWebServer  : Tomcat started on port 8080`,
			level:  types.LogLevelInfo,
			source: "o.s.b.w.e.t.TomcatWebServer",
			thread: "main",
			pid:    "35368",
			app:    "myapp",
			msgPfx: "Tomcat started",
		},
		{
			name:   "Spring Boot 3.x - DEBUG with Z timezone",
			input:  `2025-05-05T14:04:07.123Z  DEBUG 1001 --- [payment-service] [reactor-http-nio-4] c.e.p.PaymentClient : Sending request to gateway`,
			level:  types.LogLevelDebug,
			source: "c.e.p.PaymentClient",
			thread: "reactor-http-nio-4",
			pid:    "1001",
			app:    "payment-service",
			msgPfx: "Sending request",
		},
		{
			name:   "Spring Boot 3.x - TRACE",
			input:  `2025-05-05T09:00:00.001-05:00  TRACE 2222 --- [api-gateway] [pool-1-thread-7] o.s.w.r.f.DefaultWebFilterChain : Invoking filter`,
			level:  types.LogLevelTrace,
			source: "o.s.w.r.f.DefaultWebFilterChain",
			thread: "pool-1-thread-7",
			pid:    "2222",
			app:    "api-gateway",
			msgPfx: "Invoking filter",
		},
		{
			name:    "not matching - plain Logback",
			input:   `14:04:07.123 [main] INFO com.example.MyApp - Starting`,
			wantNil: true,
		},
		{
			name:    "not matching - Go logrus",
			input:   `2025-12-09 11:49:52.939 [ERROR][7059] plugin.go 162: error`,
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
			pid, ok := result.ExtraFields["pid"]
			if !ok || pid != tt.pid {
				t.Errorf("pid = %v, want %q", pid, tt.pid)
			}
			if tt.app != "" {
				app, ok := result.ExtraFields["app"]
				if !ok || app != tt.app {
					t.Errorf("app = %v, want %q", app, tt.app)
				}
			}
		})
	}
}

func TestSpringBootDetect(t *testing.T) {
	p := NewSpringBootParser()

	lines := []string{
		`2025-05-05 14:04:07.123  INFO 12345 --- [           main] o.s.b.SpringApplication : Starting MyApplication`,
		`2025-05-05 14:04:07.234  INFO 12345 --- [           main] o.s.b.SpringApplication : No active profile set`,
		`2025-05-05 14:04:07.567  INFO 12345 --- [           main] o.s.b.w.e.t.TomcatWebServer : Tomcat initialized`,
		`2025-05-05 14:04:07.789  INFO 12345 --- [           main] o.s.b.SpringApplication : Started in 3.4 seconds`,
	}

	score := p.Detect(lines)
	if score < 0.9 {
		t.Errorf("Detect score = %f, want >= 0.9", score)
	}
}

func TestSpringBootAutoDetect(t *testing.T) {
	lines := []string{
		`2025-05-05T14:04:07.123+01:00  INFO 35368 --- [myapp] [           main] o.s.b.SpringApplication : Starting MyApplication`,
		`2025-05-05T14:04:07.234+01:00  INFO 35368 --- [myapp] [           main] o.s.b.SpringApplication : No active profile set`,
		`2025-05-05T14:04:07.456+01:00  INFO 35368 --- [myapp] [           main] o.s.b.w.e.t.TomcatWebServer : Tomcat initialized with port(s): 8080`,
		`2025-05-05T14:04:07.789+01:00  INFO 35368 --- [myapp] [           main] o.s.b.SpringApplication : Started in 2.1 seconds`,
	}

	result := AutoDetect(lines)
	if result.Format != types.LogFormatSpringBoot {
		t.Errorf("AutoDetect = %q, want %q", result.Format, types.LogFormatSpringBoot)
	}
	if result.Confidence < 0.9 {
		t.Errorf("confidence = %f, want >= 0.9", result.Confidence)
	}
}
