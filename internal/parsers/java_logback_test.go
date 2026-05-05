package parsers

import (
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestJavaLogbackParser(t *testing.T) {
	p := NewJavaLogbackParser()

	tests := []struct {
		name    string
		input   string
		wantNil bool
		level   types.LogLevel
		source  string
		thread  string
		msgPfx  string
	}{
		{
			name:   "pattern A - time only (Log4j2/Logback default)",
			input:  `14:04:07.123 [main] INFO com.example.service.MyClass - Starting application`,
			level:  types.LogLevelInfo,
			source: "com.example.service.MyClass",
			thread: "main",
			msgPfx: "Starting application",
		},
		{
			name:   "pattern A - with comma millis",
			input:  `14:04:07,456 [http-nio-8080-exec-1] ERROR com.example.api.UserController - Failed to process request`,
			level:  types.LogLevelError,
			source: "com.example.api.UserController",
			thread: "http-nio-8080-exec-1",
			msgPfx: "Failed to process",
		},
		{
			name:   "pattern A - debug level",
			input:  `09:15:22.001 [scheduling-1] DEBUG o.s.web.servlet.DispatcherServlet - Completed 200 OK`,
			level:  types.LogLevelDebug,
			source: "o.s.web.servlet.DispatcherServlet",
			thread: "scheduling-1",
			msgPfx: "Completed 200",
		},
		{
			name:   "pattern B - date level [thread] logger",
			input:  `2025-05-05 14:04:07.123 INFO [main] com.example.MyApp - Application started`,
			level:  types.LogLevelInfo,
			source: "com.example.MyApp",
			thread: "main",
			msgPfx: "Application started",
		},
		{
			name:   "pattern B - with T separator",
			input:  `2025-05-05T14:04:07.123 WARN [pool-2-thread-1] c.e.cache.CacheManager - Cache eviction triggered`,
			level:  types.LogLevelWarn,
			source: "c.e.cache.CacheManager",
			thread: "pool-2-thread-1",
			msgPfx: "Cache eviction",
		},
		{
			name:   "pattern C - date [thread] level logger",
			input:  `2025-05-05 14:04:07.123 [main] INFO com.example.MyApp - Application started`,
			level:  types.LogLevelInfo,
			source: "com.example.MyApp",
			thread: "main",
			msgPfx: "Application started",
		},
		{
			name:   "pattern C - trace level",
			input:  `2025-05-05 09:00:00,789 [reactor-http-nio-4] TRACE io.netty.handler.codec.http.HttpDecoder - Decoded headers`,
			level:  types.LogLevelTrace,
			source: "io.netty.handler.codec.http.HttpDecoder",
			thread: "reactor-http-nio-4",
			msgPfx: "Decoded headers",
		},
		{
			name:    "not matching - Spring Boot format",
			input:   `2025-05-05 14:04:07.123  INFO 12345 --- [           main] c.e.MyApp : Starting`,
			wantNil: true,
		},
		{
			name:    "not matching - Go logrus bracket",
			input:   `2025-12-09 11:49:52.939 [ERROR][7059] plugin.go 162: message`,
			wantNil: true,
		},
		{
			name:    "not matching - syslog",
			input:   `May  5 14:04:07 myhost java[1234]: Starting`,
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
		})
	}
}

func TestJavaLogbackDetect(t *testing.T) {
	p := NewJavaLogbackParser()

	lines := []string{
		`14:04:07.123 [main] INFO com.example.service.AppMain - Starting`,
		`14:04:07.234 [main] INFO com.example.service.AppMain - Initializing context`,
		`14:04:07.345 [main] DEBUG o.s.c.a.AnnotationConfigApplicationContext - Refreshing`,
		`14:04:07.456 [main] WARN com.example.Config - No profile set, using default`,
	}

	score := p.Detect(lines)
	if score < 0.9 {
		t.Errorf("Detect score = %f, want >= 0.9", score)
	}
}

func TestJavaLogbackAutoDetect(t *testing.T) {
	lines := []string{
		`14:04:07.123 [main] INFO com.example.service.AppMain - Starting application`,
		`14:04:07.234 [main] INFO com.example.service.AppMain - Initializing Spring context`,
		`14:04:07.345 [main] DEBUG o.s.c.a.AnnotationConfigApplicationContext - Refreshing ApplicationContext`,
		`14:04:07.456 [main] WARN com.example.Config - No active profile set, falling back to default`,
		`14:04:07.567 [main] INFO com.example.service.AppMain - Started in 2.3 seconds`,
	}

	result := AutoDetect(lines)
	if result.Format != types.LogFormatJavaLogback {
		t.Errorf("AutoDetect = %q, want %q", result.Format, types.LogFormatJavaLogback)
	}
	if result.Confidence < 0.9 {
		t.Errorf("confidence = %f, want >= 0.9", result.Confidence)
	}
}
